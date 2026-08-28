// Package policy: Validate enforces a profile's rules against the effective
// configuration (env + generation options). In strict mode (test / production)
// violations are ERRORS, never warnings — production must not be bypassable by
// "continue anyway". Both CLI (`validate --profile --strict`) and Web UI reuse
// this so enforcement is identical.
package policy

import (
	"fmt"
	"strings"

	"github.com/soulteary/stargate-suite/internal/composegen"
)

// Finding is one validation result. IsError marks it as a hard failure (strict
// rule violation) versus an advisory warning (dev-experience hint). Code is a
// stable, machine-readable identifier (e.g. STARGATE_SESSION_EXCHANGE_SECRET_
// REQUIRED) so `validate --strict --json` output is scriptable; Field names the
// offending env key and Profile records the profile the rule ran under.
type Finding struct {
	Code    string
	Key     string
	Field   string
	Profile string
	Message string
	IsError bool
}

func (f Finding) String() string {
	kind := "warning"
	if f.IsError {
		kind = "error"
	}
	key := f.Key
	if key == "" {
		key = f.Field
	}
	if f.Code != "" && key != "" {
		return fmt.Sprintf("%s [%s]: %s (%s)", kind, f.Code, f.Message, key)
	}
	if key != "" {
		return fmt.Sprintf("%s: %s (%s)", kind, f.Message, key)
	}
	return fmt.Sprintf("%s: %s", kind, f.Message)
}

// weakOrTestValues are placeholder/test secrets that must never reach a
// production deployment. Matching is case-insensitive and intentionally
// catches the common separators used by generated templates (CHANGE_ME,
// REPLACE-WITH, and so on).
var weakOrTestValues = []string{
	"test-herald-api-key", "test-warden-api-key", "test-hmac-secret",
	"test-redis-password", "changeme", "change_me", "change-me",
	"replace_with", "replace-with", "replacewith", "placeholder", "example",
	"dummy", "sample",
}

// looksWeakOrTest reports whether v is empty, a known test/placeholder value,
// or clearly a non-production stub (contains "test"/"dummy"/"sample").
func looksWeakOrTest(v string) bool {
	t := strings.ToLower(strings.TrimSpace(v))
	if t == "" {
		return true
	}
	for _, w := range weakOrTestValues {
		if strings.Contains(t, w) {
			return true
		}
	}
	return strings.Contains(t, "test")
}

// isPlaintextPasswords reports whether a PASSWORDS value uses the plaintext
// algorithm (e.g. "plaintext:test1234|test1337").
func isPlaintextPasswords(v string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "plaintext:")
}

func isHashedPasswords(v string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "bcrypt:")
}

// Validate checks env + opts against profile p. Findings are returned in a
// stable order. In strict profiles, rule violations carry IsError=true.
//
// The production strict rules (all real errors) are:
//   - PASSWORDS must not be plaintext (and must be set).
//   - Service keys (Herald/Warden API keys, HMAC secret) must not be
//     test/placeholder/empty.
//   - Internal services + Redis must not publish host ports (ExposePorts off).
//   - Cookie Secure must be on.
//   - HMAC v1 must be forbidden (HMAC_V1_ENABLED not "true").
//   - Redis password required.
func Validate(p Profile, env map[string]string, opts *composegen.Options) []Finding {
	p = p.enforceCanonicalSecurity()
	var findings []Finding
	strict := p.Strict()
	get := func(k string) string { return strings.TrimSpace(env[k]) }
	has := func(k string) bool { return get(k) != "" }

	// Rule helper: emit error in strict profiles, warning otherwise (unless
	// forceError is set — some rules are always errors when they apply).
	add := func(code, key, msg string, forceError bool) {
		findings = append(findings, Finding{
			Code: code, Key: key, Field: key, Profile: p.Name,
			Message: msg, IsError: strict || forceError,
		})
	}

	// =====================================================================
	// Layer 1 — field TYPES: only validate a field's shape when it is set.
	// Type errors are always hard (a malformed port/URL/duration is wrong in
	// every profile), so they force-error regardless of strictness.
	// =====================================================================
	if v := get(EnvHmacMaxDrift); v != "" && !isValidDuration(v) {
		add(CodeHmacDriftInvalid, EnvHmacMaxDrift, "HMAC_MAX_DRIFT must be a duration (e.g. 60s)", true)
	}
	for _, k := range []string{EnvCookieSecure, EnvHmacV1Enabled, EnvPasswordHeaderAuth, EnvStepUpEnabled, EnvHeraldTestMode} {
		if v := get(k); v != "" && !isValidBool(v) {
			add(CodeBoolInvalid, k, k+" must be a boolean (true/false)", true)
		}
	}
	if v := get(EnvTrustedProxies); v != "" && !isValidCIDRList(v) {
		add(CodeTrustedProxiesInvalid, EnvTrustedProxies, "TRUSTED_PROXIES must be a comma-separated list of IPs/CIDRs", true)
	}
	if v := get(EnvHeraldTrustedProxies); v != "" && !isValidCIDRList(v) {
		add(CodeTrustedProxiesInvalid, EnvHeraldTrustedProxies, "HERALD_TRUSTED_PROXIES must be a comma-separated list of IPs/CIDRs", true)
	}
	if v := get(EnvCallbackAllowedHosts); v != "" && !isValidHostList(v) {
		add(CodeCallbackHostsInvalid, EnvCallbackAllowedHosts, "CALLBACK_ALLOWED_HOSTS must be a comma-separated list of hosts (no scheme)", true)
	}

	// =====================================================================
	// Layer 2 — single-field SAFETY (secret strength, no plaintext, no
	// placeholder values). Gated by the profile's strategy.
	// =====================================================================
	// --- Passwords ---
	pw := get(EnvPasswords)
	if p.PasswordAlgorithm == PasswordForbidPlaintext {
		if pw == "" {
			add(CodePasswordsRequired, EnvPasswords, "PASSWORDS must be provided (production forbids the default plaintext test password)", false)
		} else if isPlaintextPasswords(pw) {
			add(CodePasswordsPlaintext, EnvPasswords, "PASSWORDS uses plaintext algorithm; production requires bcrypt", false)
		} else if !isHashedPasswords(pw) || looksWeakOrTest(pw) {
			add(CodePasswordsHashRequired, EnvPasswords, "PASSWORDS must contain a real bcrypt credential, not a placeholder or an unqualified value", false)
		}
	}

	// --- Service-to-service keys ---
	if p.SecretSource == SecretUserProvidedOrFile {
		if !strongCredential(get(EnvWardenAPIKey), minAPIKeyLen) {
			add(CodeWardenAPIKeyWeak, EnvWardenAPIKey, "WARDEN_API_KEY must be a user-provided key of at least 16 characters (test/placeholder/empty rejected)", false)
		}
		hmac := get(EnvHeraldHmacSecret)
		if hmac == "" {
			hmac = get(EnvHmacSecret)
		}
		hasHeraldMtls := has(EnvHeraldTLSClientCert) && has(EnvHeraldTLSClientKey)
		requireHmac := p.HeraldAuth == HeraldHmacV2 ||
			(p.HeraldAuth == HeraldHmacV2OrMtls && !hasHeraldMtls)
		if requireHmac && !strongSecret(hmac) {
			add(CodeHmacSecretWeak, EnvHeraldHmacSecret, "HMAC secret must be a strong user-provided value of at least 32 characters (test/placeholder/empty rejected)", false)
		} else if hmac != "" && !strongSecret(hmac) {
			add(CodeHmacSecretWeak, EnvHeraldHmacSecret, "configured HMAC secret must be at least 32 characters and not be a placeholder", false)
		}
		// Herald v1.1 PII pepper + idempotency secret must be strong when set;
		// production requires them to be set (fail-closed on missing).
		if !strongSecret(get(EnvHeraldPIIPepper)) {
			add(CodePIIPepperWeak, EnvHeraldPIIPepper, "HERALD_PII_PEPPER must be a strong secret of at least 32 chars in production", false)
		}
		if !strongSecret(get(EnvHeraldIdempotencySecr)) {
			add(CodeIdempotencySecretWeak, EnvHeraldIdempotencySecr, "HERALD_IDEMPOTENCY_SECRET must be a strong secret of at least 32 chars in production", false)
		}
	}

	// --- Port exposure ---
	if p.PortBinding == PortReverseProxyOnly && opts != nil && opts.ExposePorts {
		add(CodeExposePortsForbidden, "exposePorts", "internal services and Redis must not publish host ports in production (only the reverse-proxy entrypoint is exposed)", false)
	}

	// --- Cookie Secure ---
	if p.CookieSecure == CookieRequired && !isTrue(get(EnvCookieSecure)) {
		add(CodeCookieSecureRequired, EnvCookieSecure, "Cookie Secure must be enabled in production (COOKIE_SECURE=true)", false)
	}

	// --- HMAC v1 (forbidden in every profile; always an error) ---
	if isTrue(get(EnvHmacV1Enabled)) {
		add(CodeHmacV1Forbidden, EnvHmacV1Enabled, "HMAC v1 is forbidden (HMAC_V1_ENABLED must not be true)", true)
	}

	// --- Redis password ---
	if p.RedisPassword == RedisRequired {
		if !strongCredential(get(EnvHeraldRedisPassword), minRedisPasswordLen) {
			add(CodeRedisPasswordRequired, EnvHeraldRedisPassword, "HERALD_REDIS_PASSWORD must be a user-provided password of at least 16 characters", false)
		}
		if !strongCredential(get(EnvWardenRedisPassword), minRedisPasswordLen) {
			add(CodeRedisPasswordRequired, EnvWardenRedisPassword, "WARDEN_REDIS_PASSWORD must be a user-provided password of at least 16 characters", false)
		}
	}

	// --- Herald test mode ---
	if p.HeraldTestAPI == HeraldTestAPIForbidden && isTrue(get(EnvHeraldTestMode)) {
		add(CodeHeraldTestModeForbidden, EnvHeraldTestMode, "Herald test mode is forbidden in production (HERALD_TEST_MODE must not be true)", true)
	}

	// --- Container least privilege (S-01/S-03) ---
	if p.ContainerPrivileges == PrivLeastPrivilegeReadonly && opts != nil {
		if !opts.LeastPrivilege {
			add(CodeContainerPrivileges, "containerPrivileges", "production requires least-privilege containers (cap_drop ALL, no-new-privileges)", false)
		}
		if !opts.ReadOnlyRootFS {
			add(CodeContainerReadonly, "containerPrivileges", "production requires a read-only root filesystem on containers", false)
		}
	}

	// =====================================================================
	// Layer 3 — CROSS-FIELD rules (interactions between two+ fields).
	// =====================================================================
	// Cross-domain callback / cookie sharing needs a strong session-exchange
	// secret: if CALLBACK_ALLOWED_HOSTS or a cross-domain COOKIE_DOMAIN is set,
	// SESSION_EXCHANGE_SECRET must be >= 32 random chars.
	crossDomain := has(EnvCallbackAllowedHosts) || has(EnvCookieDomain)
	if crossDomain && !strongSecret(get(EnvSessionExchangeSecret)) {
		add(CodeSessionExchangeSecret, EnvSessionExchangeSecret, "cross-domain callback/cookie sharing requires SESSION_EXCHANGE_SECRET of at least 32 characters", false)
	}

	// step-up requires both the guarded paths and a trusted-proxy list (so the
	// client IP used for step-up decisions is trustworthy).
	if isTrue(get(EnvStepUpEnabled)) {
		if get(EnvStepUpPaths) == "" {
			add(CodeStepUpPathsRequired, EnvStepUpPaths, "STEP_UP_ENABLED=true requires STEP_UP_PATHS to be non-empty", false)
		}
		if get(EnvTrustedProxies) == "" {
			add(CodeStepUpProxiesRequired, EnvTrustedProxies, "STEP_UP_ENABLED=true requires TRUSTED_PROXIES so the client IP is trustworthy", false)
		}
	}

	// TLS cert/key must be configured as a pair (Stargate->Warden client mTLS,
	// Stargate->Herald client mTLS). A half-configured pair is always an error.
	checkPair := func(code, certKey, keyKey string) {
		c, k := has(certKey), has(keyKey)
		if c != k {
			missing := keyKey
			if !c {
				missing = certKey
			}
			add(code, missing, "TLS client cert/key must be configured as a pair ("+certKey+" + "+keyKey+")", true)
		}
	}
	checkPair(CodeTLSPairIncomplete, EnvWardenTLSClientCertFile, EnvWardenTLSClientKeyFile)
	checkPair(CodeTLSPairIncomplete, EnvHeraldTLSClientCert, EnvHeraldTLSClientKey)

	// =====================================================================
	// Layer 4 — CROSS-SERVICE rules: Stargate's outbound auth toward Herald
	// must match a single, explicit server-side mode (HMAC v2 or mTLS or API
	// key) rather than being inferred from several fields at once.
	// =====================================================================
	findings = append(findings, validateCrossServiceAuth(p, env)...)

	return findings
}

// validateCrossServiceAuth enforces that Stargate's client auth toward Herald
// resolves to the mode declared by the profile. The built-in production
// profile requires HMAC v2; the legacy hmacV2OrMtls strategy remains available
// to custom profiles and keeps its explicit ambiguity checks.
func validateCrossServiceAuth(p Profile, env map[string]string) []Finding {
	var out []Finding
	strict := p.Strict()
	get := func(k string) string { return strings.TrimSpace(env[k]) }
	add := func(code, key, msg string, forceError bool) {
		out = append(out, Finding{
			Code: code, Key: key, Field: key, Profile: p.Name,
			Message: msg, IsError: strict || forceError,
		})
	}

	hasHmac := get(EnvHeraldHmacSecret) != "" || get(EnvHmacSecret) != ""
	hasMtls := get(EnvHeraldTLSClientCert) != "" && get(EnvHeraldTLSClientKey) != ""
	hasAPIKey := get(EnvHeraldAPIKey) != ""
	mode := strings.ToLower(get(EnvRequestAuthMode))

	switch p.HeraldAuth {
	case HeraldHmacV2:
		if !hasHmac {
			add(CodeAuthModeMismatch, EnvRequestAuthMode, "production Herald auth requires HMAC v2 (HERALD_HMAC_SECRET)", false)
		}
		if mode != "" && mode != "hmac_v2" {
			add(CodeAuthModeMismatch, EnvRequestAuthMode, "REQUEST_AUTH_MODE must be hmac_v2 in production (got "+mode+")", false)
		}
	case HeraldHmacV2OrMtls:
		// production: must use HMAC v2 or mTLS; API-key-only is rejected.
		if !hasHmac && !hasMtls {
			add(CodeAuthModeMismatch, EnvRequestAuthMode, "production Herald auth requires HMAC v2 (HERALD_HMAC_SECRET) or mTLS client cert; API-key-only is not allowed", false)
		}
		if mode != "" && mode != "hmac_v2" && mode != "mtls" {
			add(CodeAuthModeMismatch, EnvRequestAuthMode, "REQUEST_AUTH_MODE must be hmac_v2 or mtls in production (got "+mode+")", false)
		}
		// Ambiguity: both mTLS and HMAC present but no explicit mode selected.
		if hasHmac && hasMtls && mode == "" {
			add(CodeAuthModeAmbiguous, EnvRequestAuthMode, "both mTLS and HMAC are configured; set REQUEST_AUTH_MODE explicitly to avoid ambiguous auth", false)
		}
	case HeraldTestAPIKeyOrHmacV2:
		// test: HMAC v2 or a dedicated test API key; never HMAC v1.
		if !hasHmac && !hasAPIKey {
			add(CodeAuthModeMismatch, EnvRequestAuthMode, "test Herald auth requires HMAC v2 or a test API key", false)
		}
	}
	return out
}

// HasErrors reports whether any finding is an error.
func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.IsError {
			return true
		}
	}
	return false
}

func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}
