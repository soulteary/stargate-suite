package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/stargate-suite/internal/contract"
)

func TestLoadLockedImageEnv(t *testing.T) {
	lock, err := contract.LoadLock(assetFS(), contract.LockPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	for name, item := range lock.Images {
		item.Digest = digest
		lock.Images[name] = item
	}
	data, err := contract.MarshalLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "components.lock.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	env, err := loadLockedImageEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := env["STARGATE_IMAGE"], lock.Images["stargate"].Image+"@"+digest; got != want {
		t.Fatalf("STARGATE_IMAGE = %q, want %q", got, want)
	}
	if got, want := env["OWLMAIL_IMAGE"], lock.Images["owlmail"].Image+"@"+digest; got != want {
		t.Fatalf("OWLMAIL_IMAGE = %q, want %q", got, want)
	}
	for _, key := range []string{"HERALD_REDIS_IMAGE", "WARDEN_REDIS_IMAGE", "STARGATE_REDIS_IMAGE"} {
		if got, want := env[key], lock.Images["redis"].Image+"@"+digest; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLoadLockedImageEnvRejectsPlaceholderDigests(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", contract.LockPath))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "components.lock.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLockedImageEnv(path); err == nil {
		t.Fatal("expected placeholder lock to be rejected")
	}
}

func TestLockMappingsCoverEveryManifestImage(t *testing.T) {
	manifest, err := loadManifest()
	if err != nil {
		t.Fatal(err)
	}
	for name := range manifest.Components {
		if len(lockImageEnvVars[name]) == 0 {
			t.Errorf("component %q has no locked environment mapping", name)
		}
	}
	for name := range manifest.Dependencies {
		if len(lockImageEnvVars[name]) == 0 {
			t.Errorf("dependency %q has no locked environment mapping", name)
		}
	}
}

func TestGenerateCanonicalWithEnvUsesLockedImages(t *testing.T) {
	dir := t.TempDir()
	ref := "ghcr.io/soulteary/stargate@sha256:" + strings.Repeat("b", 64)
	if err := generateCanonicalWithEnv(dir, "image", false, map[string]string{"STARGATE_IMAGE": ref}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "image", ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "STARGATE_IMAGE="+ref+"\n") {
		t.Fatalf("generated .env does not contain locked image %q", ref)
	}
}
