// Package main: profile-aware generation and validation shared by the CLI
// (`generate --profile`, `validate --profile --strict`) and the Web UI (profile
// selection first step). Both paths funnel through generateForProfile /
// validateForProfile so the SAME policy + composegen model is used — neither
// reimplements profile semantics. See internal/policy and config/profiles.yaml.
package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/policy"
)

const profilesPath = policy.ProfilesPath

// loadProfiles reads config/profiles.yaml from the active asset FS.
func loadProfiles() (*policy.Profiles, error) {
	return policy.LoadProfiles(assetFS(), profilesPath)
}

// resolveProfile loads profiles and returns the named one, defaulting to
// development when name is empty. Unknown names are a hard error.
func resolveProfile(name string) (policy.Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = policy.Development
	}
	if !policy.KnownProfile(name) {
		return policy.Profile{}, fmt.Errorf("unknown profile %q (want development|test|production)", name)
	}
	ps, err := loadProfiles()
	if err != nil {
		return policy.Profile{}, err
	}
	p, ok := ps.Get(name)
	if !ok {
		return policy.Profile{}, fmt.Errorf("profile %q not defined in %s", name, profilesPath)
	}
	return p, nil
}

// profileGenInput bundles the inputs to generateForProfile so CLI and Web UI
// share one call shape.
type profileGenInput struct {
	Profile   policy.Profile
	Modes     []string
	BaseOpts  *composegen.Options // starting options (nil => defaults); policy mutates a copy
	UserEnv   map[string]string   // user-supplied env overrides (respected over profile defaults)
	KeyReader io.Reader           // deterministic in tests; crypto/rand in real runs (nil => crypto/rand)
}

// effectiveEnv merges profile-injected env over user env: user values win.
func effectiveEnv(userEnv, profileEnv map[string]string) map[string]string {
	out := make(map[string]string, len(userEnv)+len(profileEnv))
	for k, v := range profileEnv {
		out[k] = v
	}
	for k, v := range userEnv {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}

// envBodyFromMap renders a deterministic KEY=VALUE body (sorted) for the
// generation env override. composegen keeps ${VAR:-default} in compose; these
// overrides land in the generated .env.
func envBodyFromMap(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(composegen.EncodeEnvValue(env[k]))
		b.WriteString("\n")
	}
	return b.String()
}

// generateForProfile is the single shared generation path. It applies the
// profile policy to the options and env, then calls composegen.Generate. The
// returned env body is the profile+user env; validation is the caller's job
// (CLI errors out on strict violations before writing).
func generateForProfile(in profileGenInput) (*composegen.Generated, map[string]string, error) {
	full, err := composegen.LoadComposeFS(assetFS(), canonicalCompose)
	if err != nil {
		return nil, nil, fmt.Errorf("load canonical compose: %w", err)
	}

	base := in.BaseOpts
	if base == nil {
		base = defaultProfileOptions()
	}
	kr := in.KeyReader
	if kr == nil {
		kr = rand.Reader
	}
	res := policy.Apply(in.Profile, base, in.UserEnv, policy.NewKeyGen(kr))

	// Feed manifest container ports (single source of truth) into composegen.
	applyManifestToComposegen()

	env := effectiveEnv(in.UserEnv, res.EnvOverrides)
	envMeta, err := composegen.LoadEnvMetaFS(assetFS(), "config/env-meta.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("load env-meta: %w", err)
	}
	gen, err := composegen.Generate(full, in.Modes, envBodyFromMap(env), res.Options, envMeta)
	if err != nil {
		return nil, nil, err
	}
	return gen, env, nil
}

// validateForProfile runs the profile policy validation against the effective
// (profile + user) config. Shared by CLI and Web UI.
func validateForProfile(p policy.Profile, baseOpts *composegen.Options, userEnv map[string]string) []policy.Finding {
	base := baseOpts
	if base == nil {
		base = defaultProfileOptions()
	}
	// Apply with a nil keygen: validation must judge the config as provided,
	// not values a generator would auto-fill (production never auto-fills keys).
	res := policy.Apply(p, base, userEnv, nil)
	env := effectiveEnv(userEnv, res.EnvOverrides)
	return policy.Validate(p, env, res.Options)
}

// defaultProfileOptions returns the baseline generation options for profile
// generation (full stack, healthchecks on, Traefik on, named volumes). Profile
// policy then adjusts port exposure/binding etc.
func defaultProfileOptions() *composegen.Options {
	return &composegen.Options{
		HealthCheck:         true,
		TraefikNetwork:      true,
		TraefikNetworkName:  "traefik",
		ExposePorts:         true,
		UseNamedVolume:      true,
		ContainerNamePrefix: "the-gate-",
		HeraldRedisDataPath: "./data/herald-redis",
		WardenRedisDataPath: "./data/warden-redis",
		EnvOverrides:        map[string]string{},
	}
}

// defaultModesForProfile returns the standard compose mode(s) generated for a
// profile when the caller does not specify --modes. The full Traefik stack is
// the canonical standard scenario for every profile.
func defaultModesForProfile(_ policy.Profile) []string {
	return []string{"traefik"}
}
