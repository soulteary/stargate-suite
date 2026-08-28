package composegen

import (
	"strings"
	"testing"
)

const validEnvMeta = `
order:
  - FOO
vars:
  FOO:
    comment: example
    services: [stargate]
    default: value
`

func TestParseEnvMetaValidatesStructure(t *testing.T) {
	if _, err := parseEnvMeta([]byte(validEnvMeta)); err != nil {
		t.Fatalf("parse valid env-meta: %v", err)
	}

	tests := map[string]string{
		"unknown field":   strings.Replace(validEnvMeta, "    comment:", "    commment:", 1),
		"duplicate order": strings.Replace(validEnvMeta, "  - FOO\nvars:", "  - FOO\n  - FOO\nvars:", 1),
		"missing var":     strings.Replace(validEnvMeta, "  - FOO", "  - MISSING", 1),
		"unknown service": strings.Replace(validEnvMeta, "stargate", "unknown", 1),
		"empty comment":   strings.Replace(validEnvMeta, "example", "''", 1),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseEnvMeta([]byte(data)); err == nil {
				t.Fatal("parseEnvMeta accepted invalid metadata")
			}
		})
	}
}
