package contract

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LockedImage records both the human-readable tag and immutable registry
// digest for one component or dependency image.
type LockedImage struct {
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
	Digest  string `yaml:"digest"`
}

// ComponentLock is the reproducible image snapshot derived from Manifest.
type ComponentLock struct {
	SchemaVersion int                    `yaml:"schemaVersion"`
	Images        map[string]LockedImage `yaml:"images"`
}

// DigestResolver resolves an image:tag reference to a sha256 digest.
type DigestResolver func(ref string) (string, error)

func LoadLock(fsys fs.FS, path string) (*ComponentLock, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("read component lock: %w", err)
	}
	return ParseLock(data)
}

func ParseLock(data []byte) (*ComponentLock, error) {
	var lock ComponentLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse component lock: %w", err)
	}
	if lock.Images == nil {
		lock.Images = make(map[string]LockedImage)
	}
	return &lock, nil
}

// NewResolvedLock resolves every manifest image in stable name order.
func NewResolvedLock(manifest *Manifest, resolve DigestResolver) (*ComponentLock, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is nil")
	}
	if resolve == nil {
		return nil, fmt.Errorf("digest resolver is nil")
	}
	refs := make(map[string]LockedImage, len(manifest.Components)+len(manifest.Dependencies))
	for name, component := range manifest.Components {
		refs[name] = LockedImage{Image: component.Image, Version: component.Version}
	}
	for name, dependency := range manifest.Dependencies {
		refs[name] = LockedImage{Image: dependency.Image, Version: dependency.Version}
	}
	names := make([]string, 0, len(refs))
	for name := range refs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item := refs[name]
		ref := item.Image + ":" + item.Version
		digest, err := resolve(ref)
		if err != nil {
			return nil, fmt.Errorf("resolve %s (%s): %w", name, ref, err)
		}
		item.Digest = strings.TrimSpace(digest)
		if !strings.HasPrefix(item.Digest, "sha256:") {
			return nil, fmt.Errorf("resolve %s (%s): invalid digest %q", name, ref, item.Digest)
		}
		refs[name] = item
	}
	return &ComponentLock{SchemaVersion: manifest.SchemaVersion, Images: refs}, nil
}

func MarshalLock(lock *ComponentLock) ([]byte, error) {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return nil, fmt.Errorf("marshal component lock: %w", err)
	}
	return data, nil
}

// ValidateLock ensures the lock is a complete manifest snapshot. Development
// snapshots may leave digests empty; release snapshots require all digests.
func ValidateLock(manifest *Manifest, lock *ComponentLock, requireDigests bool) error {
	if manifest == nil || lock == nil {
		return fmt.Errorf("manifest and lock are required")
	}
	if lock.SchemaVersion != manifest.SchemaVersion {
		return fmt.Errorf("lock schemaVersion %d does not match manifest %d", lock.SchemaVersion, manifest.SchemaVersion)
	}
	want := make(map[string]LockedImage, len(manifest.Components)+len(manifest.Dependencies))
	for name, component := range manifest.Components {
		want[name] = LockedImage{Image: component.Image, Version: component.Version}
	}
	for name, dependency := range manifest.Dependencies {
		want[name] = LockedImage{Image: dependency.Image, Version: dependency.Version}
	}
	for name, expected := range want {
		actual, ok := lock.Images[name]
		if !ok {
			return fmt.Errorf("lock is missing image %q", name)
		}
		if actual.Image != expected.Image || actual.Version != expected.Version {
			return fmt.Errorf("lock image %q is %s:%s, want %s:%s", name, actual.Image, actual.Version, expected.Image, expected.Version)
		}
		if requireDigests && !strings.HasPrefix(actual.Digest, "sha256:") {
			return fmt.Errorf("lock image %q has no sha256 digest", name)
		}
	}
	if len(lock.Images) != len(want) {
		return fmt.Errorf("lock has %d images, manifest has %d", len(lock.Images), len(want))
	}
	return nil
}
