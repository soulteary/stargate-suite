package main

import (
	"os"
	"path/filepath"
	"testing"
)

// resetConfigDir restores the global override after a test mutates it.
func resetConfigDir(t *testing.T) {
	t.Helper()
	prev := configDirOverride
	t.Cleanup(func() { configDirOverride = prev })
}

// TestEmbeddedAssetsLoadWithoutRepo proves serve/validate no longer depend on
// the repository source tree: loadPageData reads everything from the embedded
// FS even when the working directory has no config/ or compose/ (B-02).
func TestEmbeddedAssetsLoadWithoutRepo(t *testing.T) {
	resetConfigDir(t)
	configDirOverride = ""
	// Change into an empty temp dir with no repo assets on disk.
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()

	page, err := loadPageData(pageYAMLPath)
	if err != nil {
		t.Fatalf("loadPageData from embedded assets: %v", err)
	}
	if page == nil {
		t.Fatal("expected non-nil page data")
	}
	if len(page.Services) == 0 {
		t.Error("expected embedded services.yaml to populate page.Services")
	}
	if err := cmdValidate(); err != nil {
		t.Fatalf("cmdValidate with embedded assets: %v", err)
	}
}

// TestReadAssetEmbedded reads a known embedded asset directly.
func TestReadAssetEmbedded(t *testing.T) {
	resetConfigDir(t)
	configDirOverride = ""
	b, err := readAsset("compose/canonical/docker-compose.yml")
	if err != nil {
		t.Fatalf("readAsset canonical compose: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("canonical compose is empty")
	}
}

// TestConfigDirOverride verifies that --config-dir overrides embedded config
// files, while non-config paths (compose/) still fall back to embedded assets.
func TestConfigDirOverride(t *testing.T) {
	resetConfigDir(t)
	dir := t.TempDir()
	// Write a minimal ports.yaml override with a recognizable marker.
	const override = "ports:\n  - serviceKey: warden\n    optionId: portWarden\n    containerPort: \"9999\"\n"
	if err := os.WriteFile(filepath.Join(dir, "ports.yaml"), []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}
	configDirOverride = dir

	b, err := readAsset("config/ports.yaml")
	if err != nil {
		t.Fatalf("readAsset overridden ports.yaml: %v", err)
	}
	if string(b) != override {
		t.Errorf("expected overridden ports.yaml, got:\n%s", b)
	}

	// compose/ is not under config/, so it must still resolve from embedded.
	c, err := readAsset("compose/canonical/docker-compose.yml")
	if err != nil {
		t.Fatalf("readAsset compose with override active: %v", err)
	}
	if len(c) == 0 {
		t.Fatal("compose should fall back to embedded when override lacks it")
	}
}

// TestConfigDirOverrideFallback verifies that a missing file in the override
// directory falls back to the embedded asset (partial override is allowed).
func TestConfigDirOverrideFallback(t *testing.T) {
	resetConfigDir(t)
	dir := t.TempDir() // empty: nothing overridden
	configDirOverride = dir

	b, err := readAsset("config/page.yaml")
	if err != nil {
		t.Fatalf("readAsset page.yaml should fall back to embedded: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("page.yaml fallback is empty")
	}
}

// TestNonexistentConfigDirFallsBack mirrors the documented behavior of
// `validate --config-dir=/nonexistent`: it must still succeed via embedded assets.
func TestNonexistentConfigDirFallsBack(t *testing.T) {
	resetConfigDir(t)
	configDirOverride = filepath.Join(t.TempDir(), "does-not-exist")
	dir := t.TempDir()
	restore := chdir(t, dir)
	defer restore()

	if err := cmdValidate(); err != nil {
		t.Fatalf("cmdValidate with nonexistent --config-dir should fall back to embedded: %v", err)
	}
}

func TestMalformedConfigOverridesFailClosed(t *testing.T) {
	resetConfigDir(t)
	cases := []struct {
		name    string
		path    string
		content string
	}{
		{"services yaml", "services.yaml", "services: [\n"},
		{"scenarios json", "scenarios.json", "{not-json"},
		{"profiles yaml", "profiles.yaml", "profiles: [\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.path), []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write malformed override: %v", err)
			}
			configDirOverride = dir
			if _, err := loadPageData(pageYAMLPath); err == nil {
				t.Fatalf("loadPageData accepted malformed %s", tc.path)
			}
		})
	}
}

func TestMalformedEnvMetaOverrideBlocksGeneration(t *testing.T) {
	resetConfigDir(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "env-meta.yaml"), []byte("order: [BROKEN]\nvars: {}\n"), 0o644); err != nil {
		t.Fatalf("write malformed env-meta override: %v", err)
	}
	configDirOverride = dir
	if err := generateCanonical(t.TempDir(), "image", false); err == nil {
		t.Fatal("canonical generation ignored invalid env-meta override")
	}
}

func TestValidateRejectsUnknownScenarioOption(t *testing.T) {
	resetConfigDir(t)
	dir := t.TempDir()
	const scenarios = `{
  "invalid": {
    "modes": ["traefik"],
    "options": {"unknownOption": true}
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "scenarios.json"), []byte(scenarios), 0o644); err != nil {
		t.Fatalf("write scenarios override: %v", err)
	}
	configDirOverride = dir
	prevArgs := cmdArgs
	cmdArgs = nil
	t.Cleanup(func() { cmdArgs = prevArgs })
	if err := cmdValidate(); err == nil {
		t.Fatal("cmdValidate accepted an unknown scenario option")
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	return func() { _ = os.Chdir(prev) }
}
