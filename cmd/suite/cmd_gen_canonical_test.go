package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/soulteary/stargate-suite/internal/composegen"
)

// TestCanonicalGenerationMatchesComposegen asserts the CLI `generate --canonical`
// path (generateCanonical) produces the SAME bytes as calling composegen.Generate
// directly with options:null + empty envOverride — the exact call the Web
// /api/generate handler makes for `make gen`. This is the PR10 single-source
// guarantee: CLI and Web UI generate identical canonical output.
func TestCanonicalGenerationMatchesComposegen(t *testing.T) {
	modes := canonicalBuildModes

	// Reference: the raw composegen path the Web /api/generate uses (options:null).
	full, err := composegen.LoadComposeFS(assetFS(), canonicalCompose)
	if err != nil {
		t.Fatalf("load canonical compose: %v", err)
	}
	applyManifestToComposegen()
	envMeta, _ := composegen.LoadEnvMetaFS(assetFS(), "config/env-meta.yaml")
	ref, err := composegen.Generate(full, modes, "", nil, envMeta)
	if err != nil {
		t.Fatalf("reference generate: %v", err)
	}
	for _, mode := range []string{"image", "build"} {
		if !bytes.Contains(ref.Composes[mode], []byte("healthcheck:")) {
			t.Errorf("mode %q must preserve canonical health checks when options are nil", mode)
		}
	}

	// CLI path: write to a temp dir and read back.
	dir := t.TempDir()
	if err := generateCanonical(dir, "", false); err != nil {
		t.Fatalf("generateCanonical: %v", err)
	}
	for _, mode := range modes {
		gotCompose, err := os.ReadFile(dir + "/" + mode + "/docker-compose.yml")
		if err != nil {
			t.Fatalf("read %q compose: %v", mode, err)
		}
		if !bytes.Equal(gotCompose, ref.Composes[mode]) {
			t.Errorf("mode %q compose differs from composegen reference", mode)
		}
		gotEnv, err := os.ReadFile(dir + "/" + mode + "/.env")
		if err != nil {
			t.Fatalf("read %q .env: %v", mode, err)
		}
		if !bytes.Equal(gotEnv, ref.Env) {
			t.Errorf("mode %q .env differs from composegen reference", mode)
		}
	}
}

// TestCanonicalGenerationModeSubset asserts --modes narrows the generated set.
func TestCanonicalGenerationModeSubset(t *testing.T) {
	dir := t.TempDir()
	if err := generateCanonical(dir, "image,traefik", false); err != nil {
		t.Fatalf("generateCanonical: %v", err)
	}
	// Requested modes exist.
	for _, m := range []string{"image", "traefik"} {
		if _, err := os.ReadFile(dir + "/" + m + "/docker-compose.yml"); err != nil {
			t.Errorf("expected mode %q to be generated: %v", m, err)
		}
	}
	// A mode not requested must not be generated.
	if _, err := os.ReadFile(dir + "/build/docker-compose.yml"); err == nil {
		t.Errorf("mode build should not be generated when not requested")
	}
}
