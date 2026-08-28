package policy

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// knownCodes is the set of structured finding codes the validator can emit.
// Keep in sync with the Code* constants in schema.go; the schema-drift test
// asserts config/schemas/env-fields.yaml only references codes in this set.
var knownCodes = map[string]bool{
	CodePasswordsRequired:       true,
	CodePasswordsPlaintext:      true,
	CodeCookieSecureRequired:    true,
	CodeSessionExchangeSecret:   true,
	CodeStepUpPathsRequired:     true,
	CodeStepUpProxiesRequired:   true,
	CodeCallbackHostsInvalid:    true,
	CodeTrustedProxiesInvalid:   true,
	CodeHeraldAPIKeyWeak:        true,
	CodeWardenAPIKeyWeak:        true,
	CodeHmacSecretWeak:          true,
	CodeHmacV1Forbidden:         true,
	CodeHmacDriftInvalid:        true,
	CodeIdempotencySecretWeak:   true,
	CodePIIPepperWeak:           true,
	CodeRedisPasswordRequired:   true,
	CodeExposePortsForbidden:    true,
	CodeHeraldTestModeForbidden: true,
	CodeContainerPrivileges:     true,
	CodeContainerReadonly:       true,
	CodeAuthModeMismatch:        true,
	CodeAuthModeAmbiguous:       true,
	CodeTLSPairIncomplete:       true,
	CodeUnknownEnvVar:           true,
	CodeDurationInvalid:         true,
	CodePortInvalid:             true,
	CodeURLInvalid:              true,
	CodeBoolInvalid:             true,
}

// TestSchemaDriftEnvFields asserts every code declared in the env-fields schema
// exists as a policy code constant, so the declarative doc and the enforcement
// engine never diverge.
func TestSchemaDriftEnvFields(t *testing.T) {
	path := filepath.Join("..", "..", "config", "schemas", "env-fields.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var doc struct {
		Fields map[string]struct {
			Type string `yaml:"type"`
			Code string `yaml:"code"`
		} `yaml:"fields"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if len(doc.Fields) == 0 {
		t.Fatal("schema declares no fields")
	}
	for name, f := range doc.Fields {
		if f.Code == "" {
			t.Errorf("schema field %q has no code", name)
			continue
		}
		if !knownCodes[f.Code] {
			t.Errorf("schema field %q references unknown code %q (add to schema.go / knownCodes)", name, f.Code)
		}
	}
}
