package policy

import (
	"testing"
)

// This file holds PR9 production-reject / contract tests for the shared
// validator (the same policy.Validate used by CLI and Web UI). They cover the
// validation layers not already exercised by policy_test.go:
//   - Layer 1 (field type) errors fire in EVERY profile (malformed shape is
//     always wrong, never a warning).
//   - Layer 3 (cross-field) rules: session-exchange secret, step-up prereqs,
//     TLS client cert/key pairing.
//   - Layer 4 (cross-service) Herald auth resolution: production requires the
//     generated HMAC v2 path and rejects API-key-only or mTLS-only input.

// findingByCode returns the first finding with the given code, or a zero
// Finding and false.
func findingByCode(findings []Finding, code string) (Finding, bool) {
	for _, f := range findings {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

// TestLayer1TypeErrorsFireInEveryProfile asserts malformed field shapes are
// hard errors regardless of profile strictness (development included) — a bad
// duration/bool/CIDR/host is objectively wrong everywhere.
func TestLayer1TypeErrorsFireInEveryProfile(t *testing.T) {
	cases := []struct {
		key  string
		val  string
		code string
	}{
		{EnvHmacMaxDrift, "sixty-seconds", CodeHmacDriftInvalid},
		{EnvHmacV1Enabled, "notabool", CodeBoolInvalid},
		{EnvTrustedProxies, "not-an-ip", CodeTrustedProxiesInvalid},
		{EnvHeraldTrustedProxies, "999.999.999.999", CodeTrustedProxiesInvalid},
		{EnvCallbackAllowedHosts, "https://has-scheme.example", CodeCallbackHostsInvalid},
	}
	for _, name := range []string{Development, Test, Production} {
		p := getProfile(t, name)
		for _, tc := range cases {
			env := map[string]string{tc.key: tc.val}
			findings := Validate(p, env, baseOpts())
			f, ok := findingByCode(findings, tc.code)
			if !ok {
				t.Errorf("profile %q: %s=%q must produce a %s finding", name, tc.key, tc.val, tc.code)
				continue
			}
			if !f.IsError {
				t.Errorf("profile %q: %s type error must be a hard error, not a warning", name, tc.key)
			}
		}
	}
}

// TestCrossFieldSessionExchangeSecretRequired asserts that enabling cross-domain
// callback/cookie sharing without a strong SESSION_EXCHANGE_SECRET is rejected
// (Layer 3). A short secret must also be rejected.
func TestCrossFieldSessionExchangeSecretRequired(t *testing.T) {
	p := getProfile(t, Production)

	// Cross-domain via CALLBACK_ALLOWED_HOSTS, missing session-exchange secret.
	env := map[string]string{
		EnvCallbackAllowedHosts: "app.example.com,admin.example.com",
	}
	findings := Validate(p, env, baseOpts())
	if f, ok := findingByCode(findings, CodeSessionExchangeSecret); !ok || !f.IsError {
		t.Errorf("cross-domain callback without SESSION_EXCHANGE_SECRET must be a hard error in production")
	}

	// A too-short secret is still rejected.
	env[EnvSessionExchangeSecret] = "short"
	findings = Validate(p, env, baseOpts())
	if f, ok := findingByCode(findings, CodeSessionExchangeSecret); !ok || !f.IsError {
		t.Errorf("short SESSION_EXCHANGE_SECRET must be rejected under cross-domain sharing")
	}

	// A strong secret clears the finding.
	env[EnvSessionExchangeSecret] = "sxc-0011223344556677889900112233aabb"
	findings = Validate(p, env, baseOpts())
	if _, ok := findingByCode(findings, CodeSessionExchangeSecret); ok {
		t.Errorf("strong SESSION_EXCHANGE_SECRET should clear the cross-domain finding")
	}
}

// TestCrossFieldStepUpRequiresPathsAndProxies asserts STEP_UP_ENABLED=true
// requires both STEP_UP_PATHS and TRUSTED_PROXIES (so the client IP used for
// step-up decisions is trustworthy) — Layer 3.
func TestCrossFieldStepUpRequiresPathsAndProxies(t *testing.T) {
	p := getProfile(t, Production)
	env := map[string]string{EnvStepUpEnabled: "true"}
	findings := Validate(p, env, baseOpts())
	if f, ok := findingByCode(findings, CodeStepUpPathsRequired); !ok || !f.IsError {
		t.Errorf("STEP_UP_ENABLED=true without STEP_UP_PATHS must be a hard error")
	}
	if f, ok := findingByCode(findings, CodeStepUpProxiesRequired); !ok || !f.IsError {
		t.Errorf("STEP_UP_ENABLED=true without TRUSTED_PROXIES must be a hard error")
	}

	// Supplying both clears the step-up prerequisites.
	env[EnvStepUpPaths] = "/admin,/settings"
	env[EnvTrustedProxies] = "10.0.0.0/8"
	findings = Validate(p, env, baseOpts())
	if _, ok := findingByCode(findings, CodeStepUpPathsRequired); ok {
		t.Errorf("STEP_UP_PATHS supplied should clear the paths finding")
	}
	if _, ok := findingByCode(findings, CodeStepUpProxiesRequired); ok {
		t.Errorf("TRUSTED_PROXIES supplied should clear the proxies finding")
	}
}

// TestCrossFieldTLSPairMustBeComplete asserts a half-configured TLS client
// cert/key pair is always an error (in every profile) — Layer 3.
func TestCrossFieldTLSPairMustBeComplete(t *testing.T) {
	for _, name := range []string{Development, Test, Production} {
		p := getProfile(t, name)
		// Only the cert, no key.
		env := map[string]string{EnvWardenTLSClientCertFile: "/etc/tls/client.crt"}
		findings := Validate(p, env, baseOpts())
		if f, ok := findingByCode(findings, CodeTLSPairIncomplete); !ok || !f.IsError {
			t.Errorf("profile %q: incomplete Warden TLS pair (cert only) must be a hard error", name)
		}

		// Both present clears it.
		env[EnvWardenTLSClientKeyFile] = "/etc/tls/client.key"
		findings = Validate(p, env, baseOpts())
		if _, ok := findingByCode(findings, CodeTLSPairIncomplete); ok {
			// The Herald pair is separate; ensure this specific finding is gone
			// only when neither pair is incomplete. Here we set no Herald pair,
			// so no incomplete finding should remain.
			t.Errorf("profile %q: complete Warden TLS pair should clear the incomplete finding", name)
		}
	}
}

// TestCrossServiceProductionRejectsAPIKeyOnly asserts production Herald auth
// rejects an API-key-only configuration with no HMAC secret — Layer 4.
func TestCrossServiceProductionRejectsAPIKeyOnly(t *testing.T) {
	p := getProfile(t, Production)
	env := map[string]string{
		EnvHeraldAPIKey: "9f8e7d6c5b4a39281706", // strong-looking key, but API-key-only
	}
	findings := Validate(p, env, baseOpts())
	if f, ok := findingByCode(findings, CodeAuthModeMismatch); !ok || !f.IsError {
		t.Errorf("production must reject API-key-only Herald auth (needs HMAC v2)")
	}
}

// TestCrossServiceProductionRejectsMtlsOnly keeps the built-in production
// profile honest: generated deployments do not manage certificate mounts yet,
// so mTLS-only must not be advertised or accepted as the profile auth mode.
func TestCrossServiceProductionRejectsMtlsOnly(t *testing.T) {
	p := getProfile(t, Production)
	env := prodStrongEnv()
	delete(env, EnvHeraldHmacSecret)
	env[EnvHeraldTLSClientCert] = "/etc/tls/herald-client.crt"
	env[EnvHeraldTLSClientKey] = "/etc/tls/herald-client.key"
	env[EnvRequestAuthMode] = "mtls"
	findings := Validate(p, env, baseOpts())
	if f, ok := findingByCode(findings, CodeAuthModeMismatch); !ok || !f.IsError {
		t.Errorf("mTLS-only input must be rejected by the built-in HMAC v2 production profile")
	}

	// Adding HMAC v2 and selecting it allows TLS client material to coexist as
	// transport configuration without changing the request-auth contract.
	env[EnvHeraldHmacSecret] = "0011223344556677889900112233445566778899"
	env[EnvRequestAuthMode] = "hmac_v2"
	findings = Validate(p, env, baseOpts())
	if _, ok := findingByCode(findings, CodeAuthModeMismatch); ok {
		t.Errorf("HMAC v2 with optional TLS transport material should satisfy the profile")
	}
}
