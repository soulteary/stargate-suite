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
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

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

const hmacV1Forbidden = "forbidden"

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
func (p Profile) Strict() bool {
	return p.Name == Test || p.Name == Production || p.ValidationMode == ValidationStrict
}

// enforceCanonicalSecurity makes the test and production security boundary
// independent of mutable YAML strategy fields. ParseProfiles rejects weakened
// canonical profiles, while this second line of defence also protects callers
// that construct a Profile directly.
func (p Profile) enforceCanonicalSecurity() Profile {
	switch p.Name {
	case Test:
		p.ValidationMode = ValidationStrict
	case Production:
		p.PortBinding = PortReverseProxyOnly
		p.SecretSource = SecretUserProvidedOrFile
		p.PasswordAlgorithm = PasswordForbidPlaintext
		p.HeraldAuth = HeraldHmacV2
		p.HeraldTestAPI = HeraldTestAPIForbidden
		p.RedisPassword = RedisRequired
		p.CookieSecure = CookieRequired
		p.HmacV1 = hmacV1Forbidden
		p.ContainerPrivileges = PrivLeastPrivilegeReadonly
		p.ValidationMode = ValidationStrict
	}
	return p
}

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
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&ps); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse profiles: multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	if err := validateProfiles(&ps); err != nil {
		return nil, fmt.Errorf("validate profiles: %w", err)
	}
	return &ps, nil
}

func validateProfiles(ps *Profiles) error {
	if ps.SchemaVersion != 1 {
		return fmt.Errorf("schemaVersion must be 1 (got %d)", ps.SchemaVersion)
	}
	if len(ps.Profiles) != 3 {
		return fmt.Errorf("profiles must define exactly development, test, and production")
	}
	for _, name := range []string{Development, Test, Production} {
		p, ok := ps.Profiles[name]
		if !ok {
			return fmt.Errorf("profile %q is required", name)
		}
		if err := validateProfile(name, p); err != nil {
			return fmt.Errorf("profile %q: %w", name, err)
		}
	}
	return nil
}

func validateProfile(name string, p Profile) error {
	if strings.TrimSpace(p.Description) == "" {
		return fmt.Errorf("description is required")
	}
	checks := []struct {
		field string
		value string
		valid []string
	}{
		{"portBinding", p.PortBinding, []string{PortLoopback, PortReverseProxyOnly}},
		{"secretSource", p.SecretSource, []string{SecretAutoGenerateOrInput, SecretDeterministicTest, SecretUserProvidedOrFile}},
		{"passwordAlgorithm", p.PasswordAlgorithm, []string{PasswordAllowPlaintext, PasswordAllowTest, PasswordForbidPlaintext}},
		{"heraldAuth", p.HeraldAuth, []string{HeraldAPIKeyOptional, HeraldTestAPIKeyOrHmacV2, HeraldHmacV2, HeraldHmacV2OrMtls}},
		{"heraldTestApi", p.HeraldTestAPI, []string{HeraldTestAPIDisabled, HeraldTestAPILoopback, HeraldTestAPIForbidden}},
		{"redisPassword", p.RedisPassword, []string{RedisAutoGenerate, RedisIsolated, RedisRequired}},
		{"cookieSecure", p.CookieSecure, []string{CookieOptional, CookieRequired}},
		{"hmacV1", p.HmacV1, []string{hmacV1Forbidden}},
		{"containerPrivileges", p.ContainerPrivileges, []string{PrivLeastPrivilege, PrivLeastPrivilegeReadonly}},
		{"validationMode", p.ValidationMode, []string{ValidationWarnAndError, ValidationStrict}},
	}
	for _, check := range checks {
		if !contains(check.valid, check.value) {
			return fmt.Errorf("%s has invalid value %q", check.field, check.value)
		}
	}

	canonical := map[string]Profile{
		Development: {
			PortBinding: PortLoopback, SecretSource: SecretAutoGenerateOrInput,
			PasswordAlgorithm: PasswordAllowPlaintext, HeraldAuth: HeraldAPIKeyOptional,
			HeraldTestAPI: HeraldTestAPIDisabled, RedisPassword: RedisAutoGenerate,
			CookieSecure: CookieOptional, HmacV1: hmacV1Forbidden,
			ContainerPrivileges: PrivLeastPrivilege, ValidationMode: ValidationWarnAndError,
		},
		Test: {
			PortBinding: PortLoopback, SecretSource: SecretDeterministicTest,
			PasswordAlgorithm: PasswordAllowTest, HeraldAuth: HeraldTestAPIKeyOrHmacV2,
			HeraldTestAPI: HeraldTestAPILoopback, RedisPassword: RedisIsolated,
			CookieSecure: CookieOptional, HmacV1: hmacV1Forbidden,
			ContainerPrivileges: PrivLeastPrivilege, ValidationMode: ValidationStrict,
		},
		Production: {
			PortBinding: PortReverseProxyOnly, SecretSource: SecretUserProvidedOrFile,
			PasswordAlgorithm: PasswordForbidPlaintext, HeraldAuth: HeraldHmacV2,
			HeraldTestAPI: HeraldTestAPIForbidden, RedisPassword: RedisRequired,
			CookieSecure: CookieRequired, HmacV1: hmacV1Forbidden,
			ContainerPrivileges: PrivLeastPrivilegeReadonly, ValidationMode: ValidationStrict,
		},
	}[name]
	for _, pair := range []struct {
		field string
		got   string
		want  string
	}{
		{"portBinding", p.PortBinding, canonical.PortBinding},
		{"secretSource", p.SecretSource, canonical.SecretSource},
		{"passwordAlgorithm", p.PasswordAlgorithm, canonical.PasswordAlgorithm},
		{"heraldAuth", p.HeraldAuth, canonical.HeraldAuth},
		{"heraldTestApi", p.HeraldTestAPI, canonical.HeraldTestAPI},
		{"redisPassword", p.RedisPassword, canonical.RedisPassword},
		{"cookieSecure", p.CookieSecure, canonical.CookieSecure},
		{"hmacV1", p.HmacV1, canonical.HmacV1},
		{"containerPrivileges", p.ContainerPrivileges, canonical.ContainerPrivileges},
		{"validationMode", p.ValidationMode, canonical.ValidationMode},
	} {
		if pair.got != pair.want {
			return fmt.Errorf("%s must be %q (got %q)", pair.field, pair.want, pair.got)
		}
	}
	return nil
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
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
