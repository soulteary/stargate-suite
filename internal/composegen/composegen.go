// Package composegen 从单一 compose 源解析并生成多份 compose（traefik 全量/三分开）及 .env 模板。
package composegen

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Options 控制生成时的健康检查、Traefik 网络、端口暴露、容器名、环境变量及 Redis 数据存储方式等。
// nil 表示全部使用默认（健康检查开、Traefik 开、暴露端口开、前缀 the-gate-、命名卷、无 env 覆盖）。
type Options struct {
	HealthCheck            bool   // 是否保留各服务的 healthcheck
	HealthCheckInterval    string // 健康检查间隔，如 "10s"；空表示不覆盖
	HealthCheckStartPeriod string // 健康检查启动延迟，如 "10s"；空表示不覆盖
	TraefikNetwork         bool   // 是否加入 Traefik 网络及相关 labels
	TraefikNetworkName     string // Traefik 网络名称，默认 "traefik"
	ExposePorts            bool   // true 保留 ports:，false 改为仅 expose
	// 暴露端口时可选的主机端口，空表示使用 compose 默认
	PortHerald          string            // Herald 主机端口，如 "8082"
	PortWarden          string            // Warden 主机端口，如 "8081"
	PortHeraldRedis     string            // Herald Redis 主机端口，如 "6379"
	ContainerNamePrefix string            // 容器名前缀，如 "the-gate-"
	EnvOverrides        map[string]string // 环境变量覆盖，合并进各服务 environment
	// Redis 数据：true 使用 Docker 命名卷，false 使用主机绑定路径
	UseNamedVolume      bool   // 为 true 时保持命名卷；为 false 时使用 HeraldRedisDataPath / WardenRedisDataPath
	HeraldRedisDataPath string // 绑定路径时 Herald Redis 数据目录，默认 ./data/herald-redis
	WardenRedisDataPath string // 绑定路径时 Warden Redis 数据目录，默认 ./data/warden-redis
}

// serviceNameToContainerSuffix 逻辑服务名 -> container_name 后缀（前缀由 Options 提供）
var serviceNameToContainerSuffix = map[string]string{
	"herald": "herald", "herald-redis": "herald-redis", "herald-dingtalk": "herald-dingtalk",
	"warden": "warden", "warden-redis": "warden-redis",
	"stargate": "stargate", "protected-service": "whoami",
}

// splitDef 定义从完整 compose 中切出的服务、卷及是否应用 stargate 覆盖。
type splitDef struct {
	name              string
	services          []string
	volumes           []string
	stargateOverrides bool
}

var traefikSplitDefs = []splitDef{
	{"traefik", nil, nil, false}, // 全量，services/volumes 为 nil 表示全部保留
	{"traefik-herald", []string{"herald", "herald-redis"}, []string{"herald-redis-data"}, false},
	{"traefik-warden", []string{"warden", "warden-redis"}, []string{"warden-redis-data"}, false},
	{"traefik-stargate", []string{"stargate", "protected-service"}, nil, true},
}

// envComments 环境变量名 -> 注释（用于在生成的 docker-compose 中插入注释，便于用户查看和修改）
var envComments = map[string]string{
	"PORT":                            "服务监听端口",
	"REDIS_ADDR":                      "Herald Redis 地址 (host:port)，可通过 HERALD_REDIS_ADDR 覆盖",
	"REDIS_PASSWORD":                  "Redis 密码，留空表示无认证",
	"REDIS_DB":                        "Herald Redis 库号",
	"LOG_LEVEL":                       "日志级别 (info/debug/warn/error)",
	"API_KEY":                         "服务间 API 密钥，生产请修改",
	"HMAC_SECRET":                     "Herald HMAC 签名密钥，生产请修改",
	"HERALD_TEST_MODE":                "Herald 测试模式（免真实发送验证码）",
	"PROVIDER_FAILURE_POLICY":         "Provider 失败策略 (soft/strict)",
	"CHALLENGE_EXPIRY":                "验证码有效期",
	"CODE_LENGTH":                     "验证码长度",
	"MAX_ATTEMPTS":                    "单 challenge 最大验证次数",
	"RESEND_COOLDOWN":                 "重发冷却时间",
	"IDEMPOTENCY_KEY_TTL":             "Herald 幂等键 TTL（0 表示使用 CHALLENGE_EXPIRY）",
	"ALLOWED_PURPOSES":                "Herald 允许的 purpose 列表，逗号分隔，如 login,reset,bind,stepup",
	"SERVICE_NAME":                    "Herald 服务标识（HMAC 等）",
	"HERALD_HMAC_KEYS":                "Herald 多密钥 HMAC JSON，如 {\"key-id\":\"secret\"}，可选",
	"REDIS":                           "Warden Redis 地址 (host:port)，可通过 WARDEN_REDIS_ADDR 覆盖",
	"REDIS_PASSWORD_FILE":             "Warden Redis 密码文件路径（可选，优先于 REDIS_PASSWORD）",
	"REDIS_ENABLED":                   "Warden 是否启用 Redis（可选，默认 true）",
	"DATA_FILE":                       "Warden 本地用户数据文件路径（容器内路径）",
	"MODE":                            "Warden 模式 (ONLY_LOCAL/REMOTE/HYBRID 等)",
	"INTERVAL":                        "Warden 轮询间隔（秒）",
	"CONFIG":                          "Warden 远程配置 URL（REMOTE 等模式）",
	"KEY":                             "Warden 远程配置认证 Header（如 Bearer token）",
	"HTTP_MAX_IDLE_CONNS":             "Warden HTTP 最大空闲连接数",
	"HTTP_INSECURE_TLS":               "Warden 是否跳过 TLS 校验（仅开发）",
	"AUTH_HOST":                       "认证页 Host / 域名",
	"LOGIN_PAGE_TITLE":                "登录页标题",
	"LOGIN_PAGE_FOOTER_TEXT":          "登录页页脚文案",
	"COOKIE_DOMAIN":                   "Cookie 域名（多子域时设置）",
	"PASSWORDS":                       "登录密码配置，生产请修改",
	"LANGUAGE":                        "界面语言",
	"WARDEN_URL":                      "Stargate 调用 Warden 的地址",
	"WARDEN_ENABLED":                  "是否启用 Warden",
	"WARDEN_API_KEY":                  "Warden API 密钥",
	"WARDEN_CACHE_TTL":                "Warden 缓存 TTL（秒）",
	"HERALD_URL":                      "Stargate 调用 Herald 的地址",
	"HERALD_ENABLED":                  "是否启用 Herald",
	"HERALD_API_KEY":                  "Herald API 密钥",
	"HERALD_HMAC_SECRET":              "Herald HMAC 密钥",
	"SESSION_STORAGE_ENABLED":         "是否启用会话存储",
	"SESSION_STORAGE_REDIS_ADDR":      "会话存储 Redis 地址",
	"SESSION_STORAGE_REDIS_PASSWORD":  "会话存储 Redis 密码",
	"AUDIT_LOG_ENABLED":               "是否启用审计日志",
	"AUDIT_LOG_FORMAT":                "审计日志格式 (json/text)",
	"WARDEN_REDIS_PASSWORD":           "Warden Redis 密码",
	"WARDEN_HTTP_TIMEOUT":             "Warden HTTP 请求超时（秒）",
	"LOCKOUT_DURATION":                "Herald 锁定时长（超过最大尝试次数后）",
	"RATE_LIMIT_PER_USER":             "Herald 每用户/小时限流",
	"RATE_LIMIT_PER_IP":               "Herald 每 IP/分钟限流",
	"RATE_LIMIT_PER_DESTINATION":      "Herald 每目标/小时限流",
	"HERALD_DINGTALK_API_URL":         "Herald 钉钉通道：herald-dingtalk 服务地址（可选）",
	"HERALD_DINGTALK_API_KEY":         "Herald 钉钉通道：herald-dingtalk API 密钥（可选）",
	"HERALD_DINGTALK_IMAGE":           "herald-dingtalk 服务镜像（可选）",
	"DINGTALK_APP_KEY":                "herald-dingtalk：钉钉应用 Key",
	"DINGTALK_APP_SECRET":             "herald-dingtalk：钉钉应用 Secret",
	"DINGTALK_AGENT_ID":               "herald-dingtalk：钉钉应用 AgentId（工作通知）",
	"HERALD_DINGTALK_IDEMPOTENCY_TTL": "herald-dingtalk 幂等缓存 TTL（秒）",
	"DEBUG":                           "调试模式",
}

// LoadCompose 读取并解析 compose 文件为 map。
func LoadCompose(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compose: %w", err)
	}
	var out map[string]interface{}
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse compose: %w", err)
	}
	return out, nil
}

// envVarRegex 匹配 ${VAR:-default} 或 ${VAR}
var envVarRegex = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// envLineRegex 匹配 environment 列表项 "- KEY=VALUE" 或 "- KEY=${VAR:-default}"
var envLineRegex = regexp.MustCompile(`^(\s+)-\s+([^=]+)=`)

// injectEnvComments 在生成的 YAML 中为 environment 列表项插入注释行（便于用户查看和修改）。
func injectEnvComments(yml []byte, comments map[string]string) []byte {
	if len(comments) == 0 {
		return yml
	}
	lines := strings.Split(string(yml), "\n")
	var out []string
	inEnv := false
	baseIndent := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "environment:" {
			inEnv = true
			baseIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			out = append(out, line)
			continue
		}
		lead := len(line) - len(strings.TrimLeft(line, " \t"))
		// 离开 environment 块：缩进不大于 baseIndent，且不是空行/注释，且不是 env 列表项（- KEY=VAL）
		if inEnv && lead <= baseIndent && trimmed != "" && !strings.HasPrefix(trimmed, "#") && envLineRegex.FindStringSubmatch(line) == nil {
			inEnv = false
		}
		if inEnv {
			if m := envLineRegex.FindStringSubmatch(line); len(m) >= 3 {
				key := strings.TrimSpace(m[2])
				if c, ok := comments[key]; ok && c != "" {
					indent := m[1]
					out = append(out, indent+"# "+c)
				}
			}
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

// ExtractEnvVars 从 compose map 中扫描 image、environment、labels 等中的 ${VAR:-default}，返回变量名到默认值的映射。
func ExtractEnvVars(compose map[string]interface{}) map[string]string {
	vars := make(map[string]string)
	scanValue := func(s string) {
		for _, m := range envVarRegex.FindAllStringSubmatch(s, -1) {
			name := strings.TrimSpace(m[1])
			if name == "" {
				continue
			}
			defaultVal := ""
			if len(m) > 2 {
				defaultVal = m[2]
			}
			if _, ok := vars[name]; !ok {
				vars[name] = defaultVal
			}
		}
	}
	services, _ := compose["services"].(map[string]interface{})
	if services == nil {
		return vars
	}
	for _, s := range services {
		svc, _ := s.(map[string]interface{})
		if svc == nil {
			continue
		}
		if v, ok := svc["image"]; ok {
			if s, ok := v.(string); ok {
				scanValue(s)
			}
		}
		if env, ok := svc["environment"]; ok {
			switch e := env.(type) {
			case []interface{}:
				for _, item := range e {
					if s, ok := item.(string); ok {
						scanValue(s)
					}
				}
			case map[string]interface{}:
				for k, val := range e {
					scanValue(k)
					if s, ok := val.(string); ok {
						scanValue(s)
					}
				}
			}
		}
		if labels, ok := svc["labels"]; ok {
			switch l := labels.(type) {
			case []interface{}:
				for _, item := range l {
					if s, ok := item.(string); ok {
						scanValue(s)
					}
				}
			case map[string]interface{}:
				for k, val := range l {
					scanValue(k)
					if s, ok := val.(string); ok {
						scanValue(s)
					}
				}
			}
		}
	}
	return vars
}

// EnvBodyFromVars 根据变量映射生成 .env 文件内容；optionalOverride 可覆盖或追加（每行 KEY=VALUE）。
func EnvBodyFromVars(vars map[string]string, optionalOverride string) string {
	// 常用顺序（镜像与域名优先，再按服务分组；Herald 当前使用 REDIS_ADDR，HERALD_REDIS_URL 为规范建议，待 Herald 支持后可启用）
	order := []string{
		"HERALD_IMAGE", "WARDEN_IMAGE", "STARGATE_IMAGE",
		"HERALD_REDIS_IMAGE", "WARDEN_REDIS_IMAGE",
		"HERALD_REDIS_ADDR", "HERALD_REDIS_PASSWORD", "HERALD_REDIS_DB",
		"WARDEN_REDIS_ADDR", "WARDEN_REDIS_PASSWORD", "WARDEN_REDIS_PASSWORD_FILE", "WARDEN_REDIS_ENABLED", "WARDEN_DATA_FILE",
		"HERALD_REDIS_DATA_PATH", "WARDEN_REDIS_DATA_PATH",
		"PROTECTED_IMAGE",
		"AUTH_HOST", "STARGATE_DOMAIN", "PROTECTED_DOMAIN",
		"STARGATE_PREFIX", "PROTECTED_PREFIX", "USER_HEADER_NAME",
		"LOGIN_PAGE_TITLE", "LOGIN_PAGE_FOOTER_TEXT", "COOKIE_DOMAIN",
		"LANGUAGE", "PASSWORDS",
		"HERALD_API_KEY", "HERALD_HMAC_SECRET", "WARDEN_API_KEY",
		"WARDEN_ENABLED", "HERALD_ENABLED", "SESSION_STORAGE_ENABLED",
		"SESSION_STORAGE_REDIS_ADDR", "SESSION_STORAGE_REDIS_PASSWORD",
		"WARDEN_CACHE_TTL", "AUDIT_LOG_ENABLED", "AUDIT_LOG_FORMAT", "DEBUG",
		"MODE", "LOG_LEVEL", "INTERVAL", "WARDEN_REMOTE_CONFIG", "WARDEN_REMOTE_KEY",
		"WARDEN_HTTP_TIMEOUT", "WARDEN_HTTP_MAX_IDLE_CONNS", "WARDEN_HTTP_INSECURE_TLS",
		"HERALD_TEST_MODE", "CHALLENGE_EXPIRY", "CODE_LENGTH", "MAX_ATTEMPTS",
		"PROVIDER_FAILURE_POLICY", "RESEND_COOLDOWN", "LOCKOUT_DURATION",
		"IDEMPOTENCY_KEY_TTL", "ALLOWED_PURPOSES", "SERVICE_NAME", "HERALD_HMAC_KEYS",
		"RATE_LIMIT_PER_USER", "RATE_LIMIT_PER_IP", "RATE_LIMIT_PER_DESTINATION",
		"HERALD_DINGTALK_IMAGE", "HERALD_DINGTALK_API_URL", "HERALD_DINGTALK_API_KEY",
		"DINGTALK_APP_KEY", "DINGTALK_APP_SECRET", "DINGTALK_AGENT_ID", "HERALD_DINGTALK_IDEMPOTENCY_TTL",
	}
	seen := make(map[string]bool)
	var lines []string
	lines = append(lines, "# Container Image / Env - generated from compose")
	lines = append(lines, "")
	redisCommentAdded := false
	dingtalkCommentAdded := false
	for _, k := range order {
		if v, ok := vars[k]; ok {
			seen[k] = true
			if (k == "HERALD_REDIS_ADDR" || k == "WARDEN_REDIS_ADDR") && !redisCommentAdded {
				lines = append(lines, "# Redis connection (override for external Redis)")
				redisCommentAdded = true
			}
			if k == "HERALD_DINGTALK_IMAGE" && !dingtalkCommentAdded {
				lines = append(lines, "# DingTalk channel (optional): Herald calls herald-dingtalk via HTTP")
				dingtalkCommentAdded = true
			}
			lines = append(lines, k+"="+v)
		}
	}
	for k, v := range vars {
		if !seen[k] {
			lines = append(lines, k+"="+v)
		}
	}
	if optionalOverride != "" {
		lines = append(lines, "")
		lines = append(lines, strings.TrimSpace(optionalOverride))
	}
	return strings.Join(lines, "\n") + "\n"
}

// DefaultEnvBody 返回与现有 defaultEnvBody 一致的默认 .env 内容（当未从 compose 推断时使用）。
func DefaultEnvBody() string {
	return `# Container Image Version Configuration

# Herald Service Image
HERALD_IMAGE=ghcr.io/soulteary/herald:v0.5.0

# Warden Service Image
WARDEN_IMAGE=ghcr.io/soulteary/warden:v0.9.3

# Stargate Service Image
STARGATE_IMAGE=ghcr.io/soulteary/stargate:v0.8.4

# Redis Image Version
HERALD_REDIS_IMAGE=redis:8.4-alpine
WARDEN_REDIS_IMAGE=redis:8.4-alpine

# Herald Redis connection (Herald uses REDIS_ADDR; HERALD_REDIS_URL is spec suggestion, not yet used)
HERALD_REDIS_ADDR=herald-redis:6379
HERALD_REDIS_PASSWORD=
HERALD_REDIS_DB=0

# Warden Redis connection
WARDEN_REDIS_ADDR=warden-redis:6379
WARDEN_REDIS_PASSWORD=
# WARDEN_REDIS_PASSWORD_FILE=
# WARDEN_REDIS_ENABLED=true

# Warden remote config (when MODE is REMOTE / HYBRID etc.)
# WARDEN_REMOTE_CONFIG=http://example.com/data.json
# WARDEN_REMOTE_KEY=

# Warden HTTP client (optional)
# WARDEN_HTTP_MAX_IDLE_CONNS=100
# WARDEN_HTTP_INSECURE_TLS=false

# Redis data path (only used when UseNamedVolume=false / bind path)
# HERALD_REDIS_DATA_PATH=./data/herald-redis
# WARDEN_REDIS_DATA_PATH=./data/warden-redis

# Herald optional: idempotency TTL (0=use challenge expiry), allowed purposes, HMAC keys (JSON), service name
# IDEMPOTENCY_KEY_TTL=0
# ALLOWED_PURPOSES=login
# SERVICE_NAME=herald
# HERALD_HMAC_KEYS=

# DingTalk channel (optional): Herald calls herald-dingtalk via HTTP for verification code push
# HERALD_DINGTALK_IMAGE=ghcr.io/soulteary/herald-dingtalk:latest
# HERALD_DINGTALK_API_URL=http://herald-dingtalk:8083
# HERALD_DINGTALK_API_KEY=
# DINGTALK_APP_KEY=
# DINGTALK_APP_SECRET=
# DINGTALK_AGENT_ID=
`
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copyServices(svcs map[string]interface{}) map[string]interface{} {
	if svcs == nil {
		return nil
	}
	out := make(map[string]interface{}, len(svcs))
	for k, v := range svcs {
		if m, ok := v.(map[string]interface{}); ok {
			out[k] = copyMap(m)
		} else {
			out[k] = v
		}
	}
	return out
}

func splitComposeComment(name string) string {
	switch name {
	case "traefik":
		return `# Stargate Suite with Traefik - 三合一（由 canonical 生成）
# 使用：docker compose -f build/traefik/docker-compose.yml up -d
#
`
	case "traefik-herald":
		return `# Herald 独立 compose - 仅 OTP/验证码服务及其 Redis（由 canonical 生成）
# 使用前先创建共享网络：docker network create the-gate-network
# 启动：docker compose -f build/traefik-herald/docker-compose.yml up -d
#
`
	case "traefik-warden":
		return `# Warden 独立 compose - 仅白名单用户服务及其 Redis（由 canonical 生成）
# 使用前先创建共享网络：docker network create the-gate-network
# 启动：docker compose -f build/traefik-warden/docker-compose.yml up -d
#
`
	case "traefik-stargate":
		return `# Stargate 独立 compose - 仅 Forward Auth 与示例受保护服务（由 canonical 生成）
# 依赖：Herald、Warden 已用独立 compose 启动，且与 Stargate 同属 the-gate-network。
# 启动：docker compose -f build/traefik-stargate/docker-compose.yml up -d
#
`
	default:
		return ""
	}
}

// applyOptions 对单个服务应用 Options（健康检查、端口、容器名、环境变量）。
func applyOptions(svc map[string]interface{}, serviceName string, opts *Options) {
	if opts == nil {
		return
	}
	if !opts.HealthCheck {
		delete(svc, "healthcheck")
	} else if opts.HealthCheckInterval != "" || opts.HealthCheckStartPeriod != "" {
		// 覆盖健康检查间隔与启动延迟。保留两分支以兼容不同 unmarshal 结果（yaml.v3 通常为 map[interface{}]interface{}）。
		if hc, ok := svc["healthcheck"].(map[string]interface{}); ok {
			if opts.HealthCheckInterval != "" {
				hc["interval"] = opts.HealthCheckInterval
			}
			if opts.HealthCheckStartPeriod != "" {
				hc["start_period"] = opts.HealthCheckStartPeriod
			}
		}
		if hc, ok := svc["healthcheck"].(map[interface{}]interface{}); ok {
			if opts.HealthCheckInterval != "" {
				hc["interval"] = opts.HealthCheckInterval
			}
			if opts.HealthCheckStartPeriod != "" {
				hc["start_period"] = opts.HealthCheckStartPeriod
			}
		}
	}
	if !opts.ExposePorts {
		if ports, ok := svc["ports"]; ok {
			switch p := ports.(type) {
			case []interface{}:
				if len(p) > 0 {
					var expose []interface{}
					for _, item := range p {
						s, _ := item.(string)
						// "host:container" -> container port; "port" -> port
						if idx := strings.Index(s, ":"); idx >= 0 {
							s = s[idx+1:]
						}
						if s != "" {
							expose = append(expose, s)
						}
					}
					if len(expose) > 0 {
						delete(svc, "ports")
						svc["expose"] = expose
					}
				}
			}
		}
	} else {
		// 暴露端口时，可选覆盖主机端口
		if ports, ok := svc["ports"].([]interface{}); ok && len(ports) > 0 {
			var hostPort string
			switch serviceName {
			case "herald":
				hostPort = strings.TrimSpace(opts.PortHerald)
				if hostPort != "" {
					ports[0] = hostPort + ":8082"
				}
			case "warden":
				hostPort = strings.TrimSpace(opts.PortWarden)
				if hostPort != "" {
					ports[0] = hostPort + ":8081"
				}
			case "herald-redis":
				hostPort = strings.TrimSpace(opts.PortHeraldRedis)
				if hostPort != "" {
					ports[0] = hostPort + ":6379"
				}
			}
		}
	}
	if opts.ContainerNamePrefix != "" {
		if suffix, ok := serviceNameToContainerSuffix[serviceName]; ok {
			svc["container_name"] = opts.ContainerNamePrefix + suffix
		}
		if serviceName == "stargate" {
			prefix := opts.ContainerNamePrefix
			if env, ok := svc["environment"].([]interface{}); ok {
				for i, e := range env {
					s, _ := e.(string)
					if strings.HasPrefix(s, "WARDEN_URL=") {
						env[i] = "WARDEN_URL=http://" + prefix + "warden:8081"
					}
					if strings.HasPrefix(s, "HERALD_URL=") {
						env[i] = "HERALD_URL=http://" + prefix + "herald:8082"
					}
				}
			}
			if labels, ok := svc["labels"].([]interface{}); ok {
				for i, l := range labels {
					s, _ := l.(string)
					if strings.Contains(s, "forwardauth.address=http://stargate/_auth") {
						labels[i] = strings.Replace(s, "http://stargate/_auth", "http://"+prefix+"stargate/_auth", 1)
					}
				}
			}
		}
	}
	if len(opts.EnvOverrides) > 0 {
		envList, _ := svc["environment"].([]interface{})
		overrides := opts.EnvOverrides
		used := make(map[string]bool)
		var newList []interface{}
		for _, e := range envList {
			s, _ := e.(string)
			if idx := strings.Index(s, "="); idx >= 0 {
				key := strings.TrimSpace(s[:idx])
				if v, ok := overrides[key]; ok {
					newList = append(newList, key+"="+v)
					used[key] = true
				} else {
					newList = append(newList, s)
				}
			} else {
				newList = append(newList, e)
			}
		}
		for k, v := range overrides {
			if !used[k] {
				newList = append(newList, k+"="+v)
			}
		}
		svc["environment"] = newList
	}
}

// applyOptionsToCompose 对整份 compose（out）应用 Options：每个服务 applyOptions，并处理 Traefik 网络。
func applyOptionsToCompose(out map[string]interface{}, opts *Options) {
	if opts == nil {
		return
	}
	services, _ := out["services"].(map[string]interface{})
	for name, s := range services {
		svc, _ := s.(map[string]interface{})
		if svc != nil {
			applyOptions(svc, name, opts)
		}
	}
	networks, _ := out["networks"].(map[string]interface{})
	if networks == nil {
		return
	}
	traefikName := "traefik"
	if opts.TraefikNetworkName != "" {
		traefikName = opts.TraefikNetworkName
	}
	if !opts.TraefikNetwork {
		delete(networks, "traefik")
		delete(networks, traefikName)
		if services != nil {
			for _, name := range []string{"stargate", "protected-service"} {
				if s, ok := services[name]; ok {
					svc, _ := s.(map[string]interface{})
					if svc == nil {
						continue
					}
					if n, ok := svc["networks"]; ok {
						switch nlist := n.(type) {
						case []interface{}:
							var kept []interface{}
							for _, v := range nlist {
								s, _ := v.(string)
								if s != "traefik" && s != traefikName {
									kept = append(kept, v)
								}
							}
							svc["networks"] = kept
						}
					}
					if name == "stargate" || name == "protected-service" {
						if labels, ok := svc["labels"].([]interface{}); ok {
							var kept []interface{}
							for _, l := range labels {
								s, _ := l.(string)
								if !strings.HasPrefix(s, "traefik.") {
									kept = append(kept, l)
								}
							}
							svc["labels"] = kept
						}
					}
				}
			}
		}
	} else if traefikName != "traefik" {
		if v, ok := networks["traefik"]; ok {
			delete(networks, "traefik")
			networks[traefikName] = v
		}
		if services != nil {
			for _, name := range []string{"stargate", "protected-service"} {
				if s, ok := services[name]; ok {
					svc, _ := s.(map[string]interface{})
					if svc == nil {
						continue
					}
					if n, ok := svc["networks"].([]interface{}); ok {
						for i, v := range n {
							if s, _ := v.(string); s == "traefik" {
								n[i] = traefikName
							}
						}
					}
					if labels, ok := svc["labels"].([]interface{}); ok {
						for i, l := range labels {
							s, _ := l.(string)
							if strings.Contains(s, "traefik.docker.network=traefik") {
								labels[i] = strings.Replace(s, "traefik.docker.network=traefik", "traefik.docker.network="+traefikName, 1)
							}
						}
					}
				}
			}
		}
	}
}

func applyStargateSplitOverrides(svc map[string]interface{}, containerNamePrefix string) {
	prefix := containerNamePrefix
	if prefix == "" {
		prefix = "the-gate-"
	}
	delete(svc, "depends_on")
	if env, ok := svc["environment"].([]interface{}); ok {
		for i, e := range env {
			s, _ := e.(string)
			if s == "WARDEN_URL=http://warden:8081" {
				env[i] = "WARDEN_URL=http://" + prefix + "warden:8081"
			}
			if s == "HERALD_URL=http://herald:8082" {
				env[i] = "HERALD_URL=http://" + prefix + "herald:8082"
			}
		}
	}
	if labels, ok := svc["labels"].([]interface{}); ok {
		for i, l := range labels {
			s, _ := l.(string)
			if s == "traefik.http.middlewares.stargate-auth.forwardauth.address=http://stargate/_auth" {
				labels[i] = "traefik.http.middlewares.stargate-auth.forwardauth.address=http://" + prefix + "stargate/_auth"
			}
		}
	}
}

// GenerateOne 根据 mode 从完整 compose 生成一份 compose YAML；mode 为 traefik | traefik-herald | traefik-warden | traefik-stargate。opts 为 nil 时使用默认行为。
func GenerateOne(full map[string]interface{}, mode string, opts *Options) ([]byte, error) {
	services, _ := full["services"].(map[string]interface{})
	if services == nil {
		return nil, fmt.Errorf("compose missing services")
	}
	volumes, _ := full["volumes"].(map[string]interface{})
	prefix := "the-gate-"
	if opts != nil && opts.ContainerNamePrefix != "" {
		prefix = opts.ContainerNamePrefix
	}

	var def *splitDef
	for i := range traefikSplitDefs {
		if traefikSplitDefs[i].name == mode {
			def = &traefikSplitDefs[i]
			break
		}
	}
	if def == nil {
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}

	out := make(map[string]interface{})
	if def.services == nil {
		// 全量 traefik；若需应用 opts 则复制避免污染 full
		if opts != nil {
			out["services"] = copyServices(services)
			out["volumes"] = full["volumes"]
			if n, _ := full["networks"].(map[string]interface{}); n != nil {
				out["networks"] = copyMap(n)
			} else {
				out["networks"] = full["networks"]
			}
		} else {
			out["services"] = full["services"]
			out["volumes"] = full["volumes"]
			out["networks"] = full["networks"]
		}
	} else {
		outSvcs := make(map[string]interface{})
		for _, name := range def.services {
			if svc, ok := services[name]; ok {
				svcMap, _ := svc.(map[string]interface{})
				if svcMap == nil {
					outSvcs[name] = svc
					continue
				}
				clone := copyMap(svcMap)
				if def.stargateOverrides && name == "stargate" {
					applyStargateSplitOverrides(clone, prefix)
				}
				outSvcs[name] = clone
			}
		}
		out["services"] = outSvcs
		if len(def.volumes) > 0 && volumes != nil {
			outVol := make(map[string]interface{})
			for _, vn := range def.volumes {
				if v, ok := volumes[vn]; ok {
					outVol[vn] = v
				}
			}
			out["volumes"] = outVol
		}
		outNet := make(map[string]interface{})
		if def.stargateOverrides {
			outNet["the-gate-network"] = map[string]interface{}{"external": true}
			outNet["traefik"] = map[string]interface{}{"external": true}
		} else {
			outNet["the-gate-network"] = map[string]interface{}{"external": true}
		}
		out["networks"] = outNet
	}

	applyOptionsToCompose(out, opts)

	// Redis 数据：命名卷 vs 绑定路径
	if opts != nil && !opts.UseNamedVolume {
		applyRedisBindPaths(out, opts)
	}

	outData, err := yaml.Marshal(out)
	if err != nil {
		return nil, err
	}
	outData = injectEnvComments(outData, envComments)
	header := splitComposeComment(mode)
	return append([]byte(header), outData...), nil
}

// applyRedisBindPaths 将 herald-redis / warden-redis 的命名卷改为绑定路径，并从顶层 volumes 中移除对应命名卷。
func applyRedisBindPaths(out map[string]interface{}, opts *Options) {
	defaultHerald := "./data/herald-redis"
	if opts.HeraldRedisDataPath != "" {
		defaultHerald = opts.HeraldRedisDataPath
	}
	defaultWarden := "./data/warden-redis"
	if opts.WardenRedisDataPath != "" {
		defaultWarden = opts.WardenRedisDataPath
	}
	services, _ := out["services"].(map[string]interface{})
	if services != nil {
		if svc, ok := services["herald-redis"].(map[string]interface{}); ok {
			svc["volumes"] = []interface{}{"${HERALD_REDIS_DATA_PATH:-" + defaultHerald + "}:/data"}
		}
		if svc, ok := services["warden-redis"].(map[string]interface{}); ok {
			svc["volumes"] = []interface{}{"${WARDEN_REDIS_DATA_PATH:-" + defaultWarden + "}:/data"}
		}
	}
	volumes, _ := out["volumes"].(map[string]interface{})
	if volumes != nil {
		delete(volumes, "herald-redis-data")
		delete(volumes, "warden-redis-data")
		if len(volumes) == 0 {
			delete(out, "volumes")
		}
	}
}

// Generated 表示单次生成结果：多份 compose 与一份 .env。
type Generated struct {
	Composes map[string][]byte // mode -> docker-compose.yml 内容
	Env      []byte            // .env 内容
}

// Generate 从完整 compose 生成指定 modes 的 compose 与 .env；envOverride 可选覆盖 .env 内容（为空则从 compose 推断）；opts 为 nil 时使用默认（健康检查开、Traefik 开、暴露端口开、前缀 the-gate-、无 env 覆盖）。
func Generate(full map[string]interface{}, modes []string, envOverride string, opts *Options) (*Generated, error) {
	out := &Generated{Composes: make(map[string][]byte), Env: nil}
	for _, mode := range modes {
		yml, err := GenerateOne(full, mode, opts)
		if err != nil {
			return nil, err
		}
		out.Composes[mode] = yml
	}
	vars := ExtractEnvVars(full)
	if opts != nil && len(opts.EnvOverrides) > 0 {
		for k, v := range opts.EnvOverrides {
			vars[k] = v
		}
	}
	if envOverride != "" {
		out.Env = []byte(envOverride)
	} else {
		out.Env = []byte(EnvBodyFromVars(vars, ""))
	}
	if len(out.Env) == 0 {
		out.Env = []byte(DefaultEnvBody())
	}
	return out, nil
}

// AllTraefikModes 返回所有可由 canonical 生成的 traefik 相关 mode。
func AllTraefikModes() []string {
	var modes []string
	for _, d := range traefikSplitDefs {
		modes = append(modes, d.name)
	}
	return modes
}
