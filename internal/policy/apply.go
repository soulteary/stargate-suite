// Package policy: Apply turns a Profile into concrete composegen inputs
// (Options mutations + env overrides). Both CLI and Web UI call Apply so the
// two paths share one policy model rather than each reimplementing it.
package policy

import (
	"encoding/hex"
	"io"
	"strings"

	"github.com/soulteary/stargate-suite/internal/composegen"
)

// Env var keys the policy layer sets or inspects. These are the security- and
// runtime-relevant knobs surfaced by profiles; they are written into the
// generated .env as overrides (composegen keeps ${VAR:-default} in compose).
const (
	EnvPasswords            = "PASSWORDS"
	EnvHeraldAPIKey         = "HERALD_API_KEY"
	EnvWardenAPIKey         = "WARDEN_API_KEY"
	EnvHeraldHmacSecret     = "HERALD_HMAC_SECRET"
	EnvHmacSecret           = "HMAC_SECRET"
	EnvHeraldRedisPassword  = "HERALD_REDIS_PASSWORD"
	EnvWardenRedisPassword  = "WARDEN_REDIS_PASSWORD"
	EnvSessionRedisPassword = "SESSION_STORAGE_REDIS_PASSWORD"
	EnvCookieSecure         = "COOKIE_SECURE"
	EnvEnvironment          = "ENVIRONMENT"
	EnvRequestAuthMode      = "REQUEST_AUTH_MODE"
	EnvHmacV1Enabled        = "HMAC_V1_ENABLED"
	EnvHeraldTestMode       = "HERALD_TEST_MODE"
	EnvProviderFailurePol   = "PROVIDER_FAILURE_POLICY"
)

// Deterministic test values folded from the current default suite behaviour
// (canonical uses these as ${VAR:-...} defaults). They are development/test-only.
const (
	testPasswords   = "plaintext:test1234|test1337"
	testHeraldKey   = "test-herald-api-key"
	testWardenKey   = "test-warden-api-key"
	testHmacSecret  = "test-hmac-secret"
	testRedisPasswd = "test-redis-password"
)

// KeyGen produces deterministic-or-random secret material. In tests a
// deterministic io.Reader makes generation byte-stable; production/dev default
// to crypto/rand (see NewKeyGen).
type KeyGen struct {
	r io.Reader
}

// NewKeyGen returns a KeyGen reading from r. Pass a deterministic reader in
// tests for byte-stable generation; pass crypto/rand.Reader in real runs.
func NewKeyGen(r io.Reader) *KeyGen { return &KeyGen{r: r} }

// hexN returns n bytes of hex-encoded material (2*n hex chars). On short reads
// it pads deterministically so output length is stable.
func (k *KeyGen) hexN(n int) string {
	buf := make([]byte, n)
	_, _ = io.ReadFull(k.r, buf)
	return hex.EncodeToString(buf)
}

// ApplyResult is the outcome of applying a profile: the mutated Options plus
// the env overrides the profile injected. Callers merge EnvOverrides into the
// generation env (user-provided values take precedence and are not overwritten).
type ApplyResult struct {
	Options      *composegen.Options
	EnvOverrides map[string]string
}

// Apply mutates opts and returns the env overrides implied by the profile.
// Existing user values in userEnv are respected: the profile only fills gaps
// (so a user who supplied a real production key/password keeps it). keygen may
// be nil for profiles that never auto-generate (test/production).
//
// This is the single policy application shared by CLI and Web UI.
func Apply(p Profile, opts *composegen.Options, userEnv map[string]string, keygen *KeyGen) *ApplyResult {
	if opts == nil {
		opts = &composegen.Options{}
	}
	env := make(map[string]string)
	has := func(key string) bool {
		v, ok := userEnv[key]
		return ok && strings.TrimSpace(v) != ""
	}
	set := func(key, val string) {
		if !has(key) {
			env[key] = val
		}
	}

	// --- Port binding -----------------------------------------------------
	switch p.PortBinding {
	case PortReverseProxyOnly:
		// Internal services + Redis must not publish host ports. Only the
		// reverse-proxy entrypoint (Traefik) is exposed, so we stop generating
		// host port mappings for the core services and Redis.
		opts.ExposePorts = false
	case PortLoopback:
		// development/test keep host access, but bound to loopback. composegen
		// emits host:container mappings; the loopback bind is expressed via the
		// LoopbackBind option consumed by composegen when writing ports.
		opts.ExposePorts = true
		opts.LoopbackBind = true
	}

	// --- Environment marker ----------------------------------------------
	switch p.Name {
	case Production:
		set(EnvEnvironment, "production")
	case Test:
		set(EnvEnvironment, "test")
	default:
		set(EnvEnvironment, "development")
	}

	// --- Herald test mode + provider failure policy -----------------------
	switch p.HeraldTestAPI {
	case HeraldTestAPIForbidden:
		set(EnvHeraldTestMode, "false")
	case HeraldTestAPILoopback:
		set(EnvHeraldTestMode, "true")
	default:
		set(EnvHeraldTestMode, "false")
	}
	if p.Name == Production {
		// Herald fail-closed: production forces strict provider failure policy.
		set(EnvProviderFailurePol, "strict")
	}

	// --- Herald auth mode -------------------------------------------------
	switch p.HeraldAuth {
	case HeraldHmacV2OrMtls:
		set(EnvRequestAuthMode, "hmac_v2")
	case HeraldTestAPIKeyOrHmacV2:
		set(EnvRequestAuthMode, "hmac_v2")
	}
	// HMAC v1 is always forbidden; make the intent explicit in the env.
	set(EnvHmacV1Enabled, "false")

	// --- Cookie Secure ----------------------------------------------------
	switch p.CookieSecure {
	case CookieRequired:
		set(EnvCookieSecure, "true")
	case CookieOptional:
		set(EnvCookieSecure, "false")
	}

	// --- Passwords --------------------------------------------------------
	switch p.PasswordAlgorithm {
	case PasswordAllowPlaintext, PasswordAllowTest:
		set(EnvPasswords, testPasswords)
	case PasswordForbidPlaintext:
		// production: never inject a default; the user must supply a non-plaintext
		// PASSWORDS value (validation enforces this). Leave unset so the omission
		// is visible rather than silently defaulting to a weak value.
	}

	// --- Service-to-service keys -----------------------------------------
	switch p.SecretSource {
	case SecretDeterministicTest:
		set(EnvHeraldAPIKey, testHeraldKey)
		set(EnvWardenAPIKey, testWardenKey)
		set(EnvHeraldHmacSecret, testHmacSecret)
		set(EnvHmacSecret, testHmacSecret)
	case SecretAutoGenerateOrInput:
		if keygen != nil {
			set(EnvHeraldAPIKey, "dev-"+keygen.hexN(16))
			set(EnvWardenAPIKey, "dev-"+keygen.hexN(16))
			hmac := keygen.hexN(32)
			set(EnvHeraldHmacSecret, hmac)
			set(EnvHmacSecret, hmac)
		}
	case SecretUserProvidedOrFile:
		// production: keys must be user-provided or reference a secret file.
		// Never inject a value; validation rejects empty/test/placeholder keys.
	}

	// --- Redis passwords --------------------------------------------------
	switch p.RedisPassword {
	case RedisIsolated:
		set(EnvHeraldRedisPassword, testRedisPasswd)
		set(EnvWardenRedisPassword, testRedisPasswd)
	case RedisAutoGenerate:
		if keygen != nil {
			set(EnvHeraldRedisPassword, keygen.hexN(16))
			set(EnvWardenRedisPassword, keygen.hexN(16))
		}
	case RedisRequired:
		// production: password mandatory, never auto-injected; validation enforces.
	}

	// --- Network segmentation (S-03) --------------------------------------
	// All three profiles run on the segmented internal networks (edge /
	// auth-internal / warden-data / herald-data) so the flat the-gate-network
	// is never the sole isolation boundary. This is topology, not a secret, so
	// it applies uniformly regardless of environment.
	opts.NetworkSegmentation = true

	// --- Container least privilege (S-01/S-03 hardening) ------------------
	switch p.ContainerPrivileges {
	case PrivLeastPrivilegeReadonly:
		opts.LeastPrivilege = true
		opts.ReadOnlyRootFS = true
	case PrivLeastPrivilege:
		opts.LeastPrivilege = true
		opts.ReadOnlyRootFS = false
	}

	return &ApplyResult{Options: opts, EnvOverrides: env}
}
