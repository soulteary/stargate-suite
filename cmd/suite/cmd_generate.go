// Package main: `generate` subcommand. Produces profile-aware compose + .env
// on disk. Shares the policy + composegen model with the Web UI via
// generateForProfile / validateForProfile (see profile.go). Production strict
// policy violations abort generation (they are real errors, not warnings).
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/policy"
)

// stringSliceFlag collects repeated --set KEY=VALUE flags.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	i := strings.IndexByte(v, '=')
	if i <= 0 {
		return fmt.Errorf("expected KEY=VALUE, got %q", v)
	}
	if key := strings.TrimSpace(v[:i]); !composegen.ValidEnvKey(key) {
		return fmt.Errorf("invalid environment variable name %q", key)
	}
	*s = append(*s, v)
	return nil
}

// profileSecretEnvKeys are the security-relevant env keys the generate command
// reads from the process environment (and --set) as user-supplied overrides.
// This lets an operator generate a production stack with real secrets without
// ever baking them into config, while a bare invocation correctly fails the
// production strict rules.
var profileSecretEnvKeys = []string{
	policy.EnvPasswords,
	policy.EnvHeraldAPIKey,
	policy.EnvWardenAPIKey,
	policy.EnvHeraldHmacSecret,
	policy.EnvHmacSecret,
	policy.EnvHeraldPIIPepper,
	policy.EnvHeraldIdempotencySecr,
	policy.EnvHeraldRedisPassword,
	policy.EnvWardenRedisPassword,
	policy.EnvSessionRedisPassword,
	policy.EnvCookieSecure,
	policy.EnvRequestAuthMode,
	policy.EnvHeraldTLSCACertFile,
	policy.EnvHeraldTLSClientCert,
	policy.EnvHeraldTLSClientKey,
	policy.EnvHeraldTLSServerName,
	policy.EnvWardenTLSCACertFile,
	policy.EnvWardenTLSClientCertFile,
	policy.EnvWardenTLSClientKeyFile,
	policy.EnvWardenTLSServerName,
}

// collectUserEnv builds the user-supplied env overrides from the process
// environment (only the profile-relevant keys) and explicit --set entries.
// --set wins over the ambient environment.
func collectUserEnv(sets []string) map[string]string {
	env := make(map[string]string)
	for _, k := range profileSecretEnvKeys {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	for _, kv := range sets {
		if i := strings.IndexByte(kv, '='); i > 0 {
			env[strings.TrimSpace(kv[:i])] = kv[i+1:]
		}
	}
	return env
}

func cmdGenerate() error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	profileName := fs.String("profile", policy.Development, "deployment profile: development|test|production")
	output := fs.String("output", "", "output directory for generated compose + .env (required)")
	modesCSV := fs.String("modes", "", "comma-separated compose modes (default: profile standard scenario)")
	force := fs.Bool("force", false, "generate even if a non-production profile has policy violations (production is never bypassable)")
	jsonOut := fs.Bool("json", false, "emit a structured result (profile, modes, outputs, findings) as JSON")
	canonical := fs.Bool("canonical", false, "generate raw canonical compose(s) with NO profile policy applied, one subdir per mode (reproduces `make gen`; --modes selects modes, default: all build modes)")
	lockPath := fs.String("lock", "", "path to a release components.lock.yaml; pin generated images to immutable digests")
	seed := fs.String("seed", "", "deterministic seed for auto-generated dev/test keys (byte-stable output; leave empty for crypto/rand). Never use a real seed for real deployments.")
	var sets stringSliceFlag
	fs.Var(&sets, "set", "override an env value as KEY=VALUE (repeatable); also read from process env for known secret keys")
	if err := fs.Parse(cmdArgs); err != nil {
		return err
	}

	if strings.TrimSpace(*output) == "" {
		return fmt.Errorf("generate: --output directory is required")
	}
	lockedEnv := map[string]string(nil)
	if strings.TrimSpace(*lockPath) != "" {
		var err error
		lockedEnv, err = loadLockedImageEnv(*lockPath)
		if err != nil {
			return fmt.Errorf("generate: lock: %w", err)
		}
	}

	// Canonical path: reproduce the historical `make gen` output (raw canonical
	// compose, options:null, no profile policy) entirely in-process — no Web
	// server, no jq. This shares composegen.Generate with the Web /api/generate
	// (options:null) path, so CLI and Web produce identical bytes.
	if *canonical {
		return generateCanonicalWithEnv(*output, *modesCSV, *jsonOut, lockedEnv)
	}

	prof, err := resolveProfile(*profileName)
	if err != nil {
		return err
	}

	userEnv := collectUserEnv(sets)
	for key, value := range lockedEnv {
		userEnv[key] = value
	}

	modes := defaultModesForProfile(prof)
	if strings.TrimSpace(*modesCSV) != "" {
		modes = splitCSV(*modesCSV)
	}

	// Enforce profile policy BEFORE writing. In strict profiles (test /
	// production) any error aborts. production must never be bypassable.
	findings := validateForProfile(prof, nil, userEnv)
	if policy.HasErrors(findings) {
		if !*jsonOut {
			printFindings(findings)
		}
		if prof.Name == policy.Production || !*force {
			if *jsonOut {
				emitGenerateJSON(generateJSONResult{
					Profile:  prof.Name,
					Modes:    modes,
					Output:   filepath.Clean(*output),
					OK:       false,
					Findings: findingsToJSON(findings),
					Error:    fmt.Sprintf("profile %q has policy violations; refusing to generate", prof.Name),
				})
			}
			return fmt.Errorf("generate: profile %q has policy violations; refusing to generate (supply real secrets via env/--set; production strict rules are hard errors)", prof.Name)
		}
	} else if len(findings) > 0 && !*jsonOut {
		printFindings(findings)
	}

	gen, env, err := generateForProfile(profileGenInput{
		Profile:   prof,
		Modes:     modes,
		UserEnv:   userEnv,
		KeyReader: keyReaderForSeed(*seed),
	})
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	_ = env

	outDir := filepath.Clean(*output)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("generate: mkdir %s: %w", outDir, err)
	}
	var written []string
	// Single mode → write directly under outDir; multiple → subdir per mode.
	if len(modes) == 1 {
		if err := writeGenerated(outDir, gen.Composes[modes[0]], gen.Env); err != nil {
			return err
		}
		written = append(written, filepath.Join(outDir, "docker-compose.yml"), filepath.Join(outDir, ".env"))
		if !*jsonOut {
			fmt.Printf("Generated profile %q (%s) → %s\n", prof.Name, modes[0], outDir)
		}
	} else {
		for _, mode := range modes {
			sub := filepath.Join(outDir, mode)
			if err := os.MkdirAll(sub, 0o755); err != nil {
				return fmt.Errorf("generate: mkdir %s: %w", sub, err)
			}
			if err := writeGenerated(sub, gen.Composes[mode], gen.Env); err != nil {
				return err
			}
			written = append(written, filepath.Join(sub, "docker-compose.yml"), filepath.Join(sub, ".env"))
		}
		if !*jsonOut {
			fmt.Printf("Generated profile %q (%s) → %s/<mode>/\n", prof.Name, strings.Join(modes, ","), outDir)
		}
	}
	if prof.Experimental && !*jsonOut {
		fmt.Printf("note: profile %q is experimental; review generated config before real deployment\n", prof.Name)
	}
	if *jsonOut {
		emitGenerateJSON(generateJSONResult{
			Profile:      prof.Name,
			Modes:        modes,
			Output:       outDir,
			OK:           true,
			Experimental: prof.Experimental,
			Written:      written,
			Findings:     findingsToJSON(findings),
		})
	}
	return nil
}

// generateJSONResult is the stable --json shape for `generate`. It never
// includes secret values — only the profile, requested modes, the paths
// written, and any advisory findings (by stable code).
type generateJSONResult struct {
	Profile      string            `json:"profile"`
	Modes        []string          `json:"modes"`
	Output       string            `json:"output"`
	OK           bool              `json:"ok"`
	Experimental bool              `json:"experimental,omitempty"`
	Written      []string          `json:"written,omitempty"`
	Findings     []jsonFindingItem `json:"findings,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// jsonFindingItem mirrors validate --json findings for a consistent contract.
type jsonFindingItem struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Profile string `json:"profile"`
	Message string `json:"message"`
	IsError bool   `json:"is_error"`
}

func findingsToJSON(findings []policy.Finding) []jsonFindingItem {
	out := make([]jsonFindingItem, 0, len(findings))
	for _, f := range findings {
		out = append(out, jsonFindingItem{Code: f.Code, Field: f.Field, Profile: f.Profile, Message: f.Message, IsError: f.IsError})
	}
	return out
}

func emitGenerateJSON(res generateJSONResult) {
	b, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(b))
}

// canonicalBuildModes are the compose modes `make gen` produced via the Web API
// path. Kept here so the CLI reproduces the same build/* layout in one process.
var canonicalBuildModes = []string{"image", "build", "traefik", "traefik-herald", "traefik-warden", "traefik-stargate"}

// generateCanonical writes one subdir per mode under outDir, each containing a
// docker-compose.yml + .env, using the raw canonical compose with NO profile
// policy applied (options:null semantics). It calls the same
// composegen.Generate path as the Web /api/generate handler, so output is
// byte-identical without starting a Web server.
func generateCanonical(output, modesCSV string, jsonOut bool) error {
	return generateCanonicalWithEnv(output, modesCSV, jsonOut, nil)
}

func generateCanonicalWithEnv(output, modesCSV string, jsonOut bool, lockedEnv map[string]string) error {
	modes := canonicalBuildModes
	if strings.TrimSpace(modesCSV) != "" {
		modes = splitCSV(modesCSV)
	}
	full, err := composegen.LoadComposeFS(assetFS(), canonicalCompose)
	if err != nil {
		return fmt.Errorf("generate: load canonical compose: %w", err)
	}
	// Feed manifest container ports (single source of truth) into composegen,
	// matching the Web UI startup path.
	applyManifestToComposegen()
	envMeta, err := composegen.LoadEnvMetaFS(assetFS(), "config/env-meta.yaml")
	if err != nil {
		return fmt.Errorf("generate: load env-meta: %w", err)
	}
	// options:null + empty envOverride == the canonical defaults the Web API
	// uses for `make gen`.
	gen, err := composegen.Generate(full, modes, envBodyFromMap(lockedEnv), nil, envMeta)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	outDir := filepath.Clean(output)
	var written []string
	for _, mode := range modes {
		sub := filepath.Join(outDir, mode)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return fmt.Errorf("generate: mkdir %s: %w", sub, err)
		}
		compose, ok := gen.Composes[mode]
		if !ok {
			return fmt.Errorf("generate: canonical compose has no mode %q", mode)
		}
		if err := writeGenerated(sub, compose, gen.Env); err != nil {
			return err
		}
		written = append(written, filepath.Join(sub, "docker-compose.yml"), filepath.Join(sub, ".env"))
	}
	if jsonOut {
		emitGenerateJSON(generateJSONResult{
			Profile: "canonical",
			Modes:   modes,
			Output:  outDir,
			OK:      true,
			Written: written,
		})
	} else {
		fmt.Printf("Generated canonical compose (%s) → %s/<mode>/\n", strings.Join(modes, ","), outDir)
	}
	return nil
}

// writeGenerated writes docker-compose.yml and .env into dir.
func writeGenerated(dir string, compose, env []byte) error {
	composePath := filepath.Join(dir, "docker-compose.yml")
	if err := writeFileWithMode(composePath, compose, 0o644); err != nil {
		return fmt.Errorf("generate: write %s: %w", composePath, err)
	}
	envPath := filepath.Join(dir, ".env")
	if err := writeFileWithMode(envPath, env, 0o600); err != nil {
		return fmt.Errorf("generate: write %s: %w", envPath, err)
	}
	return nil
}

func writeFileWithMode(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// keyReaderForSeed returns crypto/rand when seed is empty, otherwise a
// deterministic byte stream derived from the seed. The deterministic stream
// makes `generate` output byte-stable for committed build/* artifacts and
// golden tests. It MUST NOT be used for real deployments (dev/test only).
func keyReaderForSeed(seed string) io.Reader {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return rand.Reader
	}
	return newSeededReader(seed)
}

// seededReader is a deterministic io.Reader producing a SHA-256 keystream:
// block_i = SHA256(seed || counter_i). Repeatable across runs and platforms.
type seededReader struct {
	seed    []byte
	counter uint64
	buf     []byte
}

func newSeededReader(seed string) *seededReader {
	return &seededReader{seed: []byte("stargate-suite-seed:" + seed)}
}

func (r *seededReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if len(r.buf) == 0 {
			h := sha256.New()
			h.Write(r.seed)
			var c [8]byte
			for i := 0; i < 8; i++ {
				c[i] = byte(r.counter >> (8 * uint(i)))
			}
			h.Write(c[:])
			r.buf = h.Sum(nil)
			r.counter++
		}
		m := copy(p[n:], r.buf)
		r.buf = r.buf[m:]
		n += m
	}
	return n, nil
}

func printFindings(findings []policy.Finding) {
	for _, f := range findings {
		fmt.Fprintln(os.Stderr, "  "+f.String())
	}
}
