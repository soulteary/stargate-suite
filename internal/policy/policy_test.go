package policy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/soulteary/stargate-suite/internal/composegen"
)

// fixedProfiles returns the three canonical profiles parsed from the committed
// config/profiles.yaml content so tests do not depend on an asset FS. Keeping a
// literal here also documents the exact policy contract PR 5 promises.
func fixedProfiles(t *testing.T) *Profiles {
	t.Helper()
	const y = `
schemaVersion: 1
profiles:
  development:
    description: dev
    experimental: false
    portBinding: loopback
    secretSource: autoGenerateOrInput
    passwordAlgorithm: allowPlaintext
    heraldAuth: apiKeyOptional
    heraldTestApi: disabled
    redisPassword: autoGenerate
    cookieSecure: optional
    hmacV1: forbidden
    containerPrivileges: leastPrivilege
    validationMode: warnAndError
  test:
    description: test
    experimental: false
    portBinding: loopback
    secretSource: deterministicTest
    passwordAlgorithm: allowTest
    heraldAuth: testApiKeyOrHmacV2
    heraldTestApi: loopbackListener
    redisPassword: isolated
    cookieSecure: optional
    hmacV1: forbidden
    containerPrivileges: leastPrivilege
    validationMode: strict
  production:
    description: prod
    experimental: true
    portBinding: reverseProxyOnly
    secretSource: userProvidedOrFile
    passwordAlgorithm: forbidPlaintext
    heraldAuth: hmacV2OrMtls
    heraldTestApi: forbidden
    redisPassword: required
    cookieSecure: required
    hmacV1: forbidden
    containerPrivileges: leastPrivilegeReadonly
    validationMode: strict
`
	ps, err := ParseProfiles([]byte(y))
	if err != nil {
		t.Fatalf("parse profiles: %v", err)
	}
	return ps
}

func getProfile(t *testing.T, name string) Profile {
	t.Helper()
	p, ok := fixedProfiles(t).Get(name)
	if !ok {
		t.Fatalf("profile %q missing", name)
	}
	return p
}

func baseOpts() *composegen.Options {
	return &composegen.Options{
		HealthCheck:    true,
		TraefikNetwork: true,
		ExposePorts:    true,
		UseNamedVolume: true,
	}
}

// TestApplyDevelopmentFoldsCurrentDefaults asserts development keeps the current
// suite defaults (loopback exposure, plaintext test password, dev keys).
func TestApplyDevelopmentFoldsCurrentDefaults(t *testing.T) {
	p := getProfile(t, Development)
	res := Apply(p, baseOpts(), nil, NewKeyGen(bytes.NewReader(bytes.Repeat([]byte{0xAB}, 512))))
	if !res.Options.ExposePorts {
		t.Errorf("development should expose ports (loopback), got ExposePorts=false")
	}
	if !res.Options.LoopbackBind {
		t.Errorf("development should bind to loopback")
	}
	if got := res.EnvOverrides[EnvPasswords]; !strings.HasPrefix(got, "plaintext:") {
		t.Errorf("development PASSWORDS = %q, want plaintext:*", got)
	}
	if res.EnvOverrides[EnvHeraldAPIKey] == "" {
		t.Errorf("development should auto-generate HERALD_API_KEY")
	}
	if res.EnvOverrides[EnvCookieSecure] != "false" {
		t.Errorf("development COOKIE_SECURE = %q, want false", res.EnvOverrides[EnvCookieSecure])
	}
}

// TestApplyTestUsesDeterministicSecrets asserts test uses the isolated
// deterministic test values (no crypto/rand needed).
func TestApplyTestUsesDeterministicSecrets(t *testing.T) {
	p := getProfile(t, Test)
	res := Apply(p, baseOpts(), nil, nil) // nil keygen: test never auto-generates
	if res.EnvOverrides[EnvHeraldAPIKey] != testHeraldKey {
		t.Errorf("test HERALD_API_KEY = %q, want %q", res.EnvOverrides[EnvHeraldAPIKey], testHeraldKey)
	}
	if res.EnvOverrides[EnvHeraldRedisPassword] != testRedisPasswd {
		t.Errorf("test HERALD_REDIS_PASSWORD = %q, want %q", res.EnvOverrides[EnvHeraldRedisPassword], testRedisPasswd)
	}
	if res.EnvOverrides[EnvHeraldTestMode] != "true" {
		t.Errorf("test HERALD_TEST_MODE = %q, want true (loopback listener)", res.EnvOverrides[EnvHeraldTestMode])
	}
	if !res.Options.LoopbackBind || !res.Options.ExposePorts {
		t.Errorf("test should expose loopback ports")
	}
}

// TestApplyProductionInjectsNoWeakSecrets asserts production never injects a
// password / key / redis password (they must be user-provided) and disables
// host port publishing.
func TestApplyProductionInjectsNoWeakSecrets(t *testing.T) {
	p := getProfile(t, Production)
	res := Apply(p, baseOpts(), nil, NewKeyGen(bytes.NewReader(bytes.Repeat([]byte{0x01}, 512))))
	if res.Options.ExposePorts {
		t.Errorf("production must not publish host ports (ExposePorts should be false)")
	}
	for _, k := range []string{EnvPasswords, EnvHeraldAPIKey, EnvWardenAPIKey, EnvHeraldHmacSecret, EnvHeraldRedisPassword, EnvWardenRedisPassword} {
		if v, ok := res.EnvOverrides[k]; ok && strings.TrimSpace(v) != "" {
			t.Errorf("production must not inject %s (got %q); it must be user-provided", k, v)
		}
	}
	if res.EnvOverrides[EnvCookieSecure] != "true" {
		t.Errorf("production COOKIE_SECURE = %q, want true", res.EnvOverrides[EnvCookieSecure])
	}
	if res.EnvOverrides[EnvRequestAuthMode] != "hmac_v2" {
		t.Errorf("production REQUEST_AUTH_MODE = %q, want hmac_v2", res.EnvOverrides[EnvRequestAuthMode])
	}
	if res.EnvOverrides[EnvProviderFailurePol] != "strict" {
		t.Errorf("production PROVIDER_FAILURE_POLICY = %q, want strict", res.EnvOverrides[EnvProviderFailurePol])
	}
}

// TestValidateProductionStrictRulesAreErrors is the core strict-policy test:
// every production strict rule must be a REAL error, never a warning.
func TestValidateProductionStrictRulesAreErrors(t *testing.T) {
	p := getProfile(t, Production)
	// Weak/test config that must be rejected.
	env := map[string]string{
		EnvPasswords:           "plaintext:test1234|test1337",
		EnvHeraldAPIKey:        "test-herald-api-key",
		EnvWardenAPIKey:        "test-warden-api-key",
		EnvHeraldHmacSecret:    "test-hmac-secret",
		EnvHmacV1Enabled:       "true",
		EnvHeraldTestMode:      "true",
		EnvCookieSecure:        "false",
		EnvHeraldRedisPassword: "",
		EnvWardenRedisPassword: "",
	}
	opts := baseOpts()
	opts.ExposePorts = true // publishing ports is forbidden in production
	findings := Validate(p, env, opts)
	if !HasErrors(findings) {
		t.Fatalf("production validation must produce errors for weak config; got none")
	}
	// Assert each rule fires as an error.
	wantKeys := map[string]bool{
		EnvPasswords:           false,
		EnvHeraldAPIKey:        false,
		EnvWardenAPIKey:        false,
		EnvHeraldHmacSecret:    false,
		"exposePorts":          false,
		EnvCookieSecure:        false,
		EnvHmacV1Enabled:       false,
		EnvHeraldTestMode:      false,
		EnvHeraldRedisPassword: false,
		EnvWardenRedisPassword: false,
	}
	for _, f := range findings {
		if !f.IsError {
			t.Errorf("production finding %q must be an error, not a warning", f.Key)
		}
		if _, ok := wantKeys[f.Key]; ok {
			wantKeys[f.Key] = true
		}
	}
	for k, seen := range wantKeys {
		if !seen {
			t.Errorf("production strict rule for %q did not fire", k)
		}
	}
}

// TestValidateProductionAcceptsStrongConfig asserts real user-provided secrets
// pass production validation (no errors).
func TestValidateProductionAcceptsStrongConfig(t *testing.T) {
	p := getProfile(t, Production)
	env := map[string]string{
		EnvPasswords:           "bcrypt:$2a$10$abcdefghijklmnopqrstuv",
		EnvHeraldAPIKey:        "9f8e7d6c5b4a39281706",
		EnvWardenAPIKey:        "abcdef0123456789abcdef",
		EnvHeraldHmacSecret:    "0011223344556677889900112233445566778899",
		EnvHmacV1Enabled:       "false",
		EnvHeraldTestMode:      "false",
		EnvCookieSecure:        "true",
		EnvHeraldRedisPassword: "s3cr3t-redis-herald",
		EnvWardenRedisPassword: "s3cr3t-redis-warden",
	}
	opts := baseOpts()
	opts.ExposePorts = false
	// production containers are hardened (least privilege + read-only root);
	// the real path sets these via Apply, so mirror that for the accept case.
	opts.LeastPrivilege = true
	opts.ReadOnlyRootFS = true
	findings := Validate(p, env, opts)
	if HasErrors(findings) {
		for _, f := range findings {
			t.Logf("unexpected finding: %s", f.String())
		}
		t.Fatalf("production validation should pass with strong config")
	}
}

// TestValidateDevelopmentPlaintextIsNotError asserts development tolerates the
// current plaintext test password (warnAndError, not strict).
func TestValidateDevelopmentPlaintextIsNotError(t *testing.T) {
	p := getProfile(t, Development)
	res := Apply(p, baseOpts(), nil, NewKeyGen(bytes.NewReader(bytes.Repeat([]byte{0x02}, 512))))
	findings := Validate(p, res.EnvOverrides, res.Options)
	if HasErrors(findings) {
		for _, f := range findings {
			t.Logf("finding: %s", f.String())
		}
		t.Fatalf("development validation must not error on default dev config")
	}
}

// TestApplyByteStabilityDeterministicKeygen asserts that applying a profile
// twice with the same deterministic key source yields identical env overrides.
func TestApplyByteStabilityDeterministicKeygen(t *testing.T) {
	p := getProfile(t, Development)
	seed := bytes.Repeat([]byte{0x5A, 0xC3, 0x11, 0x77}, 256)
	res1 := Apply(p, baseOpts(), nil, NewKeyGen(bytes.NewReader(seed)))
	res2 := Apply(p, baseOpts(), nil, NewKeyGen(bytes.NewReader(seed)))
	if len(res1.EnvOverrides) != len(res2.EnvOverrides) {
		t.Fatalf("env override count differs: %d vs %d", len(res1.EnvOverrides), len(res2.EnvOverrides))
	}
	for k, v := range res1.EnvOverrides {
		if res2.EnvOverrides[k] != v {
			t.Errorf("env %q not byte-stable: %q vs %q", k, v, res2.EnvOverrides[k])
		}
	}
}

// TestApplyRespectsUserSuppliedSecrets asserts a profile never overwrites a
// user-supplied value (so an operator's real production secret is preserved).
func TestApplyRespectsUserSuppliedSecrets(t *testing.T) {
	p := getProfile(t, Development)
	user := map[string]string{
		EnvHeraldAPIKey: "operator-provided-key",
		EnvPasswords:    "bcrypt:$2a$10$xyz",
	}
	res := Apply(p, baseOpts(), user, NewKeyGen(bytes.NewReader(bytes.Repeat([]byte{0x03}, 512))))
	if _, ok := res.EnvOverrides[EnvHeraldAPIKey]; ok {
		t.Errorf("profile overrode user-supplied HERALD_API_KEY")
	}
	if _, ok := res.EnvOverrides[EnvPasswords]; ok {
		t.Errorf("profile overrode user-supplied PASSWORDS")
	}
}

// TestHmacV1ForbiddenInEveryProfile asserts enabling HMAC v1 is an error in all
// three profiles (it is always forbidden, even development).
func TestHmacV1ForbiddenInEveryProfile(t *testing.T) {
	for _, name := range []string{Development, Test, Production} {
		p := getProfile(t, name)
		env := map[string]string{EnvHmacV1Enabled: "true"}
		findings := Validate(p, env, baseOpts())
		found := false
		for _, f := range findings {
			if f.Key == EnvHmacV1Enabled && f.IsError {
				found = true
			}
		}
		if !found {
			t.Errorf("profile %q: HMAC v1 enabled must be a hard error", name)
		}
	}
}
