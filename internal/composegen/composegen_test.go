package composegen

import (
	"gopkg.in/yaml.v3"
	"testing"
)

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
					"herald":     map[string]interface{}{"condition": "service_healthy"},
					"herald-totp": map[string]interface{}{"condition": "service_healthy"},
					"warden":     map[string]interface{}{"condition": "service_healthy"},
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
