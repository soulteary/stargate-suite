package composegen

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInjectOwlmailUsesVersionedImageVariable(t *testing.T) {
	services := make(map[string]interface{})
	injectOwlmailService(services, &Options{})
	owlmail, ok := services["owlmail"].(map[string]interface{})
	if !ok {
		t.Fatal("owlmail service was not injected")
	}
	image, _ := owlmail["image"].(string)
	if image != "${OWLMAIL_IMAGE:-ghcr.io/soulteary/owlmail:0.4.0}" {
		t.Fatalf("owlmail image = %q", image)
	}
	if strings.Contains(image, ":latest") {
		t.Fatal("owlmail image must not use latest")
	}
}

func TestStargateSplitOverridesRewriteCurrentCanonicalValues(t *testing.T) {
	service := map[string]interface{}{
		"environment": []interface{}{
			"WARDEN_URL=${WARDEN_URL:-http://warden:8081}",
			"HERALD_URL=${HERALD_URL:-http://herald:8082}",
			"HERALD_TOTP_BASE_URL=${HERALD_TOTP_BASE_URL:-http://herald-totp:8084}",
		},
		"labels": []interface{}{
			"traefik.http.middlewares.stargate-auth.forwardauth.address=http://stargate:8080/_auth",
		},
		"depends_on": []interface{}{"warden", "herald"},
	}
	applyStargateSplitOverrides(service, "isolated-", nil)

	env := service["environment"].([]interface{})
	wantEnv := []string{
		"WARDEN_URL=http://isolated-warden:8081",
		"HERALD_URL=http://isolated-herald:8082",
		"HERALD_TOTP_BASE_URL=http://isolated-herald-totp:8084",
	}
	for i, want := range wantEnv {
		if env[i] != want {
			t.Errorf("environment[%d] = %q, want %q", i, env[i], want)
		}
	}
	labels := service["labels"].([]interface{})
	wantLabel := "traefik.http.middlewares.stargate-auth.forwardauth.address=http://isolated-stargate:8080/_auth"
	if labels[0] != wantLabel {
		t.Errorf("forwardauth label = %q, want %q", labels[0], wantLabel)
	}
	if _, ok := service["depends_on"]; ok {
		t.Error("split Stargate service must not retain depends_on")
	}
}

func TestRewriteForwardAuthAddressSupportsLegacyPortlessForm(t *testing.T) {
	input := "forwardauth.address=http://stargate/_auth"
	got, ok := rewriteForwardAuthAddress(input, "custom-")
	if !ok || got != "forwardauth.address=http://custom-stargate/_auth" {
		t.Fatalf("rewriteForwardAuthAddress() = %q, %v", got, ok)
	}
}

// TestGenerateImageOrBuildStargateNoHeraldTotp 确保 image/build 模式下生成的 compose 中 stargate 不依赖 herald-totp，否则 docker compose config 会报错。
func TestGenerateImageOrBuildStargateNoHeraldTotp(t *testing.T) {
	full := map[string]interface{}{
		"services": map[string]interface{}{
			"herald": map[string]interface{}{
				"image": "herald:test",
				"depends_on": map[string]interface{}{
					"herald-redis": map[string]interface{}{"condition": "service_healthy"},
				},
			},
			"herald-redis": map[string]interface{}{"image": "redis:test"},
			"warden": map[string]interface{}{
				"image": "warden:test",
				"depends_on": map[string]interface{}{
					"warden-redis": map[string]interface{}{"condition": "service_healthy"},
				},
			},
			"warden-redis": map[string]interface{}{"image": "redis:test"},
			"stargate": map[string]interface{}{
				"image": "stargate:test",
				"environment": []interface{}{
					"HERALD_TOTP_ENABLED=false",
					"HERALD_TOTP_BASE_URL=http://herald-totp:8084",
					"HERALD_URL=http://herald:8082",
				},
				"depends_on": map[string]interface{}{
					"herald":      map[string]interface{}{"condition": "service_healthy"},
					"herald-totp": map[string]interface{}{"condition": "service_healthy"},
					"warden":      map[string]interface{}{"condition": "service_healthy"},
				},
			},
		},
		"volumes": map[string]interface{}{
			"herald-redis-data": nil,
			"warden-redis-data": nil,
		},
	}

	for _, mode := range []string{"image", "build"} {
		yml, err := generateImageOrBuild(full, mode, nil, nil)
		if err != nil {
			t.Fatalf("generateImageOrBuild(%q): %v", mode, err)
		}
		var out struct {
			Services map[string]struct {
				DependsOn interface{} `yaml:"depends_on"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal(yml, &out); err != nil {
			t.Fatalf("yaml unmarshal: %v", err)
		}
		stargate, ok := out.Services["stargate"]
		if !ok {
			t.Fatalf("mode %q: services.stargate missing", mode)
		}
		// depends_on 应为 map，且不应包含 herald-totp
		dep, ok := stargate.DependsOn.(map[string]interface{})
		if !ok {
			// 或为 list
			depList, _ := stargate.DependsOn.([]interface{})
			for _, v := range depList {
				if s, _ := v.(string); s == "herald-totp" {
					t.Errorf("mode %q: stargate.depends_on must not contain herald-totp (list form)", mode)
				}
			}
			continue
		}
		if _, has := dep["herald-totp"]; has {
			t.Errorf("mode %q: stargate.depends_on must not contain herald-totp, got %v", mode, dep)
		}
	}
}
