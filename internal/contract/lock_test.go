package contract

import (
	"fmt"
	"strings"
	"testing"
)

func lockTestManifest() *Manifest {
	return &Manifest{
		SchemaVersion: 1,
		Components: map[string]Component{
			"app": {Image: "ghcr.io/example/app", Version: "1.2.3"},
		},
		Dependencies: map[string]Dependency{
			"redis": {Image: "redis", Version: "8-alpine"},
		},
	}
}

func TestNewResolvedLockAndValidate(t *testing.T) {
	lock, err := NewResolvedLock(lockTestManifest(), func(ref string) (string, error) {
		return "sha256:" + strings.Repeat("a", 64), nil
	})
	if err != nil {
		t.Fatalf("NewResolvedLock: %v", err)
	}
	if err := ValidateLock(lockTestManifest(), lock, true); err != nil {
		t.Fatalf("ValidateLock: %v", err)
	}
	data, err := MarshalLock(lock)
	if err != nil {
		t.Fatalf("MarshalLock: %v", err)
	}
	parsed, err := ParseLock(data)
	if err != nil {
		t.Fatalf("ParseLock: %v", err)
	}
	if err := ValidateLock(lockTestManifest(), parsed, true); err != nil {
		t.Fatalf("round-trip ValidateLock: %v", err)
	}
}

func TestNewResolvedLockRejectsInvalidDigest(t *testing.T) {
	_, err := NewResolvedLock(lockTestManifest(), func(ref string) (string, error) {
		return "not-a-digest", nil
	})
	if err == nil {
		t.Fatal("expected invalid digest error")
	}
}

func TestNewResolvedLockWrapsResolverFailure(t *testing.T) {
	_, err := NewResolvedLock(lockTestManifest(), func(ref string) (string, error) {
		return "", fmt.Errorf("registry unavailable")
	})
	if err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Fatalf("expected wrapped resolver error, got %v", err)
	}
}

func TestValidateLockAllowsEmptyDevelopmentDigests(t *testing.T) {
	manifest := lockTestManifest()
	lock := &ComponentLock{SchemaVersion: 1, Images: map[string]LockedImage{
		"app":   {Image: "ghcr.io/example/app", Version: "1.2.3"},
		"redis": {Image: "redis", Version: "8-alpine"},
	}}
	if err := ValidateLock(manifest, lock, false); err != nil {
		t.Fatalf("development snapshot should allow empty digests: %v", err)
	}
	if err := ValidateLock(manifest, lock, true); err == nil {
		t.Fatal("release snapshot must require digests")
	}
}
