// Package main: /api/generate request types and scenario presets (Web UI only).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/the-gate/internal/composegen"
)

type scenarioPreset struct {
	Name          string                 `json:"name"`
	NameZh        string                 `json:"nameZh"`
	NameEn        string                 `json:"nameEn"`
	Description   string                 `json:"description"`
	DescriptionZh string                 `json:"descriptionZh"`
	DescriptionEn string                 `json:"descriptionEn"`
	RiskNote      string                 `json:"riskNote"`
	RiskNoteZh    string                 `json:"riskNoteZh"`
	RiskNoteEn    string                 `json:"riskNoteEn"`
	Modes         []string               `json:"modes"`
	EnvOverrides  map[string]string      `json:"envOverrides"`
	Options       map[string]interface{} `json:"options"` // 通用键值；通过 scenarioOptionSetters 映射到 composegen.Options
}

// scenarioOptionSetters 将 scenarios.json 的 option 键映射到 Options 字段；新增选项时在此表与 config 各加一项即可，无需改结构体。
var scenarioOptionSetters = map[string]func(*composegen.Options, interface{}){
	"includeDingTalk": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.IncludeDingTalk = b
		}
	},
	"includeSmtp": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.IncludeSmtp = b
		}
	},
	"useOwlmailForSmtp": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.UseOwlmailForSmtp = b
		}
	},
	"includeTotp": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.IncludeTotp = b
		}
	},
	"stargateSessionRedisUseBuiltin": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.StargateSessionRedisUseBuiltin = b
		}
	},
	"useNamedVolume": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.UseNamedVolume = b
		}
	},
	"disableWardenRedisService": func(opts *composegen.Options, v interface{}) {
		if b, ok := toBool(v); ok {
			opts.DisableWardenRedisService = b
		}
	},
}

func toBool(v interface{}) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case *bool:
		if x != nil {
			return *x, true
		}
		return false, false
	default:
		return false, false
	}
}

func loadScenarioPresets(root string) (map[string]scenarioPreset, error) {
	path := filepath.Join(root, "config", "scenarios.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenarios file: %w", err)
	}
	out := make(map[string]scenarioPreset)
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse scenarios file: %w", err)
	}
	return out, nil
}

func applyScenePresetToOptions(opts *composegen.Options, scene *scenarioPreset) {
	if opts == nil || scene == nil {
		return
	}
	if opts.EnvOverrides == nil {
		opts.EnvOverrides = make(map[string]string)
	}
	for k, v := range scene.EnvOverrides {
		opts.EnvOverrides[k] = v
	}
	for key, val := range scene.Options {
		if setter, ok := scenarioOptionSetters[key]; ok {
			setter(opts, val)
		}
	}
}

type generateRequest struct {
	Modes       []string               `json:"modes"`
	EnvOverride string                 `json:"envOverride"`
	Options     *composeGenOptionsJSON `json:"options"`
}

type composeGenOptionsJSON struct {
	HealthCheck                   *bool             `json:"healthCheck"`
	HealthCheckInterval           string            `json:"healthCheckInterval"`
	HealthCheckStartPeriod        string            `json:"healthCheckStartPeriod"`
	TraefikNetwork                *bool             `json:"traefikNetwork"`
	TraefikNetworkName            string            `json:"traefikNetworkName"`
	ExposePorts                   *bool             `json:"exposePorts"`
	PortHerald                    string            `json:"portHerald"`
	PortWarden                    string            `json:"portWarden"`
	PortHeraldRedis               string            `json:"portHeraldRedis"`
	PortHeraldTotp                string            `json:"portHeraldTotp"`
	PortHeraldSmtp                string            `json:"portHeraldSmtp"`
	PortOwlmail                   string            `json:"portOwlmail"`
	ContainerNamePrefix           string            `json:"containerNamePrefix"`
	DingtalkEnabled               *bool             `json:"dingtalkEnabled"`
	SmtpEnabled                   *bool             `json:"smtpEnabled"`
	SmtpUseOwlmail                *bool             `json:"smtpUseOwlmail"`
	TotpEnabled                   *bool             `json:"totpEnabled"`
	EnvOverrides                  map[string]string `json:"envOverrides"`
	UseNamedVolume                *bool             `json:"useNamedVolume"`
	HeraldRedisDataPath           string            `json:"heraldRedisDataPath"`
	WardenRedisDataPath           string            `json:"wardenRedisDataPath"`
	SessionStorageRedisUseBuiltin *bool             `json:"sessionStorageRedisUseBuiltin"`
	DisableWardenRedisService     *bool             `json:"disableWardenRedisService"`
}

// optionToComposeGenJSONSetters 将 session/API 的 option 键统一映射到 composeGenOptionsJSON；新增选项时在此表与 config 各加一项即可。
var optionToComposeGenJSONSetters = map[string]func(*composeGenOptionsJSON, interface{}){
	"healthCheck": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.HealthCheck = &b
		}
	},
	"traefikNetwork": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.TraefikNetwork = &b
		}
	},
	"exposePorts": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.ExposePorts = &b
		}
	},
	"dingtalkEnabled": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.DingtalkEnabled = &b
		}
	},
	"smtpEnabled": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.SmtpEnabled = &b
		}
	},
	"smtpUseOwlmail": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.SmtpUseOwlmail = &b
		}
	},
	"totpEnabled": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.TotpEnabled = &b
		}
	},
	"useNamedVolume": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.UseNamedVolume = &b
		}
	},
	"sessionStorageRedisUseBuiltin": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.SessionStorageRedisUseBuiltin = &b
		}
	},
	"disableWardenRedisService": func(o *composeGenOptionsJSON, v interface{}) {
		if b, ok := toBool(v); ok {
			o.DisableWardenRedisService = &b
		}
	},
	"traefikNetworkName":     func(o *composeGenOptionsJSON, v interface{}) { o.TraefikNetworkName = optStr(v) },
	"heraldRedisDataPath":    func(o *composeGenOptionsJSON, v interface{}) { o.HeraldRedisDataPath = optStr(v) },
	"wardenRedisDataPath":    func(o *composeGenOptionsJSON, v interface{}) { o.WardenRedisDataPath = optStr(v) },
	"portHerald":             func(o *composeGenOptionsJSON, v interface{}) { o.PortHerald = optStr(v) },
	"portWarden":             func(o *composeGenOptionsJSON, v interface{}) { o.PortWarden = optStr(v) },
	"containerNamePrefix":    func(o *composeGenOptionsJSON, v interface{}) { o.ContainerNamePrefix = optStr(v) },
	"healthCheckInterval":    func(o *composeGenOptionsJSON, v interface{}) { o.HealthCheckInterval = optStr(v) },
	"healthCheckStartPeriod": func(o *composeGenOptionsJSON, v interface{}) { o.HealthCheckStartPeriod = optStr(v) },
	"portHeraldRedis":        func(o *composeGenOptionsJSON, v interface{}) { o.PortHeraldRedis = optStr(v) },
	"portHeraldTotp":         func(o *composeGenOptionsJSON, v interface{}) { o.PortHeraldTotp = optStr(v) },
	"portHeraldSmtp":         func(o *composeGenOptionsJSON, v interface{}) { o.PortHeraldSmtp = optStr(v) },
	"portOwlmail":            func(o *composeGenOptionsJSON, v interface{}) { o.PortOwlmail = optStr(v) },
}

func optStr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%v", x))
	default:
		return ""
	}
}

// FillComposeGenOptionsFromMap 根据 option 键值映射填充 composeGenOptionsJSON，供 session 与 API 共用。
func FillComposeGenOptionsFromMap(o *composeGenOptionsJSON, m map[string]interface{}) {
	if o == nil || m == nil {
		return
	}
	for key, val := range m {
		if setter, ok := optionToComposeGenJSONSetters[key]; ok {
			setter(o, val)
		}
	}
}

func reqOptionsToComposegen(o *composeGenOptionsJSON) *composegen.Options {
	if o == nil {
		return nil
	}
	opts := &composegen.Options{
		TraefikNetworkName:  o.TraefikNetworkName,
		ContainerNamePrefix: o.ContainerNamePrefix,
		EnvOverrides:        o.EnvOverrides,
	}
	if o.HealthCheck != nil {
		opts.HealthCheck = *o.HealthCheck
	} else {
		opts.HealthCheck = true
	}
	opts.HealthCheckInterval = strings.TrimSpace(o.HealthCheckInterval)
	opts.HealthCheckStartPeriod = strings.TrimSpace(o.HealthCheckStartPeriod)
	if o.TraefikNetwork != nil {
		opts.TraefikNetwork = *o.TraefikNetwork
	} else {
		opts.TraefikNetwork = true
	}
	if o.ExposePorts != nil {
		opts.ExposePorts = *o.ExposePorts
	} else {
		opts.ExposePorts = true
	}
	opts.PortHerald = strings.TrimSpace(o.PortHerald)
	opts.PortWarden = strings.TrimSpace(o.PortWarden)
	opts.PortHeraldRedis = strings.TrimSpace(o.PortHeraldRedis)
	opts.PortHeraldTotp = strings.TrimSpace(o.PortHeraldTotp)
	opts.PortHeraldSmtp = strings.TrimSpace(o.PortHeraldSmtp)
	opts.PortOwlmail = strings.TrimSpace(o.PortOwlmail)
	if opts.TraefikNetworkName == "" {
		opts.TraefikNetworkName = "traefik"
	}
	if o.UseNamedVolume != nil {
		opts.UseNamedVolume = *o.UseNamedVolume
	} else {
		opts.UseNamedVolume = true
	}
	opts.HeraldRedisDataPath = strings.TrimSpace(o.HeraldRedisDataPath)
	opts.WardenRedisDataPath = strings.TrimSpace(o.WardenRedisDataPath)
	if opts.HeraldRedisDataPath == "" {
		opts.HeraldRedisDataPath = "./data/herald-redis"
	}
	if opts.WardenRedisDataPath == "" {
		opts.WardenRedisDataPath = "./data/warden-redis"
	}
	if o.SessionStorageRedisUseBuiltin != nil {
		opts.StargateSessionRedisUseBuiltin = *o.SessionStorageRedisUseBuiltin
	} else {
		opts.StargateSessionRedisUseBuiltin = false
	}
	if o.DisableWardenRedisService != nil {
		opts.DisableWardenRedisService = *o.DisableWardenRedisService
	} else {
		opts.DisableWardenRedisService = false
	}
	if o.DingtalkEnabled != nil {
		opts.IncludeDingTalk = *o.DingtalkEnabled
	} else {
		opts.IncludeDingTalk = false
	}
	if o.SmtpEnabled != nil {
		opts.IncludeSmtp = *o.SmtpEnabled
	} else {
		opts.IncludeSmtp = false
	}
	if o.SmtpUseOwlmail != nil {
		opts.UseOwlmailForSmtp = *o.SmtpUseOwlmail
	} else {
		opts.UseOwlmailForSmtp = false
	}
	if o.TotpEnabled != nil {
		opts.IncludeTotp = *o.TotpEnabled
	} else {
		opts.IncludeTotp = false
	}
	if opts.EnvOverrides == nil {
		opts.EnvOverrides = make(map[string]string)
	}
	if o.TotpEnabled != nil {
		if *o.TotpEnabled {
			opts.EnvOverrides["HERALD_TOTP_ENABLED"] = "true"
		} else {
			opts.EnvOverrides["HERALD_TOTP_ENABLED"] = "false"
		}
	}
	return opts
}
