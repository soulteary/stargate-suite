// Package composegen: validation for Options and EnvOverrides before generate.
package composegen

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ValidateOptions 校验 Options 中端口范围（1-65535）及可选 URL 格式；返回首个错误。
func ValidateOptions(opts *Options) error {
	if opts == nil {
		return nil
	}
	portFields := []struct {
		name  string
		value string
	}{
		{"portHerald", opts.PortHerald},
		{"portWarden", opts.PortWarden},
		{"portHeraldRedis", opts.PortHeraldRedis},
		{"portHeraldTotp", opts.PortHeraldTotp},
		{"portHeraldSmtp", opts.PortHeraldSmtp},
		{"portOwlmail", opts.PortOwlmail},
	}
	for _, f := range portFields {
		v := strings.TrimSpace(f.value)
		if v == "" {
			continue
		}
		// 允许 "8082" 或 ":8082" 形式
		numStr := v
		if strings.HasPrefix(numStr, ":") {
			numStr = numStr[1:]
		}
		if idx := strings.Index(numStr, ":"); idx >= 0 {
			numStr = numStr[:idx]
		}
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return fmt.Errorf("%s: invalid port %q", f.name, f.value)
		}
		if num < 1 || num > 65535 {
			return fmt.Errorf("%s: port %d out of range (1-65535)", f.name, num)
		}
	}
	return nil
}

// ValidateEnvOverrides 校验 EnvOverrides 中 URL 类值的格式（可选）；allowed 为 nil 时不校验白名单。
func ValidateEnvOverrides(overrides map[string]string, allowed map[string]map[string]bool) []string {
	var errs []string
	urlKeys := map[string]bool{
		"WARDEN_URL": true, "HERALD_URL": true, "HERALD_TOTP_BASE_URL": true,
		"HERALD_DINGTALK_API_URL": true, "HERALD_SMTP_API_URL": true,
		"WARDEN_REMOTE_CONFIG": true, "OTLP_ENDPOINT": true,
	}
	for k, v := range overrides {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if urlKeys[k] && looksLikeURL(v) {
			if _, err := url.ParseRequestURI(v); err != nil {
				errs = append(errs, fmt.Sprintf("env %s: invalid URL %q", k, v))
			}
		}
		if allowed != nil {
			found := false
			for _, m := range allowed {
				if m[k] {
					found = true
					break
				}
			}
			if !found {
				// 仅 warning，不阻断
				errs = append(errs, fmt.Sprintf("env %q not in any service allowlist (may be ignored in compose)", k))
			}
		}
	}
	return errs
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
