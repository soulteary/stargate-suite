// Package composegen: load env-meta.yaml for order, comments, defaults, and service allowlist.
package composegen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// EnvVarMeta describes one env key: comment, which services may use it, optional default.
type EnvVarMeta struct {
	Comment  string   `yaml:"comment"`
	Services []string `yaml:"services"`
	Default  string   `yaml:"default"`
}

// EnvMeta is the loaded env-meta config (order + per-key meta).
type EnvMeta struct {
	Order []string              `yaml:"order"`
	Vars  map[string]EnvVarMeta `yaml:"vars"`
}

// LoadEnvMeta reads and parses env-meta.yaml from path. Returns nil, nil if file does not exist (caller uses built-in).
func LoadEnvMeta(path string) (*EnvMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read env-meta: %w", err)
	}
	var meta EnvMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse env-meta: %w", err)
	}
	if meta.Vars == nil {
		meta.Vars = make(map[string]EnvVarMeta)
	}
	return &meta, nil
}

// Comments returns env key -> comment for compose/.env. Keys not in meta get empty string.
func (m *EnvMeta) Comments() map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m.Vars))
	for k, v := range m.Vars {
		out[k] = v.Comment
	}
	return out
}

// OrderKeys returns the preferred .env key order. Empty if nil.
func (m *EnvMeta) OrderKeys() []string {
	if m == nil || len(m.Order) == 0 {
		return nil
	}
	return m.Order
}

// Defaults returns env key -> default value (only keys that have a default).
func (m *EnvMeta) Defaults() map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string)
	for k, v := range m.Vars {
		if v.Default != "" {
			out[k] = v.Default
		}
	}
	return out
}

// ServiceAllowedEnvKeys returns service name -> set of env keys allowed for that service (for validation / allowlist).
func (m *EnvMeta) ServiceAllowedEnvKeys() map[string]map[string]bool {
	if m == nil {
		return nil
	}
	out := make(map[string]map[string]bool)
	for key, meta := range m.Vars {
		for _, svc := range meta.Services {
			if out[svc] == nil {
				out[svc] = make(map[string]bool)
			}
			out[svc][key] = true
		}
	}
	return out
}
