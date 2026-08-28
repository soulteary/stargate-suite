// Package main: `generate` subcommand. Produces profile-aware compose + .env
// on disk. Shares the policy + composegen model with the Web UI via
// generateForProfile / validateForProfile (see profile.go). Production strict
// policy violations abort generation (they are real errors, not warnings).
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/stargate-suite/internal/policy"
)

// stringSliceFlag collects repeated --set KEY=VALUE flags.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
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
	policy.EnvHeraldRedisPassword,
	policy.EnvWardenRedisPassword,
	policy.EnvSessionRedisPassword,
	policy.EnvCookieSecure,
}

// collectUserEnv builds the user-supplied env overrides from the process
// environment (only the profile-relevant keys) and explicit --set entries.
// --set wins over the ambient environment.
func collectUserEnv(sets []string) map[string]string {
	env := make(map[string]string)
	for _, k := range profileSecretEnvKeys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			env[k] = v
		}
	}
	for _, kv := range sets {
		if i := strings.IndexByte(kv, '='); i > 0 {
			env[strings.TrimSpace(kv[:i])] = strings.TrimSpace(kv[i+1:])
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
	seed := fs.String("seed", "", "deterministic seed for auto-generated dev/test keys (byte-stable output; leave empty for crypto/rand). Never use a real seed for real deployments.")
	var sets stringSliceFlag
	fs.Var(&sets, "set", "override an env value as KEY=VALUE (repeatable); also read from process env for known secret keys")
	if err := fs.Parse(cmdArgs); err != nil {
		return err
	}

	prof, err := resolveProfile(*profileName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*output) == "" {
		return fmt.Errorf("generate: --output directory is required")
	}

	userEnv := collectUserEnv(sets)

	modes := defaultModesForProfile(prof)
	if strings.TrimSpace(*modesCSV) != "" {
		modes = splitCSV(*modesCSV)
	}

	// Enforce profile policy BEFORE writing. In strict profiles (test /
	// production) any error aborts. production must never be bypassable.
	findings := validateForProfile(prof, nil, userEnv)
	if policy.HasErrors(findings) {
		printFindings(findings)
		if prof.Name == policy.Production || !*force {
			return fmt.Errorf("generate: profile %q has policy violations; refusing to generate (supply real secrets via env/--set; production strict rules are hard errors)", prof.Name)
		}
	} else if len(findings) > 0 {
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
	// Single mode → write directly under outDir; multiple → subdir per mode.
	if len(modes) == 1 {
		if err := writeGenerated(outDir, gen.Composes[modes[0]], gen.Env); err != nil {
			return err
		}
		fmt.Printf("Generated profile %q (%s) → %s\n", prof.Name, modes[0], outDir)
	} else {
		for _, mode := range modes {
			sub := filepath.Join(outDir, mode)
			if err := os.MkdirAll(sub, 0o755); err != nil {
				return fmt.Errorf("generate: mkdir %s: %w", sub, err)
			}
			if err := writeGenerated(sub, gen.Composes[mode], gen.Env); err != nil {
				return err
			}
		}
		fmt.Printf("Generated profile %q (%s) → %s/<mode>/\n", prof.Name, strings.Join(modes, ","), outDir)
	}
	if prof.Experimental {
		fmt.Printf("note: profile %q is experimental; review generated config before real deployment\n", prof.Name)
	}
	return nil
}

// writeGenerated writes docker-compose.yml and .env into dir.
func writeGenerated(dir string, compose, env []byte) error {
	composePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composePath, compose, 0o644); err != nil {
		return fmt.Errorf("generate: write %s: %w", composePath, err)
	}
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, env, 0o644); err != nil {
		return fmt.Errorf("generate: write %s: %w", envPath, err)
	}
	return nil
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
