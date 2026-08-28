package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/soulteary/stargate-suite/internal/policy"
)

// seededReaderFor mirrors the CLI's deterministic seed reader so golden tests
// exercise the exact byte-stable path used by `generate --seed`.
func seededReaderFor(seed string) *seededReader { return newSeededReader(seed) }

// genOnce runs the shared generateForProfile path for one profile + seed and
// returns the standard mode compose bytes plus the env bytes.
func genOnce(t *testing.T, profileName, seed string, userEnv map[string]string) (compose, env []byte) {
	t.Helper()
	prof, err := resolveProfile(profileName)
	if err != nil {
		t.Fatalf("resolve profile %q: %v", profileName, err)
	}
	modes := defaultModesForProfile(prof)
	gen, envMap, err := generateForProfile(profileGenInput{
		Profile:   prof,
		Modes:     modes,
		UserEnv:   userEnv,
		KeyReader: seededReaderFor(seed),
	})
	if err != nil {
		t.Fatalf("generate profile %q: %v", profileName, err)
	}
	c, ok := gen.Composes[modes[0]]
	if !ok {
		t.Fatalf("profile %q: missing compose for mode %q", profileName, modes[0])
	}
	body := envBodyFromMap(envMap)
	return c, []byte(body)
}

// sum returns a short sha256 hex of b for stable comparison in failure output.
func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// prodSecrets are non-secret placeholder values that satisfy production strict
// rules without being real credentials (never written to a committed .env by a
// test — golden output stays under build/, which is gitignored).
var prodSecrets = map[string]string{
	policy.EnvPasswords:           "bcrypt:REPLACE_WITH_REAL_BCRYPT_HASH",
	policy.EnvHeraldAPIKey:        "CHANGE_ME_HERALD_API_KEY",
	policy.EnvWardenAPIKey:        "CHANGE_ME_WARDEN_API_KEY",
	policy.EnvHeraldHmacSecret:    "CHANGE_ME_HMAC_SECRET_32BYTES_MIN",
	policy.EnvHeraldRedisPassword: "CHANGE_ME_HERALD_REDIS_PW",
	policy.EnvWardenRedisPassword: "CHANGE_ME_WARDEN_REDIS_PW",
}

// TestGoldenProfilesByteStable is the golden/byte-stability test: for each
// profile, generating twice with the same deterministic seed must yield
// byte-identical compose + env. It also asserts profile-specific invariants so
// the golden is meaningful (loopback for dev/test, no ports + secure for prod).
func TestGoldenProfilesByteStable(t *testing.T) {
	cases := []struct {
		profile string
		seed    string
		userEnv map[string]string
	}{
		{policy.Development, "pr5-golden", nil},
		{policy.Test, "pr5-golden", nil},
		{policy.Production, "pr5-golden", prodSecrets},
	}
	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			c1, e1 := genOnce(t, tc.profile, tc.seed, tc.userEnv)
			c2, e2 := genOnce(t, tc.profile, tc.seed, tc.userEnv)
			if !bytes.Equal(c1, c2) {
				t.Errorf("compose not byte-stable for %q: %s vs %s", tc.profile, sum(c1), sum(c2))
			}
			if !bytes.Equal(e1, e2) {
				t.Errorf("env not byte-stable for %q: %s vs %s", tc.profile, sum(e1), sum(e2))
			}

			env := string(e1)
			compose := string(c1)
			switch tc.profile {
			case policy.Development, policy.Test:
				if !strings.Contains(compose, "127.0.0.1:") {
					t.Errorf("%q compose should bind exposed ports to loopback (127.0.0.1)", tc.profile)
				}
				if !strings.Contains(env, "ENVIRONMENT="+tc.profile) {
					t.Errorf("%q env should set ENVIRONMENT=%s", tc.profile, tc.profile)
				}
			case policy.Production:
				if strings.Contains(env, "COOKIE_SECURE=false") || !strings.Contains(env, "COOKIE_SECURE=true") {
					t.Errorf("production env must set COOKIE_SECURE=true")
				}
				if !strings.Contains(env, "REQUEST_AUTH_MODE=hmac_v2") {
					t.Errorf("production env must set REQUEST_AUTH_MODE=hmac_v2")
				}
				if !strings.Contains(env, "HMAC_V1_ENABLED=false") {
					t.Errorf("production env must forbid HMAC v1")
				}
				// No core-service host port publishing: the only "ports:" entry
				// should be the reverse-proxy/whoami; core services carry none.
				if strings.Contains(compose, "127.0.0.1:") {
					t.Errorf("production compose must not publish loopback host ports")
				}
			}
		})
	}
}

// TestProductionGenerationRejectsWeakConfig asserts the shared validation path
// (used by CLI + Web UI) blocks production generation on weak/test config.
func TestProductionGenerationRejectsWeakConfig(t *testing.T) {
	prof, err := resolveProfile(policy.Production)
	if err != nil {
		t.Fatalf("resolve production: %v", err)
	}
	findings := validateForProfile(prof, nil, nil)
	if !policy.HasErrors(findings) {
		t.Fatalf("production must reject empty/default config with errors")
	}
}

// TestProfilesLoadFromAssets asserts the three canonical profiles load from the
// embedded asset FS (the same source the CLI and Web UI use).
func TestProfilesLoadFromAssets(t *testing.T) {
	ps, err := loadProfiles()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	for _, name := range []string{policy.Development, policy.Test, policy.Production} {
		if _, ok := ps.Get(name); !ok {
			t.Errorf("profile %q missing from config/profiles.yaml", name)
		}
	}
}
