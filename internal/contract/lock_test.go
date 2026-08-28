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
	for _, digest := range []string{
		"not-a-digest",
		"sha256:",
		"sha256:abc123",
		"sha256:" + strings.Repeat("g", 64),
		"sha256:" + strings.Repeat("a", 65),
	} {
		t.Run(digest, func(t *testing.T) {
			_, err := NewResolvedLock(lockTestManifest(), func(string) (string, error) {
				return digest, nil
			})
			if err == nil {
				t.Fatal("expected invalid digest error")
			}
		})
	}
}

func TestValidateLockRejectsMalformedOptionalDigest(t *testing.T) {
	manifest := lockTestManifest()
	lock := &ComponentLock{SchemaVersion: 1, Images: map[string]LockedImage{
		"app":   {Image: "ghcr.io/example/app", Version: "1.2.3", Digest: "sha256:short"},
		"redis": {Image: "redis", Version: "8-alpine"},
	}}
	if err := ValidateLock(manifest, lock, false); err == nil {
		t.Fatal("development lock accepted a supplied malformed digest")
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

func TestLockRejectsComponentDependencyNameCollision(t *testing.T) {
	manifest := lockTestManifest()
	manifest.Dependencies["app"] = Dependency{Image: "example.invalid/dependency", Version: "latest"}

	if _, err := NewResolvedLock(manifest, func(string) (string, error) {
		return "sha256:" + strings.Repeat("a", 64), nil
	}); err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("NewResolvedLock collision error = %v", err)
	}
	if err := ValidateLock(manifest, &ComponentLock{SchemaVersion: 1}, false); err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("ValidateLock collision error = %v", err)
	}
}
