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
	policy.EnvPasswords:             "bcrypt:REPLACE_WITH_REAL_BCRYPT_HASH",
	policy.EnvHeraldAPIKey:          "CHANGE_ME_HERALD_API_KEY",
	policy.EnvWardenAPIKey:          "CHANGE_ME_WARDEN_API_KEY",
	policy.EnvHeraldHmacSecret:      "CHANGE_ME_HMAC_SECRET_32BYTES_MIN",
	policy.EnvHeraldRedisPassword:   "CHANGE_ME_HERALD_REDIS_PW",
	policy.EnvWardenRedisPassword:   "CHANGE_ME_WARDEN_REDIS_PW",
	policy.EnvHeraldPIIPepper:       "CHANGE_ME_HERALD_PII_PEPPER_32BYTES",
	policy.EnvHeraldIdempotencySecr: "CHANGE_ME_HERALD_IDEMPOTENCY_32BYTES",
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

			// PR 6 invariants shared by all profiles: network segmentation
			// (flat the-gate-network replaced by segmented internal networks)
			// and Redis password closure (server --requirepass + authenticated
			// healthcheck, no unauthenticated `redis-cli ping`).
			for _, seg := range []string{"auth-internal", "warden-data", "herald-data"} {
				if !strings.Contains(compose, seg) {
					t.Errorf("%q compose should declare segmented network %q", tc.profile, seg)
				}
			}
			if strings.Contains(compose, "the-gate-network") {
				t.Errorf("%q compose should not keep the flat the-gate-network after segmentation", tc.profile)
			}
			if !strings.Contains(compose, "--requirepass") {
				t.Errorf("%q compose Redis must set --requirepass (S-01)", tc.profile)
			}
			if !strings.Contains(compose, `HERALD_REDIS_PASSWORD}" ping`) ||
				!strings.Contains(compose, `WARDEN_REDIS_PASSWORD}" ping`) {
				t.Errorf("%q compose Redis healthcheck must authenticate with the password (S-01)", tc.profile)
			}
			// Least privilege applies to every profile: cap_drop ALL and
			// no-new-privileges. Redis must NOT publish a host port (S-03).
			if !strings.Contains(compose, "cap_drop") || !strings.Contains(compose, "no-new-privileges:true") {
				t.Errorf("%q compose must apply least-privilege (cap_drop ALL + no-new-privileges)", tc.profile)
			}
			if strings.Contains(compose, "6379:6379") {
				t.Errorf("%q compose must not publish Redis host port 6379 (S-03)", tc.profile)
			}

			// PR 8 invariants shared by all profiles: core images pinned to the
			// v1 contract line, Stargate on 8080 (not 80), and component-specific
			// health probes that are available in each runtime image.
			for _, img := range []string{
				"ghcr.io/soulteary/stargate:1.0.0",
				"ghcr.io/soulteary/warden:1.0.0",
				"ghcr.io/soulteary/herald:1.1.0",
			} {
				if !strings.Contains(compose, img) {
					t.Errorf("%q compose should pin core image %q (PR8)", tc.profile, img)
				}
			}
			if !strings.Contains(compose, "http://stargate:8080/_auth") {
				t.Errorf("%q compose forwardAuth must target Stargate on :8080 (PR8 port 80→8080)", tc.profile)
			}
			if strings.Contains(compose, "forwardauth.address=http://stargate/_auth") {
				t.Errorf("%q compose must not keep the legacy port-80 forwardAuth address (PR8)", tc.profile)
			}
			if !strings.Contains(compose, "8080/healthz") {
				t.Errorf("%q compose Stargate healthcheck must probe :8080/healthz (PR8)", tc.profile)
			}
			if !strings.Contains(compose, `["CMD", "/bin/herald", "-healthcheck"]`) {
				t.Errorf("%q compose Herald healthcheck must use the built-in checker (PR8)", tc.profile)
			}
			if !strings.Contains(compose, "8081/healthcheck") {
				t.Errorf("%q compose Warden healthcheck must probe :8081/healthcheck (PR8)", tc.profile)
			}
			// HMAC v1 must be disabled for every profile (never a silent
			// downgrade to the non-replay-resistant v1 canonical) — PR8 posture.
			if !strings.Contains(env, "HMAC_V1_ENABLED=false") {
				t.Errorf("%q env must forbid HMAC v1 (PR8)", tc.profile)
			}

			switch tc.profile {
			case policy.Development, policy.Test:
				if !strings.Contains(compose, "127.0.0.1:") {
					t.Errorf("%q compose should bind exposed ports to loopback (127.0.0.1)", tc.profile)
				}
				if !strings.Contains(env, "ENVIRONMENT="+tc.profile) {
					t.Errorf("%q env should set ENVIRONMENT=%s", tc.profile, tc.profile)
				}
				if tc.profile == policy.Test && !strings.Contains(env, "REQUEST_AUTH_MODE=hmac_v2") {
					t.Errorf("test env must set REQUEST_AUTH_MODE=hmac_v2 so E2E signs with HMAC v2 (PR8)")
				}
				// development/test use leastPrivilege WITHOUT a read-only root
				// filesystem (readonly is production-only).
				if strings.Contains(compose, "read_only: true") {
					t.Errorf("%q compose should not force read-only root filesystem (production-only)", tc.profile)
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
				// production hardens further with a read-only root filesystem
				// plus a writable /tmp tmpfs.
				if !strings.Contains(compose, "read_only: true") {
					t.Errorf("production compose must set read-only root filesystem")
				}
				if !strings.Contains(compose, "/tmp") {
					t.Errorf("production read-only services need a writable /tmp tmpfs")
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
