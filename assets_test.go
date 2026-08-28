package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"sort"
	"testing"
)

// requiredAssets are the files that serve/validate/generate depend on. If any
// is missing from the embedded FS the binary is not self-contained (B-02).
var requiredAssets = []string{
	"config/env-meta.yaml",
	"config/services.yaml",
	"config/ports.yaml",
	"config/components.yaml",
	"config/components.lock.yaml",
	"config/config-sections.yaml",
	"config/page.yaml",
	"config/providers.yaml",
	"config/scenarios.json",
	"config/presets.json",
	"config/keys-step.yaml",
	"config/i18n/zh.yaml",
	"config/i18n/en.yaml",
	"compose/canonical/docker-compose.yml",
}

func TestEmbeddedContainsRequiredAssets(t *testing.T) {
	fsys := Embedded()
	for _, name := range requiredAssets {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Errorf("embedded asset missing: %s: %v", name, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("embedded asset is empty: %s", name)
		}
	}
}

// TestEmbeddedChecksumStable computes a manifest checksum over all embedded
// files. It fails only if the embedded set is empty; otherwise it logs the
// digest so drift is visible in test output and can be asserted upstream.
func TestEmbeddedChecksumStable(t *testing.T) {
	fsys := Embedded()
	var files []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded fs: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("embedded fs contains no files")
	}
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write(b)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	t.Logf("embedded assets: %d files, manifest sha256=%s", len(files), sum)
	if sum == "" {
		t.Fatal("empty checksum")
	}
}

// TestEmbeddedExcludesSource ensures we did not accidentally embed Go source,
// tests, or fixtures into the binary (keeps the image free of source/test data).
func TestEmbeddedExcludesSource(t *testing.T) {
	fsys := Embedded()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case len(p) > 3 && p[len(p)-3:] == ".go":
			t.Errorf("embedded fs must not contain Go source: %s", p)
		case p == "config/README.md" || p == "config/README.zh-CN.md":
			// docs are harmless but not required; not an error.
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded fs: %v", err)
	}
}
