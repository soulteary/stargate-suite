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
// rule violation) versus an advisory warning (dev-experience hint).
type Finding struct {
	Key     string
	Message string
	IsError bool
}

func (f Finding) String() string {
	kind := "warning"
	if f.IsError {
		kind = "error"
	}
	if f.Key != "" {
		return fmt.Sprintf("%s: %s (%s)", kind, f.Message, f.Key)
	}
	return fmt.Sprintf("%s: %s", kind, f.Message)
}

// weakOrTestValues are placeholder/test secrets that must never reach a
// production deployment. Matching is case-insensitive substring for the "test"
// family plus exact well-known defaults.
var weakOrTestValues = []string{
	"test-herald-api-key", "test-warden-api-key", "test-hmac-secret",
	"test-redis-password", "changeme", "placeholder", "example",
}

// looksWeakOrTest reports whether v is empty, a known test/placeholder value,
// or clearly a non-production stub (contains "test"/"dummy"/"sample").
func looksWeakOrTest(v string) bool {
	t := strings.ToLower(strings.TrimSpace(v))
	if t == "" {
		return true
	}
	for _, w := range weakOrTestValues {
		if t == w {
			return true
		}
	}
	return strings.Contains(t, "test") || strings.Contains(t, "dummy") || strings.Contains(t, "sample")
}

// isPlaintextPasswords reports whether a PASSWORDS value uses the plaintext
// algorithm (e.g. "plaintext:test1234|test1337").
func isPlaintextPasswords(v string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "plaintext:")
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
	var findings []Finding
	strict := p.Strict()
	get := func(k string) string { return strings.TrimSpace(env[k]) }

	// A single rule helper: emit error in strict profiles, warning otherwise
	// (unless forceError is set — some rules are always errors when they apply).
	add := func(key, msg string, forceError bool) {
		findings = append(findings, Finding{Key: key, Message: msg, IsError: strict || forceError})
	}

	// --- Passwords --------------------------------------------------------
	pw := get(EnvPasswords)
	if p.PasswordAlgorithm == PasswordForbidPlaintext {
		if pw == "" {
			add(EnvPasswords, "PASSWORDS must be provided (production forbids the default plaintext test password)", false)
		} else if isPlaintextPasswords(pw) {
			add(EnvPasswords, "PASSWORDS uses plaintext algorithm; production requires bcrypt/argon2/etc.", false)
		}
	} else if pw != "" && isPlaintextPasswords(pw) && strict {
		// test profile allows test passwords; plaintext is acceptable there.
		_ = pw
	}

	// --- Service-to-service keys -----------------------------------------
	if p.SecretSource == SecretUserProvidedOrFile {
		for _, k := range []string{EnvHeraldAPIKey, EnvWardenAPIKey} {
			if looksWeakOrTest(get(k)) {
				add(k, k+" must be a user-provided key or secret-file reference (test/placeholder/empty rejected)", false)
			}
		}
		hmac := get(EnvHeraldHmacSecret)
		if hmac == "" {
			hmac = get(EnvHmacSecret)
		}
		if looksWeakOrTest(hmac) {
			add(EnvHeraldHmacSecret, "HMAC secret must be a strong user-provided value (test/placeholder/empty rejected)", false)
		}
	}

	// --- Port exposure ----------------------------------------------------
	if p.PortBinding == PortReverseProxyOnly && opts != nil && opts.ExposePorts {
		add("exposePorts", "internal services and Redis must not publish host ports in production (only the reverse-proxy entrypoint is exposed)", false)
	}

	// --- Cookie Secure ----------------------------------------------------
	if p.CookieSecure == CookieRequired {
		if !isTrue(get(EnvCookieSecure)) {
			add(EnvCookieSecure, "Cookie Secure must be enabled in production (COOKIE_SECURE=true)", false)
		}
	}

	// --- HMAC v1 ----------------------------------------------------------
	// HMAC v1 is forbidden in every profile; enabling it is always an error.
	if isTrue(get(EnvHmacV1Enabled)) {
		add(EnvHmacV1Enabled, "HMAC v1 is forbidden (HMAC_V1_ENABLED must not be true)", true)
	}

	// --- Redis password ---------------------------------------------------
	if p.RedisPassword == RedisRequired {
		for _, k := range []string{EnvHeraldRedisPassword, EnvWardenRedisPassword} {
			if get(k) == "" {
				add(k, k+" is required in production (Redis must be authenticated)", false)
			}
		}
	}

	// --- Herald test API --------------------------------------------------
	if p.HeraldTestAPI == HeraldTestAPIForbidden && isTrue(get(EnvHeraldTestMode)) {
		add(EnvHeraldTestMode, "Herald test mode is forbidden in production (HERALD_TEST_MODE must not be true)", true)
	}

	return findings
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
