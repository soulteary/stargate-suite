package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// okHandler is a trivial downstream that records it was reached.
func okHandler() (http.Handler, *bool) {
	reached := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &reached
}

func TestResolveServeConfigLoopbackDefault(t *testing.T) {
	cfg, err := resolveServeConfig("", "8085", "", false, false)
	if err != nil {
		t.Fatalf("default config should succeed: %v", err)
	}
	if cfg.listenAddr != "127.0.0.1:8085" {
		t.Errorf("default listen = %q, want 127.0.0.1:8085", cfg.listenAddr)
	}
	if !cfg.loopback {
		t.Errorf("default must be loopback")
	}
	if cfg.token != "" {
		t.Errorf("loopback default must not require a token")
	}
}

func TestResolveServeConfigRemoteRequiresAllowRemote(t *testing.T) {
	if _, err := resolveServeConfig("0.0.0.0:8085", "", "", false, false); err == nil {
		t.Fatalf("non-loopback without --allow-remote must be refused")
	}
	// With --allow-remote a token is auto-generated.
	cfg, err := resolveServeConfig("0.0.0.0:8085", "", "", true, false)
	if err != nil {
		t.Fatalf("allow-remote should succeed: %v", err)
	}
	if cfg.loopback {
		t.Errorf("0.0.0.0 must not be loopback")
	}
	if cfg.token == "" {
		t.Errorf("remote mode must auto-generate a token")
	}
}

func TestResolveServeConfigBarePortIsNotLoopback(t *testing.T) {
	// ":8085" binds all interfaces → must require --allow-remote.
	if _, err := resolveServeConfig(":8085", "", "", false, false); err == nil {
		t.Fatalf(":8085 (all interfaces) without --allow-remote must be refused")
	}
}

func TestResolveServeConfigInvalidListen(t *testing.T) {
	if _, err := resolveServeConfig("not-a-host-port", "", "", true, false); err == nil {
		t.Fatalf("invalid listen must error")
	}
}

func TestSecurityMiddlewareTokenGate(t *testing.T) {
	cfg := serveConfig{listenAddr: "0.0.0.0:8085", allowRemote: true, token: "secrettoken", loopback: false}
	h, reached := okHandler()
	mw := securityMiddleware(cfg, h)

	// No token → 401.
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing token GET: code=%d, want 401", rec.Code)
	}
	if *reached {
		t.Errorf("handler must not be reached without token")
	}

	// Correct bearer token → passes.
	*reached = false
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secrettoken")
	mw.ServeHTTP(rec, req)
	if !*reached || rec.Code != http.StatusOK {
		t.Errorf("valid token must reach handler; code=%d reached=%v", rec.Code, *reached)
	}
}

func TestSecurityMiddlewareTokenHandoffViaQuery(t *testing.T) {
	cfg := serveConfig{listenAddr: "0.0.0.0:8085", allowRemote: true, token: "abc123", loopback: false}
	h, _ := okHandler()
	mw := securityMiddleware(cfg, h)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?token=abc123", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("token handoff should redirect, got %d", rec.Code)
	}
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, serveTokenCookieName) {
		t.Errorf("token handoff must set the token cookie; got %q", setCookie)
	}
	if strings.Contains(rec.Header().Get("Location"), "token=") {
		t.Errorf("redirect target must not contain the token")
	}
}

func TestSecurityMiddlewareHealthzBypassesAuthentication(t *testing.T) {
	cfg := serveConfig{listenAddr: "0.0.0.0:8085", allowRemote: true, token: "abc123", loopback: false}
	h, reached := okHandler()
	mw := securityMiddleware(cfg, h)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz code=%d, want 200", rec.Code)
	}
	if *reached {
		t.Fatal("healthz must be handled before the application handler")
	}
}

func TestTokenHandoffCookieSecurityUsesExplicitConfig(t *testing.T) {
	cfg := serveConfig{listenAddr: "0.0.0.0:8085", allowRemote: true, token: "abc123", loopback: false, secureCookie: true}
	h, _ := okHandler()
	mw := securityMiddleware(cfg, h)

	proxied := httptest.NewRecorder()
	proxiedReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8085/?token=abc123", nil)
	mw.ServeHTTP(proxied, proxiedReq)
	if !strings.Contains(proxied.Header().Get("Set-Cookie"), "; Secure") {
		t.Fatalf("request Host must not disable Secure behind a proxy: %q", proxied.Header().Get("Set-Cookie"))
	}

	insecureCfg := cfg
	insecureCfg.secureCookie = false
	insecure := httptest.NewRecorder()
	insecureMW := securityMiddleware(insecureCfg, h)
	insecureMW.ServeHTTP(insecure, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8085/?token=abc123", nil))
	if strings.Contains(insecure.Header().Get("Set-Cookie"), "; Secure") {
		t.Fatalf("explicit local HTTP mode must omit Secure: %q", insecure.Header().Get("Set-Cookie"))
	}
}

func TestBrowserAddrRewritesUnspecifiedHosts(t *testing.T) {
	tests := map[string]string{
		"0.0.0.0:8085": "127.0.0.1:8085",
		":8085":        "127.0.0.1:8085",
		"[::]:8085":    "[::1]:8085",
		"127.0.0.1:80": "127.0.0.1:80",
	}
	for input, want := range tests {
		if got := browserAddr(input); got != want {
			t.Errorf("browserAddr(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestSecurityMiddlewareCSRFOnPost(t *testing.T) {
	// Loopback, no token: state-changing POST without Origin is allowed (local
	// CLI tooling), but a cross-origin Origin is rejected.
	cfg := serveConfig{listenAddr: "127.0.0.1:8085", loopback: true}
	h, reached := okHandler()
	mw := securityMiddleware(cfg, h)

	// Same-host Origin → allowed.
	*reached = false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Host = "127.0.0.1:8085"
	req.Header.Set("Origin", "http://127.0.0.1:8085")
	mw.ServeHTTP(rec, req)
	if !*reached {
		t.Errorf("same-origin POST must be allowed")
	}

	// Cross-origin → 403.
	*reached = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Host = "127.0.0.1:8085"
	req.Header.Set("Origin", "http://evil.example.com")
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || *reached {
		t.Errorf("cross-origin POST must be rejected; code=%d reached=%v", rec.Code, *reached)
	}
}

func TestSecurityMiddlewareRejectsDNSRebindingHost(t *testing.T) {
	cfg := serveConfig{listenAddr: "127.0.0.1:8085", loopback: true}
	h, reached := okHandler()
	mw := securityMiddleware(cfg, h)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/generate", nil)
		req.Host = "attacker.example:8085"
		req.Header.Set("Origin", "http://attacker.example:8085")
		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusMisdirectedRequest || *reached {
			t.Fatalf("%s with rebound Host reached the handler: code=%d reached=%v", method, rec.Code, *reached)
		}
	}
}

func TestLoopbackRequestHostAllowsLocalAliases(t *testing.T) {
	cfg := serveConfig{listenAddr: "127.0.0.1:8085", loopback: true}
	for _, host := range []string{"localhost:8085", "127.0.0.1:8085", "127.0.0.2:8085", "[::1]:8085"} {
		if !loopbackRequestHostOK(cfg, host) {
			t.Errorf("loopback Host %q was rejected", host)
		}
	}
	for _, host := range []string{"attacker.example:8085", "localhost:9090", "127.0.0.1", "[::1]:9090"} {
		if loopbackRequestHostOK(cfg, host) {
			t.Errorf("invalid loopback Host %q was accepted", host)
		}
	}
}

func TestSecurityMiddlewareRemoteRequiresOriginOnPost(t *testing.T) {
	// Remote mode: a POST with no Origin/Referer must be rejected even with a
	// valid token (defense in depth against non-browser CSRF replay).
	cfg := serveConfig{listenAddr: "0.0.0.0:8085", allowRemote: true, token: "tok", loopback: false}
	h, reached := okHandler()
	mw := securityMiddleware(cfg, h)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate", nil)
	req.Header.Set("Authorization", "Bearer tok")
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || *reached {
		t.Errorf("remote POST without Origin must be rejected; code=%d reached=%v", rec.Code, *reached)
	}
}

func TestClearSecretsDropsSensitiveEnv(t *testing.T) {
	s := &SessionData{
		Profile: "production",
		Modes:   []string{"traefik"},
		EnvOverrides: map[string]string{
			"AUTH_HOST":       "auth.example.com",
			"HERALD_API_KEY":  "supersecret",
			"WARDEN_PASSWORD": "hunter2",
			"PASSWORDS":       "bcrypt:...",
		},
		KeysOverrides: map[string]string{"HERALD_HMAC_SECRET": "s3cr3t"},
	}
	s.ClearSecrets()
	if s.KeysOverrides != nil {
		t.Errorf("KeysOverrides must be cleared, got %v", s.KeysOverrides)
	}
	if _, ok := s.EnvOverrides["HERALD_API_KEY"]; ok {
		t.Errorf("secret HERALD_API_KEY must be removed")
	}
	if _, ok := s.EnvOverrides["WARDEN_PASSWORD"]; ok {
		t.Errorf("secret WARDEN_PASSWORD must be removed")
	}
	if _, ok := s.EnvOverrides["PASSWORDS"]; ok {
		t.Errorf("secret PASSWORDS must be removed")
	}
	if v := s.EnvOverrides["AUTH_HOST"]; v != "auth.example.com" {
		t.Errorf("non-secret AUTH_HOST must be retained, got %q", v)
	}
	if s.Profile != "production" {
		t.Errorf("non-secret wizard state must be retained")
	}
}

func TestHostFromURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:8085":        "127.0.0.1:8085",
		"https://example.com/path?q=1": "example.com",
		"http://localhost:8085/x":      "localhost:8085",
	}
	for in, want := range cases {
		if got := hostFromURL(in); got != want {
			t.Errorf("hostFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}
