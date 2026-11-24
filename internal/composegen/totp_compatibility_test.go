package composegen

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGeneratedTotpUsesHeraldProxy(t *testing.T) {
	canonical, err := os.ReadFile("../../compose/canonical/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"traefik", "traefik-herald", "traefik-stargate", "image", "build"} {
		for _, enabled := range []bool{false, true} {
			var full map[string]interface{}
			if err := yaml.Unmarshal(canonical, &full); err != nil {
				t.Fatal(err)
			}
			generated, err := GenerateOne(full, mode, &Options{
				IncludeTotp: enabled, HealthCheck: true, UseNamedVolume: true,
				ContainerNamePrefix: "isolated-",
			})
			if err != nil {
				t.Fatalf("%s enabled=%v: %v", mode, enabled, err)
			}
			var compose struct {
				Services map[string]struct {
					Environment yaml.Node `yaml:"environment"`
					Healthcheck struct {
						Test []string `yaml:"test"`
					} `yaml:"healthcheck"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(generated, &compose); err != nil {
				t.Fatal(err)
			}
			withBackend := enabled && mode != "image" && mode != "build"
			for name, service := range compose.Services {
				if name != "herald" && name != "stargate" {
					continue
				}
				var environment []string
				if err := service.Environment.Decode(&environment); err != nil {
					t.Fatal(err)
				}
				for _, entry := range environment {
					if !strings.HasPrefix(entry, "HERALD_TOTP_") {
						continue
					}
					if !withBackend {
						t.Errorf("%s: %s retains disabled backend setting %s", mode, name, entry)
					}
					if name == "stargate" && !strings.HasPrefix(entry, "HERALD_TOTP_ENABLED=") {
						t.Errorf("%s: Stargate receives Herald-owned backend setting %s", mode, entry)
					}
				}
				if name == "herald" && withBackend {
					env := strings.Join(environment, "\n")
					for _, expected := range []string{
						"HERALD_TOTP_BASE_URL=http://isolated-herald-totp:8084",
						"HERALD_TOTP_API_KEY=${HERALD_TOTP_API_KEY:-}",
						"HERALD_TOTP_HMAC_SECRET=${HERALD_TOTP_HMAC_SECRET:-}",
					} {
						if !strings.Contains(env, expected) {
							t.Errorf("%s: Herald is missing %s", mode, expected)
						}
					}
				}
			}
			if backend, ok := compose.Services["herald-totp"]; ok {
				probe := strings.Join(backend.Healthcheck.Test, " ")
				if !strings.Contains(probe, "wget") || strings.Contains(probe, "curl") {
					t.Errorf("%s: health probe requires a tool absent from the TOTP runtime: %s", mode, probe)
				}
			}
		}
	}
}
