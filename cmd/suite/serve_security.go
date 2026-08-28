// Package main: security hardening for the local Web UI (serve).
//
// The Web UI is a local operator tool that generates deployment secrets. PR11
// tightens its boundary so it is safe by default:
//   - binds 127.0.0.1 by default; a non-loopback --listen is refused unless the
//     operator explicitly passes --allow-remote (and then an access token is
//     required — generated if not supplied);
//   - state-changing POSTs must carry a same-origin Origin/Referer and (in
//     remote mode) a valid bearer/cookie token, blocking CSRF and drive-by
//     requests from other origins;
//   - cookies are HttpOnly + SameSite=Strict and Secure by default; the local
//     HTTP container flow requires an explicit insecure-cookie opt-in;
//   - the HTTP server has read/write/idle timeouts and a bounded header size.
//
// None of this changes what is generated — CLI and Web UI still call the same
// composegen/policy functions — it only controls who may reach the UI and how.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// serveConfig captures the resolved, validated listen/security settings for the
// Web UI. It is produced by resolveServeConfig from the serve flag set.
type serveConfig struct {
	// listenAddr is the host:port the server binds. Defaults to 127.0.0.1:8085.
	listenAddr string
	// allowRemote is true when the operator explicitly opted into a non-loopback
	// bind via --allow-remote.
	allowRemote bool
	// token, when non-empty, is required on every request (bearer header or
	// cookie). It is always set in remote mode.
	token string
	// loopback reports whether listenAddr is a plain loopback address (drives
	// whether a token is mandatory).
	loopback bool
	// secureCookie is explicit server configuration. Request Host cannot safely
	// infer the browser-facing scheme when an HTTPS reverse proxy is involved.
	secureCookie bool
}

// serveTokenCookieName is the cookie carrying the access token in remote mode.
const serveTokenCookieName = "stargate_suite_token"

// resolveServeConfig turns raw flag inputs into a validated serveConfig. It
// enforces the loopback-by-default policy: a non-loopback listen address is a
// hard error unless allowRemote is set, and remote mode always yields a token
// (using the supplied one, or a freshly generated one the caller must surface).
//
// port is the legacy --port / SERVE_PORT value (used only when listen is empty,
// preserving backward compatibility). listen (--listen) takes precedence.
func resolveServeConfig(listen, port, token string, allowRemote, allowInsecureCookie bool) (serveConfig, error) {
	addr := strings.TrimSpace(listen)
	if addr == "" {
		p := strings.TrimSpace(port)
		if p == "" {
			p = "8085"
		}
		// Loopback-by-default: a bare port binds 127.0.0.1, not 0.0.0.0.
		addr = net.JoinHostPort("127.0.0.1", p)
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return serveConfig{}, fmt.Errorf("invalid --listen %q: %w (expected host:port, e.g. 127.0.0.1:8085)", addr, err)
	}

	cfg := serveConfig{
		listenAddr:  addr,
		allowRemote: allowRemote,
		token:       strings.TrimSpace(token),
		loopback:    isLoopbackHost(host),
		secureCookie: !allowInsecureCookie,
	}

	if !cfg.loopback && !allowRemote {
		return serveConfig{}, fmt.Errorf("refusing to bind non-loopback address %q without --allow-remote; "+
			"the Web UI generates secrets and must not be exposed unintentionally. "+
			"Re-run with --allow-remote (an access token will be required)", addr)
	}

	// Remote mode always requires a token; generate one if the operator did not
	// supply their own so the UI is never reachable unauthenticated off-host.
	if !cfg.loopback && cfg.token == "" {
		t, gerr := generateAccessToken()
		if gerr != nil {
			return serveConfig{}, fmt.Errorf("generate access token: %w", gerr)
		}
		cfg.token = t
	}
	return cfg, nil
}

// isLoopbackHost reports whether host is a loopback address (127.0.0.0/8, ::1)
// or the empty host that Go maps to all interfaces (treated as NON-loopback so
// an explicit ":8085" still requires --allow-remote).
func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		// ":8085" binds all interfaces → not loopback.
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// generateAccessToken returns a 32-hex-char (128-bit) random token.
func generateAccessToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// securityMiddleware enforces token auth (remote mode) and Origin/CSRF checks
// for state-changing methods. It wraps the whole mux so every route is covered.
func securityMiddleware(cfg serveConfig, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Liveness carries no application state or secrets and must remain
		// available to container health probes before authentication.
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Token gate (only when a token is configured, i.e. remote mode or an
		// operator-supplied token on loopback). GET /static and the login-less
		// entry are still gated so the UI cannot be scraped off-host.
		if cfg.token != "" && !tokenOK(cfg, r) {
			// Allow a one-shot token hand-off via ?token= on a GET so the printed
			// URL works in a browser; the middleware then sets the cookie.
			if r.Method == http.MethodGet && subtleEqual(r.URL.Query().Get("token"), cfg.token) {
				http.SetCookie(w, &http.Cookie{
					Name:     serveTokenCookieName,
					Value:    cfg.token,
					Path:     "/",
					HttpOnly: true,
					Secure:   cfg.secureCookie,
					SameSite: http.SameSiteStrictMode,
				})
				// Redirect to the same path without the token in the URL/history.
				clean := r.URL.Path
				http.Redirect(w, r, clean, http.StatusFound)
				return
			}
			http.Error(w, "unauthorized: missing or invalid access token", http.StatusUnauthorized)
			return
		}

		// CSRF / cross-origin gate for state-changing methods. A same-origin
		// browser sends Origin (or at least Referer) matching Host; a cross-site
		// attacker cannot forge these. Non-browser callers (curl) may omit both;
		// on loopback we allow that (local tooling), off-host we require it.
		if isStateChanging(r.Method) {
			if !originOK(cfg, r) {
				http.Error(w, "forbidden: cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// browserAddr turns an unspecified bind address into a usable local URL. A
// listener may bind all interfaces, but 0.0.0.0 and :: are not destinations a
// browser should be told to open.
func browserAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return net.JoinHostPort(host, port)
}

// isStateChanging reports whether the method mutates state and thus needs CSRF
// protection.
func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// tokenOK reports whether the request carries the configured access token via
// Authorization: Bearer <token> or the token cookie.
func tokenOK(cfg serveConfig, r *http.Request) bool {
	if cfg.token == "" {
		return true
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if subtleEqual(strings.TrimPrefix(auth, "Bearer "), cfg.token) {
			return true
		}
	}
	if c, err := r.Cookie(serveTokenCookieName); err == nil && c != nil {
		if subtleEqual(c.Value, cfg.token) {
			return true
		}
	}
	return false
}

// originOK validates that a state-changing request originates from the same
// host the server is serving. It checks Origin first, then falls back to
// Referer. When neither is present it is allowed only on loopback (local CLI
// tooling); off-host a missing Origin/Referer is rejected.
func originOK(cfg serveConfig, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if origin == "" && referer == "" {
		return cfg.loopback
	}
	candidate := origin
	if candidate == "" {
		candidate = referer
	}
	return hostFromURL(candidate) == r.Host
}

// hostFromURL extracts the host[:port] authority from an Origin/Referer value
// without pulling in net/url for such a small parse (Origin is scheme://host).
func hostFromURL(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "://"); i >= 0 {
		v = v[i+3:]
	}
	// strip path/query/fragment.
	if i := strings.IndexAny(v, "/?#"); i >= 0 {
		v = v[:i]
	}
	return v
}

// subtleEqual is a constant-time string comparison for tokens.
func subtleEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// newSecureHTTPServer builds an *http.Server with hardened timeouts and header
// limits for the Web UI listener.
func newSecureHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16, // 64KB
	}
}
