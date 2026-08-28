// Package policy models deployment profiles (development / test / production)
// as SECURITY & RUNTIME POLICY, not merely prefilled forms. It is the single
// shared model consumed by both the CLI (`generate --profile`, `validate
// --profile --strict`) and the Web UI (profile selection first step): both call
// Apply to derive profile-aware composegen inputs and Validate to enforce the
// profile's rules. See config/profiles.yaml and docs/deployment.md.
//
// Design notes:
//   - The current default suite behaviour (PASSWORDS=plaintext:..., API_KEY=test-*,
//     exposePorts on by default) is folded into development/test semantics.
//   - production is initially experimental, but every strict rule is a REAL
//     error (never a warning): reject plaintext passwords, reject test/placeholder
//     keys, internal services must not publish host ports, Cookie Secure must be
//     on, HMAC v1 forbidden. Generation must not let production bypass errors.
//   - Key generation is fed by an injectable io.Reader so generating twice is
//     byte-stable in tests (deterministic source) while production/dev use
//     crypto/rand by default.
package policy

import (
	"fmt"
	"io/fs"
	"sort"

	"gopkg.in/yaml.v3"
)

// ProfilesPath is the repository-relative, slash-separated path to the profile
// definitions within the embedded asset FS.
const ProfilesPath = "config/profiles.yaml"

// Profile names.
const (
	Development = "development"
	Test        = "test"
	Production  = "production"
)

// Port binding strategies.
const (
	PortLoopback         = "loopback"
	PortReverseProxyOnly = "reverseProxyOnly"
)

// Secret source strategies.
const (
	SecretAutoGenerateOrInput = "autoGenerateOrInput"
	SecretDeterministicTest   = "deterministicTest"
	SecretUserProvidedOrFile  = "userProvidedOrFile"
)

// Password algorithm strategies.
const (
	PasswordAllowPlaintext  = "allowPlaintext"
	PasswordAllowTest       = "allowTest"
	PasswordForbidPlaintext = "forbidPlaintext"
)

// Herald auth strategies.
const (
	HeraldAPIKeyOptional     = "apiKeyOptional"
	HeraldTestAPIKeyOrHmacV2 = "testApiKeyOrHmacV2"
	HeraldHmacV2             = "hmacV2"
	// HeraldHmacV2OrMtls is retained for custom profile compatibility. The
	// built-in production profile uses HMAC v2 until certificate mounts and
	// lifecycle management are part of generated deployments.
	HeraldHmacV2OrMtls = "hmacV2OrMtls"
)

// Herald test API strategies.
const (
	HeraldTestAPIDisabled  = "disabled"
	HeraldTestAPILoopback  = "loopbackListener"
	HeraldTestAPIForbidden = "forbidden"
)

// Redis password strategies.
const (
	RedisAutoGenerate = "autoGenerate"
	RedisIsolated     = "isolated"
	RedisRequired     = "required"
)

// Cookie Secure strategies.
const (
	CookieOptional = "optional"
	CookieRequired = "required"
)

// Container privilege strategies.
const (
	PrivLeastPrivilege         = "leastPrivilege"
	PrivLeastPrivilegeReadonly = "leastPrivilegeReadonly"
)

// Validation modes.
const (
	ValidationWarnAndError = "warnAndError"
	ValidationStrict       = "strict"
)

// Profile is one deployment profile's policy, mirroring config/profiles.yaml.
type Profile struct {
	Name                string `yaml:"-"`
	Description         string `yaml:"description"`
	Experimental        bool   `yaml:"experimental"`
	PortBinding         string `yaml:"portBinding"`
	SecretSource        string `yaml:"secretSource"`
	PasswordAlgorithm   string `yaml:"passwordAlgorithm"`
	HeraldAuth          string `yaml:"heraldAuth"`
	HeraldTestAPI       string `yaml:"heraldTestApi"`
	RedisPassword       string `yaml:"redisPassword"`
	CookieSecure        string `yaml:"cookieSecure"`
	HmacV1              string `yaml:"hmacV1"`
	ContainerPrivileges string `yaml:"containerPrivileges"`
	ValidationMode      string `yaml:"validationMode"`
}

// Strict reports whether the profile runs validation in strict mode (production
// / test). In strict mode policy violations are errors, never warnings.
func (p Profile) Strict() bool { return p.ValidationMode == ValidationStrict }

// Profiles is the parsed config/profiles.yaml.
type Profiles struct {
	SchemaVersion int                `yaml:"schemaVersion"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

// Names returns the profile names in a stable order.
func (ps *Profiles) Names() []string {
	if ps == nil {
		return nil
	}
	out := make([]string, 0, len(ps.Profiles))
	for name := range ps.Profiles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Get returns the named profile (with Name populated) and whether it exists.
func (ps *Profiles) Get(name string) (Profile, bool) {
	if ps == nil {
		return Profile{}, false
	}
	p, ok := ps.Profiles[name]
	if !ok {
		return Profile{}, false
	}
	p.Name = name
	return p, true
}

// LoadProfiles reads and parses config/profiles.yaml from an fs.FS (typically
// the embedded assets FS). Callers pass ProfilesPath.
func LoadProfiles(fsys fs.FS, path string) (*Profiles, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("read profiles: %w", err)
	}
	return ParseProfiles(data)
}

// ParseProfiles parses profiles YAML bytes.
func ParseProfiles(data []byte) (*Profiles, error) {
	var ps Profiles
	if err := yaml.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	if ps.Profiles == nil {
		ps.Profiles = make(map[string]Profile)
	}
	return &ps, nil
}

// KnownProfile reports whether name is one of the three canonical profiles.
func KnownProfile(name string) bool {
	switch name {
	case Development, Test, Production:
		return true
	default:
		return false
	}
}
