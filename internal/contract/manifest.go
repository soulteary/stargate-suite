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
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestPath is the repository-relative, slash-separated path to the manifest
// within the embedded asset FS.
const ManifestPath = "config/components.yaml"

// LockPath is the repository-relative path to the digest lock file.
const LockPath = "config/components.lock.yaml"

// Component is one registered component's current runtime contract.
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
	// VerifiedCombo is the component combination this Suite release is validated
	// against. It remains separate from Components so the CLI can display the
	// verified release matrix explicitly.
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
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse component manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse component manifest: multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("parse component manifest: %w", err)
	}
	if err := validateManifest(&m); err != nil {
		return nil, fmt.Errorf("validate component manifest: %w", err)
	}
	return &m, nil
}

var manifestNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var requiredManifestComponents = []string{
	"stargate",
	"warden",
	"herald",
	"herald-totp",
	"herald-dingtalk",
	"herald-smtp",
}

var requiredManifestDependencies = []string{
	"redis",
	"protected",
	"owlmail",
}

var requiredVerifiedComponents = []string{
	"stargate",
	"warden",
	"herald",
}

func validateManifest(m *Manifest) error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("schemaVersion must be 1 (got %d)", m.SchemaVersion)
	}
	if len(m.Components) == 0 {
		return fmt.Errorf("components must not be empty")
	}
	for _, name := range requiredManifestComponents {
		if _, ok := m.Components[name]; !ok {
			return fmt.Errorf("components must include %q", name)
		}
	}
	for name, component := range m.Components {
		if err := validateManifestName(name); err != nil {
			return fmt.Errorf("component %q: %w", name, err)
		}
		if err := validateImageVersion(component.Image, component.Version); err != nil {
			return fmt.Errorf("component %q: %w", name, err)
		}
		if strings.TrimSpace(component.ContractVersion) == "" {
			return fmt.Errorf("component %q: contractVersion is required", name)
		}
		if component.ContainerPort < 1 || component.ContainerPort > 65535 {
			return fmt.Errorf("component %q: containerPort must be between 1 and 65535", name)
		}
		if !validHealthPath(component.LivenessPath) {
			return fmt.Errorf("component %q: livenessPath must be an absolute path", name)
		}
		if component.ReadinessPath != "" && !validHealthPath(component.ReadinessPath) {
			return fmt.Errorf("component %q: readinessPath must be an absolute path", name)
		}
	}
	for name, dependency := range m.Dependencies {
		if err := validateManifestName(name); err != nil {
			return fmt.Errorf("dependency %q: %w", name, err)
		}
		if _, exists := m.Components[name]; exists {
			return fmt.Errorf("name %q is shared by a component and dependency", name)
		}
		if err := validateImageVersion(dependency.Image, dependency.Version); err != nil {
			return fmt.Errorf("dependency %q: %w", name, err)
		}
	}
	for _, name := range requiredManifestDependencies {
		if _, ok := m.Dependencies[name]; !ok {
			return fmt.Errorf("dependencies must include %q", name)
		}
	}
	if len(m.VerifiedCombo) == 0 {
		return fmt.Errorf("verifiedCombo must not be empty")
	}
	for _, name := range requiredVerifiedComponents {
		if _, ok := m.VerifiedCombo[name]; !ok {
			return fmt.Errorf("verifiedCombo must include %q", name)
		}
	}
	for name, version := range m.VerifiedCombo {
		component, ok := m.Components[name]
		if !ok {
			return fmt.Errorf("verifiedCombo references unknown component %q", name)
		}
		if strings.TrimPrefix(version, "v") != strings.TrimPrefix(component.Version, "v") {
			return fmt.Errorf("verifiedCombo %s=%q does not match component version %q", name, version, component.Version)
		}
	}
	return nil
}

func validateManifestName(name string) error {
	if !manifestNamePattern.MatchString(name) {
		return fmt.Errorf("name must use lowercase letters, numbers, and single hyphens")
	}
	return nil
}

func validateImageVersion(image, version string) error {
	if strings.TrimSpace(image) == "" || strings.ContainsAny(image, " \t\r\n") {
		return fmt.Errorf("image is required and must not contain whitespace")
	}
	if strings.Contains(image, "@") {
		return fmt.Errorf("image must not contain a digest; use the component lock")
	}
	if strings.TrimSpace(version) == "" || strings.ContainsAny(version, " \t\r\n") {
		return fmt.Errorf("version is required and must not contain whitespace")
	}
	return nil
}

func validHealthPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.ContainsAny(path, " \t\r\n")
}
