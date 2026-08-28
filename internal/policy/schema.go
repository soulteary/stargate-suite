// Package policy: validation primitives shared by the four-layer validator
// (see validate.go). These Go definitions are the runtime source of truth for
// field shapes, secret strength, and stable finding codes; no declarative
// shadow schema is maintained. Layer 1 checks field types; layer 2 checks
// single-field safety; higher layers live in validate.go.
package policy

import (
	"net"
	"regexp"
	"strings"
	"time"
)

// Structured error/warning codes. Stable identifiers for scripting; grouped by
// the service/domain they concern.
const (
	CodePasswordsRequired       = "STARGATE_PASSWORDS_REQUIRED"
	CodePasswordsPlaintext      = "STARGATE_PASSWORDS_PLAINTEXT_FORBIDDEN"
	CodePasswordsHashRequired   = "STARGATE_PASSWORDS_HASH_REQUIRED"
	CodeCookieSecureRequired    = "STARGATE_COOKIE_SECURE_REQUIRED"
	CodeSessionExchangeSecret   = "STARGATE_SESSION_EXCHANGE_SECRET_REQUIRED"
	CodeStepUpPathsRequired     = "STARGATE_STEP_UP_PATHS_REQUIRED"
	CodeStepUpProxiesRequired   = "STARGATE_STEP_UP_TRUSTED_PROXIES_REQUIRED"
	CodeCallbackHostsInvalid    = "STARGATE_CALLBACK_ALLOWED_HOSTS_INVALID"
	CodeTrustedProxiesInvalid   = "STARGATE_TRUSTED_PROXIES_INVALID_CIDR"
	CodeHeraldAPIKeyWeak        = "HERALD_API_KEY_WEAK"
	CodeWardenAPIKeyWeak        = "WARDEN_API_KEY_WEAK"
	CodeHmacSecretWeak          = "HMAC_SECRET_WEAK"
	CodeHmacV1Forbidden         = "HMAC_V1_FORBIDDEN"
	CodeHmacDriftInvalid        = "HERALD_HMAC_MAX_DRIFT_INVALID"
	CodeIdempotencySecretWeak   = "HERALD_IDEMPOTENCY_SECRET_WEAK"
	CodePIIPepperWeak           = "HERALD_PII_PEPPER_WEAK"
	CodeRedisPasswordRequired   = "REDIS_PASSWORD_REQUIRED"
	CodeExposePortsForbidden    = "PORTS_HOST_PUBLISH_FORBIDDEN"
	CodeHeraldTestModeForbidden = "HERALD_TEST_MODE_FORBIDDEN"
	CodeContainerPrivileges     = "CONTAINER_LEAST_PRIVILEGE_REQUIRED"
	CodeContainerReadonly       = "CONTAINER_READONLY_ROOTFS_REQUIRED"
	CodeAuthModeMismatch        = "CROSS_SERVICE_AUTH_MODE_MISMATCH"
	CodeAuthModeAmbiguous       = "CROSS_SERVICE_AUTH_MODE_AMBIGUOUS"
	CodeTLSPairIncomplete       = "TLS_CERT_KEY_PAIR_INCOMPLETE"
	CodeUnknownEnvVar           = "UNKNOWN_ENV_VAR"
	CodeDurationInvalid         = "FIELD_DURATION_INVALID"
	CodePortInvalid             = "FIELD_PORT_INVALID"
	CodeURLInvalid              = "FIELD_URL_INVALID"
	CodeBoolInvalid             = "FIELD_BOOL_INVALID"
)

// minSecretLen is the minimum length for user-provided secrets (session
// exchange secret, PII pepper, idempotency secret, HMAC secret) in strict
// profiles. 32 chars ~ 128+ bits of entropy for hex/base64 material.
const minSecretLen = 32

const (
	minAPIKeyLen        = 16
	minRedisPasswordLen = 16
)

// isValidDuration reports whether s parses as a Go duration (e.g. 5m, 60s, 1h).
func isValidDuration(s string) bool {
	_, err := time.ParseDuration(strings.TrimSpace(s))
	return err == nil
}

// isValidBool reports whether s is a recognized boolean literal.
func isValidBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "false", "1", "0", "yes", "no", "on", "off":
		return true
	}
	return false
}

// isValidCIDRList reports whether s is a comma-separated list of CIDRs or bare
// IPs (both accepted for a trusted-proxy list). Empty entries are ignored.
func isValidCIDRList(s string) bool {
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(p); err == nil {
			continue
		}
		if net.ParseIP(p) != nil {
			continue
		}
		return false
	}
	return true
}

// hostPattern matches a hostname or hostname:port or wildcard *.example.com
// used in CALLBACK_ALLOWED_HOSTS. It is intentionally permissive but rejects
// obvious garbage (spaces, schemes).
var hostPattern = regexp.MustCompile(`^\*?[a-zA-Z0-9.-]+(:[0-9]{1,5})?$`)

// isValidHostList reports whether s is a comma-separated list of hostnames
// (optionally wildcarded / with port). Empty entries are ignored.
func isValidHostList(s string) bool {
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if strings.Contains(p, "://") || strings.ContainsAny(p, " \t") {
			return false
		}
		if !hostPattern.MatchString(p) {
			return false
		}
	}
	return true
}

// strongSecret reports whether v is a non-weak secret of at least minSecretLen
// characters. Used for session-exchange / pepper / idempotency / HMAC secrets
// in strict profiles.
func strongSecret(v string) bool {
	return strongCredential(v, minSecretLen)
}

func strongCredential(v string, minLen int) bool {
	v = strings.TrimSpace(v)
	return len(v) >= minLen && !looksWeakOrTest(v)
}
