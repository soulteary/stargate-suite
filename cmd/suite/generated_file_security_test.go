package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/stargate-suite/internal/composegen"
)

func TestWriteGeneratedRestrictsEnvPermissions(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("prepare existing env: %v", err)
	}
	if err := writeGenerated(dir, []byte("services: {}\n"), []byte("SECRET=value\n")); err != nil {
		t.Fatalf("writeGenerated: %v", err)
	}
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("stat env: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("env permissions=%#o, want 0600", got)
	}
	composeInfo, err := os.Stat(filepath.Join(dir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("stat compose: %v", err)
	}
	if got := composeInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("compose permissions=%#o, want 0644", got)
	}
}

func TestGeneratedEnvRoundTripsThroughImporter(t *testing.T) {
	want := map[string]string{
		"APOSTROPHE": "operator's secret",
		"MULTILINE":  "line one\nline two",
		"BACKSLASH":  `C:\path\with\slashes`,
	}
	body := composegen.EnvBodyFromVars(want, "", nil)
	got := parseEnvText(body)
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s round-trip = %q, want %q", key, got[key], value)
		}
	}
}

func TestSetFlagRejectsInvalidEnvNames(t *testing.T) {
	var values stringSliceFlag
	for _, invalid := range []string{"NO_EQUALS", "2BAD=value", "BAD-NAME=value", "BAD\nINJECT=value"} {
		if err := values.Set(invalid); err == nil {
			t.Errorf("Set(%q) succeeded, want error", invalid)
		}
	}
	if err := values.Set("VALID= value with spaces "); err != nil {
		t.Fatalf("valid value rejected: %v", err)
	}
	env := collectUserEnv(values)
	if got := env["VALID"]; got != " value with spaces " {
		t.Fatalf("value whitespace was changed: %q", got)
	}
}
