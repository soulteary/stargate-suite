// Package contract holds the component manifest (single source of truth for
// component versions, images, ports, health paths, and contract versions) plus
// cross-file consistency tests that guard against configuration drift.
//
// The manifest (config/components.yaml) is the authoritative registry required
// by the upgrade rules (M-01): env-meta.yaml, .env.example, compose/canonical
// and composegen defaults must all derive from it, and `suite version` reads
// the verified target combo from it. The drift tests in manifest_test.go fail
// if any of those sources re-hardcode a value that diverges from the manifest.
package contract

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// ManifestPath is the repository-relative, slash-separated path to the manifest
// within the embedded asset FS.
const ManifestPath = "config/components.yaml"

// LockPath is the repository-relative path to the digest lock file.
const LockPath = "config/components.lock.yaml"

// Component is one registered component's current (pre-migration) contract.
type Component struct {
	Image           string `yaml:"image"`
	Version         string `yaml:"version"`
	ContractVersion string `yaml:"contractVersion"`
	ContainerPort   int    `yaml:"containerPort"`
	LivenessPath    string `yaml:"livenessPath"`
	ReadinessPath   string `yaml:"readinessPath"`
}

// Ref returns the "image:version" reference for a component (e.g.
// "ghcr.io/soulteary/herald:v0.9.0"). Empty when image or version is unset.
func (c Component) Ref() string {
	if c.Image == "" || c.Version == "" {
		return ""
	}
	return c.Image + ":" + c.Version
}

// Dependency is a non-suite image (redis, whoami) registered to avoid drift.
type Dependency struct {
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

// Ref returns the "image:version" reference for a dependency.
func (d Dependency) Ref() string {
	if d.Image == "" || d.Version == "" {
		return ""
	}
	return d.Image + ":" + d.Version
}

// Manifest is the parsed config/components.yaml.
type Manifest struct {
	SchemaVersion int                   `yaml:"schemaVersion"`
	Components    map[string]Component  `yaml:"components"`
	Dependencies  map[string]Dependency `yaml:"dependencies"`
	// VerifiedCombo is the target v1 component combination this Suite release is
	// validated against (read-only contract source). It is intentionally kept
	// separate from Components, which registers the current (old) running values
	// until the atomic upgrade (PR 8) converges them.
	VerifiedCombo map[string]string `yaml:"verifiedCombo"`
}

// Component returns the named component and whether it exists.
func (m *Manifest) Component(name string) (Component, bool) {
	if m == nil {
		return Component{}, false
	}
	c, ok := m.Components[name]
	return c, ok
}

// LoadManifest reads and parses the manifest from an fs.FS (typically the
// embedded assets FS). Callers pass ManifestPath.
func LoadManifest(fsys fs.FS, path string) (*Manifest, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("read component manifest: %w", err)
	}
	return ParseManifest(data)
}

// ParseManifest parses manifest YAML bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse component manifest: %w", err)
	}
	if m.Components == nil {
		m.Components = make(map[string]Component)
	}
	return &m, nil
}
