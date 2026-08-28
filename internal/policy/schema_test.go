package policy

import (
	"strings"
	"testing"

	"github.com/soulteary/stargate-suite/internal/composegen"
)

// findByCode returns the first finding with the given code, or false.
func findByCode(fs []Finding, code string) (Finding, bool) {
	for _, f := range fs {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

// TestLayer1FieldTypes: malformed field shapes are hard errors in every profile.
func TestLayer1FieldTypes(t *testing.T) {
	p := getProfile(t, Development) // non-strict, but type errors force-error
	cases := []struct {
		env  map[string]string
		code string
	}{
		{map[string]string{EnvHmacMaxDrift: "sixty"}, CodeHmacDriftInvalid},
		{map[string]string{EnvCookieSecure: "maybe"}, CodeBoolInvalid},
		{map[string]string{EnvTrustedProxies: "not-an-ip"}, CodeTrustedProxiesInvalid},
		{map[string]string{EnvHeraldTrustedProxies: "999.1.1.1/33"}, CodeTrustedProxiesInvalid},
		{map[string]string{EnvCallbackAllowedHosts: "http://evil.com"}, CodeCallbackHostsInvalid},
	}
	for _, tc := range cases {
		findings := Validate(p, tc.env, baseOpts())
		f, ok := findByCode(findings, tc.code)
		if !ok {
			t.Errorf("env %v: expected code %q", tc.env, tc.code)
			continue
		}
		if !f.IsError {
			t.Errorf("code %q must be a hard error (type errors are absolute)", tc.code)
		}
	}
}

// TestLayer1FieldTypesAcceptValid: well-formed values produce no type findings.
func TestLayer1FieldTypesAcceptValid(t *testing.T) {
	p := getProfile(t, Development)
	env := map[string]string{
		EnvHmacMaxDrift:         "60s",
		EnvCookieSecure:         "true",
		EnvTrustedProxies:       "10.0.0.0/8,192.168.1.1",
		EnvHeraldTrustedProxies: "172.16.0.0/12",
		EnvCallbackAllowedHosts: "auth.example.com,*.example.com:8443",
	}
	findings := Validate(p, env, baseOpts())
	for _, code := range []string{CodeHmacDriftInvalid, CodeBoolInvalid, CodeTrustedProxiesInvalid, CodeCallbackHostsInvalid} {
		if _, ok := findByCode(findings, code); ok {
			t.Errorf("valid config should not produce %q", code)
		}
	}
}

// TestLayer2SecretStrength: production rejects weak/short PII pepper + idempotency.
func TestLayer2SecretStrength(t *testing.T) {
	p := getProfile(t, Production)
	env := map[string]string{
		EnvHeraldPIIPepper:       "short",
		EnvHeraldIdempotencySecr: "",
	}
	findings := Validate(p, env, prodOpts())
	if _, ok := findByCode(findings, CodePIIPepperWeak); !ok {
		t.Errorf("expected %q for short pepper", CodePIIPepperWeak)
	}
	if _, ok := findByCode(findings, CodeIdempotencySecretWeak); !ok {
		t.Errorf("expected %q for missing idempotency secret", CodeIdempotencySecretWeak)
	}
}

// TestLayer3CrossFieldSessionExchange: cross-domain callback requires a strong
// SESSION_EXCHANGE_SECRET.
func TestLayer3CrossFieldSessionExchange(t *testing.T) {
	p := getProfile(t, Production)
	// callback hosts set but no session-exchange secret -> error.
	env := prodStrongEnv()
	env[EnvCallbackAllowedHosts] = "app.example.com"
	delete(env, EnvSessionExchangeSecret)
	findings := Validate(p, env, prodOpts())
	if _, ok := findByCode(findings, CodeSessionExchangeSecret); !ok {
		t.Fatalf("expected %q when callback hosts set without secret", CodeSessionExchangeSecret)
	}
	// with a strong secret it passes.
	env[EnvSessionExchangeSecret] = "exchange-secret-0011223344556677889900"
	findings = Validate(p, env, prodOpts())
	if _, ok := findByCode(findings, CodeSessionExchangeSecret); ok {
		t.Errorf("strong SESSION_EXCHANGE_SECRET should satisfy the cross-domain rule")
	}
}

// TestLayer3StepUpRequiresPathsAndProxies.
func TestLayer3StepUpRequiresPathsAndProxies(t *testing.T) {
	p := getProfile(t, Production)
	env := prodStrongEnv()
	env[EnvStepUpEnabled] = "true" // paths + proxies missing
	findings := Validate(p, env, prodOpts())
	if _, ok := findByCode(findings, CodeStepUpPathsRequired); !ok {
		t.Errorf("step-up without paths must error")
	}
	if _, ok := findByCode(findings, CodeStepUpProxiesRequired); !ok {
		t.Errorf("step-up without trusted proxies must error")
	}
}

// TestLayer3TLSPairIncomplete: a half-configured client cert/key pair is a hard
// error in every profile.
func TestLayer3TLSPairIncomplete(t *testing.T) {
	p := getProfile(t, Development)
	env := map[string]string{EnvWardenTLSClientCertFile: "/certs/warden.crt"} // no key
	findings := Validate(p, env, baseOpts())
	f, ok := findByCode(findings, CodeTLSPairIncomplete)
	if !ok || !f.IsError {
		t.Fatalf("incomplete TLS pair must be a hard error")
	}
}

// TestLayer4CrossServiceAuthMismatch: production Herald auth requires HMAC v2 or
// mTLS; API-key-only is rejected.
func TestLayer4CrossServiceAuthMismatch(t *testing.T) {
	p := getProfile(t, Production)
	env := prodStrongEnv()
	// remove HMAC + mTLS, leave only API key.
	delete(env, EnvHeraldHmacSecret)
	delete(env, EnvHmacSecret)
	env[EnvRequestAuthMode] = ""
	findings := Validate(p, env, prodOpts())
	if _, ok := findByCode(findings, CodeAuthModeMismatch); !ok {
		t.Fatalf("API-key-only Herald auth must be rejected in production")
	}
}

// TestStructuredCodesPopulated: every finding carries a non-empty Code and the
// profile name, so `validate --json` is scriptable.
func TestStructuredCodesPopulated(t *testing.T) {
	p := getProfile(t, Production)
	findings := Validate(p, map[string]string{EnvHmacV1Enabled: "true"}, prodOpts())
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	for _, f := range findings {
		if f.Code == "" {
			t.Errorf("finding %q has empty Code", f.Message)
		}
		if f.Profile != Production {
			t.Errorf("finding %q profile = %q, want production", f.Code, f.Profile)
		}
	}
}

// prodOpts is a production-hardened options baseline (no host ports, least
// privilege + read-only root) so container/port rules do not fire spuriously.
func prodOpts() *composegen.Options {
	o := baseOpts()
	o.ExposePorts = false
	o.LeastPrivilege = true
	o.ReadOnlyRootFS = true
	return o
}

// prodStrongEnv returns a production-valid env satisfying all strict rules, so
// individual tests can flip one field to assert a specific rule.
func prodStrongEnv() map[string]string {
	return map[string]string{
		EnvPasswords:             "bcrypt:$2a$10$abcdefghijklmnopqrstuv",
		EnvHeraldAPIKey:          "9f8e7d6c5b4a39281706",
		EnvWardenAPIKey:          "abcdef0123456789abcdef",
		EnvHeraldHmacSecret:      "0011223344556677889900112233445566778899",
		EnvHmacV1Enabled:         "false",
		EnvHeraldTestMode:        "false",
		EnvCookieSecure:          "true",
		EnvHeraldRedisPassword:   "s3cr3t-redis-herald",
		EnvWardenRedisPassword:   "s3cr3t-redis-warden",
		EnvHeraldPIIPepper:       "pepper-0011223344556677889900112233",
		EnvHeraldIdempotencySecr: "idem-0011223344556677889900112233aa",
		EnvRequestAuthMode:       "hmac_v2",
	}
}

// TestStrongEnvHasNoErrors: the shared strong env must pass cleanly (guards the
// other tests' baseline).
func TestStrongEnvHasNoErrors(t *testing.T) {
	p := getProfile(t, Production)
	findings := Validate(p, prodStrongEnv(), prodOpts())
	if HasErrors(findings) {
		for _, f := range findings {
			if f.IsError {
				t.Logf("unexpected error: %s", f.String())
			}
		}
		t.Fatal("prodStrongEnv should have no errors")
	}
}

// TestValidatePlaintextDriftDoesNotAffectValidBool: ensure bool literal variants
// are accepted (regression for isValidBool).
func TestValidBoolVariants(t *testing.T) {
	for _, v := range []string{"true", "false", "1", "0", "yes", "no", "on", "off", "TRUE", "Off"} {
		if !isValidBool(v) {
			t.Errorf("isValidBool(%q) = false, want true", v)
		}
	}
	if isValidBool("maybe") {
		t.Errorf("isValidBool(maybe) = true, want false")
	}
	_ = strings.TrimSpace
}
