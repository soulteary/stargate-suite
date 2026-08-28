// Package composegen: load env-meta.yaml for order, comments, defaults, and service allowlist.
package composegen

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

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
	return parseEnvMeta(data)
}

// LoadEnvMetaFS reads and parses env-meta.yaml from an fs.FS (embedded assets or
// read-only override). Returns nil, nil if the file does not exist.
func LoadEnvMetaFS(fsys fs.FS, path string) (*EnvMeta, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read env-meta: %w", err)
	}
	return parseEnvMeta(data)
}

func parseEnvMeta(data []byte) (*EnvMeta, error) {
	var meta EnvMeta
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&meta); err != nil {
		return nil, fmt.Errorf("parse env-meta: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse env-meta: multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("parse env-meta: %w", err)
	}
	if err := validateEnvMeta(&meta); err != nil {
		return nil, fmt.Errorf("validate env-meta: %w", err)
	}
	return &meta, nil
}

func validateEnvMeta(meta *EnvMeta) error {
	if len(meta.Order) == 0 {
		return fmt.Errorf("order must not be empty")
	}
	if len(meta.Vars) == 0 {
		return fmt.Errorf("vars must not be empty")
	}
	seenOrder := make(map[string]bool, len(meta.Order))
	for i, key := range meta.Order {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("order[%d] must not be empty", i)
		}
		if seenOrder[key] {
			return fmt.Errorf("order contains duplicate key %q", key)
		}
		seenOrder[key] = true
		if _, ok := meta.Vars[key]; !ok {
			return fmt.Errorf("order key %q is not declared in vars", key)
		}
	}

	allowedServices := map[string]bool{
		"herald": true, "herald-dingtalk": true, "herald-smtp": true,
		"herald-totp": true, "stargate": true, "warden": true,
	}
	for key, item := range meta.Vars {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("vars contains an empty key")
		}
		if strings.TrimSpace(item.Comment) == "" {
			return fmt.Errorf("vars.%s.comment must not be empty", key)
		}
		seenServices := make(map[string]bool, len(item.Services))
		for _, service := range item.Services {
			if !allowedServices[service] {
				return fmt.Errorf("vars.%s.services contains unknown service %q", key, service)
			}
			if seenServices[service] {
				return fmt.Errorf("vars.%s.services contains duplicate service %q", key, service)
			}
			seenServices[service] = true
		}
	}
	return nil
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
