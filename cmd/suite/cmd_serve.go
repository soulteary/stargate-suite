// Package main: serve command and Web UI (loadPageData, /api/generate handler).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/policy"
	"gopkg.in/yaml.v3"
)

// parseRequest 为 /api/parse 请求体。
type parseRequest struct {
	Compose string `json:"compose"`
	Env     string `json:"env"`
}

// parseResponse 为 /api/parse 响应体。
type parseResponse struct {
	Services []string          `json:"services"`
	EnvVars  map[string]string `json:"envVars"`
	Errors   []string          `json:"errors"`
}

// applyResponse 为 /api/apply 响应体；用于解析后一键导入生成配置。
type applyResponse struct {
	OK             bool              `json:"ok"`
	Services       []string          `json:"services"`
	EnvVars        map[string]string `json:"envVars"`
	SuggestedModes []string          `json:"suggestedModes"`
	SuggestedScene string            `json:"suggestedScene,omitempty"`
	Redirect       string            `json:"redirect,omitempty"`
	Errors         []string          `json:"errors,omitempty"`
}

func handleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Compose) == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(parseResponse{Errors: []string{"compose is required"}})
		return
	}
	parsed, err := composegen.ParseCompose([]byte(req.Compose))
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(parseResponse{Errors: []string{err.Error()}})
		return
	}
	services := extractServiceNames(parsed)
	envVars := composegen.ExtractEnvVars(parsed)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(parseResponse{Services: services, EnvVars: envVars, Errors: []string{}})
}

func extractServiceNames(compose map[string]interface{}) []string {
	svc, ok := compose["services"].(map[string]interface{})
	if !ok {
		return nil
	}
	names := make([]string, 0, len(svc))
	for k := range svc {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// parseEnvText parses dotenv assignments, including the multiline literal
// single-quoted form emitted by composegen.EncodeEnvValue.
func parseEnvText(env string) map[string]string {
	out := make(map[string]string)
	lines := strings.Split(env, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if strings.HasPrefix(val, "'") {
			var literal strings.Builder
			part := val[1:]
			for {
				end := singleQuotedEnd(part)
				if end >= 0 {
					literal.WriteString(decodeSingleQuoted(part[:end]))
					val = literal.String()
					break
				}
				literal.WriteString(decodeSingleQuoted(part))
				if i+1 >= len(lines) {
					val = literal.String()
					break
				}
				literal.WriteByte('\n')
				i++
				part = lines[i]
			}
		} else if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			out[key] = val
		}
	}
	return out
}

func singleQuotedEnd(value string) int {
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) && value[i+1] == '\'' {
			i++
			continue
		}
		if value[i] == '\'' {
			return i
		}
	}
	return -1
}

func decodeSingleQuoted(value string) string {
	return strings.ReplaceAll(value, `\'`, `'`)
}

// suggestModes 根据解析出的服务名推断建议勾选的 compose 类型（用于一键导入）。
func suggestModes(services []string) []string {
	set := make(map[string]bool)
	for _, s := range services {
		set[s] = true
	}
	hasHerald := set["herald"]
	hasWarden := set["warden"]
	hasStargate := set["stargate"]
	if (hasHerald && hasWarden) || (hasHerald && hasStargate) || (hasWarden && hasStargate) {
		return []string{"traefik"}
	}
	if hasHerald {
		return []string{"traefik-herald"}
	}
	if hasWarden {
		return []string{"traefik-warden"}
	}
	if hasStargate {
		return []string{"traefik-stargate"}
	}
	return nil
}

func envBool(envVars map[string]string, key string) bool {
	v, ok := envVars[key]
	if !ok {
		return false
	}
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

func suggestScene(services []string, envVars map[string]string) string {
	set := make(map[string]bool)
	for _, s := range services {
		set[s] = true
	}
	hasStargate := set["stargate"]
	hasWarden := set["warden"]
	hasHerald := set["herald"]
	if hasStargate && hasWarden && hasHerald {
		hasPluginSignals := envBool(envVars, "HERALD_TOTP_ENABLED") ||
			strings.TrimSpace(envVars["HERALD_SMTP_API_URL"]) != "" ||
			strings.TrimSpace(envVars["HERALD_DINGTALK_API_URL"]) != "" ||
			strings.TrimSpace(envVars["SMS_PROVIDER"]) != ""
		if hasPluginSignals {
			return "s5-gate-warden-herald-plugins"
		}
		return "s4-gate-warden-herald"
	}
	if hasStargate && hasWarden && !hasHerald {
		return "s3-gate-warden"
	}
	if hasStargate && !hasWarden && !hasHerald {
		if envBool(envVars, "SESSION_STORAGE_ENABLED") {
			return "s2-solo-gate-session-redis"
		}
		return "s1-solo-gate"
	}
	return ""
}

func handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Compose) == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(applyResponse{OK: false, Errors: []string{"compose is required"}})
		return
	}
	parsed, err := composegen.ParseCompose([]byte(req.Compose))
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(applyResponse{OK: false, Errors: []string{err.Error()}})
		return
	}
	services := extractServiceNames(parsed)
	envVars := composegen.ExtractEnvVars(parsed)
	// .env 文本覆盖/追加到从 compose 提取的变量
	for k, v := range parseEnvText(req.Env) {
		envVars[k] = v
	}
	suggested := suggestModes(services)
	suggestedScene := suggestScene(services, envVars)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(applyResponse{
		OK:             true,
		Services:       services,
		EnvVars:        envVars,
		SuggestedModes: suggested,
		SuggestedScene: suggestedScene,
	})
}

func cacheControlHandler(value string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
}

// sessionMiddleware injects session (and new cookie if needed) into request context.
func sessionMiddleware(cfg serveConfig, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sid string
		var data *SessionData
		if c, err := r.Cookie(sessionCookieName); err == nil && c != nil && c.Value != "" {
			if d, ok := defaultStore.Get(c.Value); ok {
				sid, data = c.Value, d
			}
		}
		if sid == "" {
			newID, err := newSessionID()
			if err != nil {
				http.Error(w, "session error", http.StatusInternalServerError)
				return
			}
			sid = newID
			data = &SessionData{ExpiresAt: time.Now().Add(sessionTTL)}
			defaultStore.Set(sid, data)
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    sid,
				Path:     "/",
				MaxAge:   int(sessionTTL.Seconds()),
				HttpOnly: true,
				// The browser-facing scheme is explicit configuration because a
				// reverse proxy can rewrite both the upstream scheme and Host.
				Secure:   cfg.secureCookie,
				SameSite: http.SameSiteStrictMode,
			})
		}
		r = r.WithContext(WithSessionID(WithSession(r.Context(), data), sid))
		h.ServeHTTP(w, r)
	})
}

// loadI18nFragment 加载单语言文案，如 config/i18n/zh.yaml，返回 map[string]string（顶层 key 为 zh/en）。
func loadI18nFragment(assetPath string) (lang string, entries map[string]string, err error) {
	data, err := readAsset(assetPath)
	if err != nil {
		return "", nil, err
	}
	var out map[string]map[string]string
	if err := yaml.Unmarshal(data, &out); err != nil {
		return "", nil, err
	}
	for k, v := range out {
		return k, v, nil
	}
	return "", nil, fmt.Errorf("empty i18n file: %s", assetPath)
}

// loadPageData 从嵌入/覆盖资产读取页面配置及其拆分片段。yamlPath 为仓库相对路径（斜杠分隔），如 "config/page.yaml"。
func loadPageData(yamlPath string) (*pageData, error) {
	data, err := readAsset(yamlPath)
	if err != nil {
		return nil, err
	}
	var raw pageYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	configDir := path.Dir(yamlPath)

	// 拆分布局：从独立文件合并 configSections / i18n / services / providers
	if len(raw.ConfigSections) == 0 {
		p := path.Join(configDir, "config-sections.yaml")
		if b, err := readAsset(p); err == nil {
			var frag struct {
				ConfigSections []configOptionSection `yaml:"configSections"`
			}
			if err := yaml.Unmarshal(b, &frag); err == nil && len(frag.ConfigSections) > 0 {
				raw.ConfigSections = frag.ConfigSections
			}
		}
	}
	if len(raw.I18N) == 0 {
		raw.I18N = make(map[string]map[string]string)
		for _, name := range []string{"zh", "en"} {
			p := path.Join(configDir, "i18n", name+".yaml")
			lang, entries, err := loadI18nFragment(p)
			if err == nil && lang != "" {
				raw.I18N[lang] = entries
			}
		}
	}
	if len(raw.Services) == 0 {
		p := path.Join(configDir, "services.yaml")
		if b, err := readAsset(p); err == nil {
			var frag struct {
				Services []pageService `yaml:"services"`
			}
			if err := yaml.Unmarshal(b, &frag); err == nil && len(frag.Services) > 0 {
				raw.Services = frag.Services
			}
		}
	}
	if len(raw.Providers) == 0 {
		p := path.Join(configDir, "providers.yaml")
		if b, err := readAsset(p); err == nil {
			var frag struct {
				Providers []pageService `yaml:"providers"`
			}
			if err := yaml.Unmarshal(b, &frag); err == nil {
				raw.Providers = frag.Providers
			}
		}
	}

	var keysStepVars []envVar
	keysStepPath := path.Join(configDir, "keys-step.yaml")
	if b, err := readAsset(keysStepPath); err == nil {
		var frag keysStepYAML
		if err := yaml.Unmarshal(b, &frag); err == nil && len(frag.KeysStepVars) > 0 {
			keysStepVars = frag.KeysStepVars
		}
	}

	jsonI18N, err := json.Marshal(raw.I18N)
	if err != nil {
		return nil, err
	}
	scenarios, err := loadScenarioPresets()
	if err != nil {
		// 场景为增强能力：读取失败时回退为空，避免阻断 Web UI 启动
		scenarios = map[string]scenarioPreset{}
	}
	jsonScenarios, err := json.Marshal(scenarios)
	if err != nil {
		return nil, err
	}
	title := "Stargate Suite - Compose 生成"
	if raw.I18N != nil {
		if t, ok := raw.I18N["zh"]["title"]; ok && t != "" {
			title = t
		}
	}
	var portsList []portDef
	portsPath := path.Join(configDir, "ports.yaml")
	if b, err := readAsset(portsPath); err == nil {
		var frag struct {
			Ports []portDef `yaml:"ports"`
		}
		if err := yaml.Unmarshal(b, &frag); err == nil && len(frag.Ports) > 0 {
			portsList = frag.Ports
		}
	}
	var profilesList []pageProfile
	if ps, err := loadProfiles(); err == nil && ps != nil {
		for _, name := range ps.Names() {
			p, _ := ps.Get(name)
			profilesList = append(profilesList, pageProfile{
				Name:         p.Name,
				Description:  p.Description,
				Experimental: p.Experimental,
				Strict:       p.Strict(),
			})
		}
	}
	return &pageData{
		I18N:           template.JS(jsonI18N),
		Scenarios:      template.JS(jsonScenarios),
		Title:          title,
		Lang:           "zh-CN",
		Modes:          raw.Modes,
		ConfigSections: raw.ConfigSections,
		Services:       raw.Services,
		Providers:      raw.Providers,
		KeysStepVars:   keysStepVars,
		Ports:          portsList,
		Profiles:       profilesList,
		PortValues:     nil, // 由 renderPage 在渲染时按 Session 填写
	}, nil
}

// handleWizardStepPost parses POST body (form or JSON), updates session, redirects to next step or review.
// isEnvVarKey returns true if key looks like an env var (e.g. AUTH_HOST, WARDEN_URL).
func isEnvVarKey(key string) bool {
	if key == "" {
		return false
	}
	for _, c := range key {
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			continue
		}
		return false
	}
	return key[0] >= 'A' && key[0] <= 'Z'
}

func handleWizardStepPost(w http.ResponseWriter, r *http.Request, step int) {
	sess, ok := GetSession(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
		var payload struct {
			Profile      string                 `json:"profile"`
			Modes        []string               `json:"modes"`
			Options      map[string]interface{} `json:"options"`
			EnvOverrides map[string]string      `json:"envOverrides"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if step == 1 && strings.TrimSpace(payload.Profile) != "" {
			sess.Profile = strings.TrimSpace(payload.Profile)
		}
		if step == 1 && len(payload.Modes) > 0 {
			sess.Modes = payload.Modes
		}
		if step >= 2 && payload.Options != nil {
			if sess.Options == nil {
				sess.Options = make(map[string]interface{})
			}
			for k, v := range payload.Options {
				sess.Options[k] = v
			}
		}
		if payload.EnvOverrides != nil {
			if sess.EnvOverrides == nil {
				sess.EnvOverrides = make(map[string]string)
			}
			for k, v := range payload.EnvOverrides {
				sess.EnvOverrides[k] = v
			}
		}
	} else {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if step == 1 {
			if p := strings.TrimSpace(r.FormValue("profile")); p != "" {
				sess.Profile = p
			}
			if modes := r.Form["mode"]; len(modes) > 0 {
				sess.Modes = modes
			}
			if scenarioID := strings.TrimSpace(r.FormValue("scenario")); scenarioID != "" {
				applyScenarioToSession(sess, scenarioID)
			}
		}
		if step >= 2 {
			if sess.Options == nil {
				sess.Options = make(map[string]interface{})
			}
			if sess.EnvOverrides == nil {
				sess.EnvOverrides = make(map[string]string)
			}
			for k, v := range r.Form {
				if k == "mode" || k == "scenario" {
					continue
				}
				if len(v) == 0 {
					continue
				}
				val := v[len(v)-1] // last value wins (checkbox override)
				if isEnvVarKey(k) {
					sess.EnvOverrides[k] = val
				} else {
					switch val {
					case "true", "on", "1":
						sess.Options[k] = true
					case "false", "off", "0":
						sess.Options[k] = false
					default:
						sess.Options[k] = val
					}
				}
			}
		}
	}
	SaveSession(r.Context(), sess)
	next := fmt.Sprintf("/wizard/step-%d", step+1)
	if step >= 5 {
		next = "/review"
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func handleKeysApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, ok := GetSession(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if sess.KeysOverrides == nil {
		sess.KeysOverrides = make(map[string]string)
	}
	for k, v := range payload {
		sess.KeysOverrides[k] = v
	}
	SaveSession(r.Context(), sess)
	http.Redirect(w, r, "/wizard/step-2", http.StatusFound)
}

func handleImportParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Reuse same logic as /api/parse
	handleParse(w, r)
}

func handleImportApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
	var req parseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	writeApplyJSON := func(status int, res applyResponse) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	}
	if strings.TrimSpace(req.Compose) == "" {
		writeApplyJSON(http.StatusBadRequest, applyResponse{OK: false, Errors: []string{"compose is required"}})
		return
	}
	parsed, err := composegen.ParseCompose([]byte(req.Compose))
	if err != nil {
		writeApplyJSON(http.StatusBadRequest, applyResponse{OK: false, Errors: []string{err.Error()}})
		return
	}
	services := extractServiceNames(parsed)
	envVars := composegen.ExtractEnvVars(parsed)
	for k, v := range parseEnvText(req.Env) {
		envVars[k] = v
	}
	sess, ok := GetSession(r.Context())
	if !ok || sess == nil {
		writeApplyJSON(http.StatusUnauthorized, applyResponse{OK: false, Errors: []string{"session required"}})
		return
	}
	suggestedModes := suggestModes(services)
	suggestedScene := suggestScene(services, envVars)
	if len(suggestedModes) > 0 {
		sess.Modes = suggestedModes
	}
	if suggestedScene != "" {
		applyScenarioToSession(sess, suggestedScene)
	}
	if sess.EnvOverrides == nil {
		sess.EnvOverrides = make(map[string]string)
	}
	for k, v := range envVars {
		sess.EnvOverrides[k] = v
	}
	sess.ImportApplied = &ImportApplied{
		EnvVars:        envVars,
		SuggestedModes: suggestedModes,
		SuggestedScene: suggestedScene,
	}
	SaveSession(r.Context(), sess)
	writeApplyJSON(http.StatusOK, applyResponse{
		OK:             true,
		Services:       services,
		EnvVars:        envVars,
		SuggestedModes: suggestedModes,
		SuggestedScene: suggestedScene,
		Redirect:       "/wizard/step-2",
	})
}

func handleGeneratePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, ok := GetSession(r.Context())
	if !ok || sess == nil || len(sess.Modes) == 0 {
		http.Redirect(w, r, "/wizard/step-1", http.StatusFound)
		return
	}
	full, err := composegen.LoadComposeFS(assetFS(), canonicalCompose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load compose: %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	opts := sessionToComposegenOptions(sess)
	if portsList, err := loadPortsConfig(); err == nil && len(portsList) > 0 {
		applyPortsConfigToOptions(opts, portsList)
	}
	// 向导原生表单已通过 hidden+uncheckValue 写入 EnvOverrides；此处再兜底一次，避免漏传导致 WARDEN_REDIS_ENABLED 回落到 canonical 默认 true。
	if pageSections, err := loadConfigSections(); err == nil && opts != nil && opts.EnvOverrides != nil {
		applyUncheckEnvDefaults(opts.EnvOverrides, pageSections)
	}
	envBody := ""
	for k, v := range sess.EnvOverrides {
		envBody += k + "=" + v + "\n"
	}
	for k, v := range sess.KeysOverrides {
		envBody += k + "=" + v + "\n"
	}
	envMeta, _ := composegen.LoadEnvMetaFS(assetFS(), "config/env-meta.yaml")

	// Profile-aware path: when a deployment profile is selected, route through
	// the SAME shared policy + composegen model the CLI uses (generateForProfile),
	// enforcing policy first. production strict violations are hard errors.
	if strings.TrimSpace(sess.Profile) != "" && policyKnownProfile(sess.Profile) {
		prof, perr := resolveProfile(sess.Profile)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "profile: %v\n", perr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		userEnv := map[string]string{}
		for k, v := range sess.EnvOverrides {
			userEnv[k] = v
		}
		for k, v := range sess.KeysOverrides {
			userEnv[k] = v
		}
		findings := validateForProfile(prof, opts, userEnv)
		if profileFindingsHaveError(findings) {
			writeProfileGenError(w, prof, findings)
			return
		}
		pgen, _, gerr := generateForProfile(profileGenInput{
			Profile:  prof,
			Modes:    sess.Modes,
			BaseOpts: opts,
			UserEnv:  userEnv,
			// Web UI uses crypto/rand (KeyReader nil => crypto/rand).
		})
		if gerr != nil {
			fmt.Fprintf(os.Stderr, "generate: %v\n", gerr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		writeGenerateJSON(w, pgen, findings)
		// Artifacts returned; drop operator secrets from the server session so
		// they are not persisted in memory after download.
		sess.ClearSecrets()
		SaveSession(r.Context(), sess)
		return
	}

	gen, err := composegen.Generate(full, sess.Modes, envBody, opts, envMeta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// Return JSON for multi-page: composes + env (client can show download links)
	res := map[string]interface{}{
		"composes": make(map[string]string),
		"env":      string(gen.Env),
	}
	for mode, yml := range gen.Composes {
		res["composes"].(map[string]string)[mode] = string(yml)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(res)
	sess.ClearSecrets()
	SaveSession(r.Context(), sess)
}

// policyKnownProfile reports whether name is one of the three canonical profiles.
func policyKnownProfile(name string) bool { return policy.KnownProfile(name) }

// profileFindingsHaveError reports whether any policy finding is a hard error.
func profileFindingsHaveError(findings []policy.Finding) bool { return policy.HasErrors(findings) }

// writeProfileGenError returns a 422 with the policy findings so the Web UI can
// show the operator exactly which strict rules blocked generation (production
// is never bypassable). Secrets are never echoed — only rule keys/messages.
func writeProfileGenError(w http.ResponseWriter, prof policy.Profile, findings []policy.Finding) {
	msgs := make([]string, 0, len(findings))
	for _, f := range findings {
		msgs = append(msgs, f.String())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       false,
		"profile":  prof.Name,
		"findings": msgs,
		"error":    fmt.Sprintf("profile %q policy violations must be resolved before generation", prof.Name),
	})
}

// writeGenerateJSON writes the standard generate response (composes + env) plus
// any advisory findings (warnings) for the Web UI.
func writeGenerateJSON(w http.ResponseWriter, gen *composegen.Generated, findings []policy.Finding) {
	res := map[string]interface{}{
		"composes": make(map[string]string),
		"env":      string(gen.Env),
	}
	for mode, yml := range gen.Composes {
		res["composes"].(map[string]string)[mode] = string(yml)
	}
	if len(findings) > 0 {
		msgs := make([]string, 0, len(findings))
		for _, f := range findings {
			msgs = append(msgs, f.String())
		}
		res["findings"] = msgs
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(res)
}

// sessionToComposegenOptions builds composegen.Options from session (options + env overrides + keys)；option 映射与 API 共用 optionToComposeGenJSONSetters。
func sessionToComposegenOptions(sess *SessionData) *composegen.Options {
	o := &composeGenOptionsJSON{EnvOverrides: make(map[string]string)}
	for k, v := range sess.EnvOverrides {
		o.EnvOverrides[k] = v
	}
	for k, v := range sess.KeysOverrides {
		o.EnvOverrides[k] = v
	}
	FillComposeGenOptionsFromMap(o, sess.Options)
	return reqOptionsToComposegen(o)
}

// applyScenarioToSession 按 scenarios.json 的 id 把 modes/options/envOverrides 写入会话。
func applyScenarioToSession(sess *SessionData, scenarioID string) {
	if sess == nil || scenarioID == "" {
		return
	}
	presets, err := loadScenarioPresets()
	if err != nil || presets == nil {
		return
	}
	preset, ok := presets[scenarioID]
	if !ok {
		return
	}
	if len(preset.Modes) > 0 {
		sess.Modes = append([]string(nil), preset.Modes...)
	}
	if sess.Options == nil {
		sess.Options = make(map[string]interface{})
	}
	for k, v := range preset.Options {
		sess.Options[k] = v
	}
	if sess.EnvOverrides == nil {
		sess.EnvOverrides = make(map[string]string)
	}
	for k, v := range preset.EnvOverrides {
		sess.EnvOverrides[k] = v
	}
}

// loadPortsConfig 从 config/ports.yaml 加载端口配置，用于生成时与 ports 表一致。
func loadPortsConfig() ([]portDef, error) {
	b, err := readAsset("config/ports.yaml")
	if err != nil {
		return nil, err
	}
	var frag struct {
		Ports []portDef `yaml:"ports"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, err
	}
	return frag.Ports, nil
}

// loadConfigSections 从 config/config-sections.yaml 加载向导 step-2 配置块（含 uncheckValue 等元数据）。
func loadConfigSections() ([]configOptionSection, error) {
	b, err := readAsset("config/config-sections.yaml")
	if err != nil {
		return nil, err
	}
	var frag struct {
		ConfigSections []configOptionSection `yaml:"configSections"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, err
	}
	return frag.ConfigSections, nil
}

// applyPortsConfigToOptions 用 config/ports.yaml 的容器端口填充 opts（如 HERALD_TOTP_PORT），保证与端口表集中配置一致。
func applyPortsConfigToOptions(opts *composegen.Options, ports []portDef) {
	if opts == nil {
		return
	}
	for _, p := range ports {
		if p.ServiceKey == "herald-totp" && p.ContainerPort != "" {
			opts.HeraldTotpContainerPort = strings.TrimSpace(p.ContainerPort)
			if opts.EnvOverrides == nil {
				opts.EnvOverrides = make(map[string]string)
			}
			if opts.EnvOverrides["HERALD_TOTP_PORT"] == "" {
				opts.EnvOverrides["HERALD_TOTP_PORT"] = ":" + opts.HeraldTotpContainerPort
			}
			return
		}
	}
}

func cmdServe() error {
	// Serve owns its security-related flag set so the Web
	// UI security posture is explicit. Legacy --port / SERVE_PORT (parsed in
	// main) is honored only when --listen is absent, for backward compatibility.
	sfs := flag.NewFlagSet("serve", flag.ContinueOnError)
	sfs.SetOutput(os.Stderr)
	listen := sfs.String("listen", "", "host:port to bind (default 127.0.0.1:8085; loopback-only unless --allow-remote)")
	allowRemote := sfs.Bool("allow-remote", false, "permit binding a non-loopback address (requires and, if unset, generates an access token)")
	token := sfs.String("token", "", "access token required on every request (auto-generated in remote mode when empty)")
	allowInsecureCookie := sfs.Bool("allow-insecure-cookie", false, "permit the auth cookie over HTTP (only for an explicitly loopback-published container port)")
	if err := sfs.Parse(cmdArgs); err != nil {
		return err
	}
	cfg, err := resolveServeConfig(*listen, servePort, *token, *allowRemote, *allowInsecureCookie)
	if err != nil {
		return err
	}

	page, err := loadPageData(pageYAMLPath)
	if err != nil {
		return fmt.Errorf("load page config (%s): %w", pageYAMLPath, err)
	}
	tmpl, err := template.New("root").Funcs(template.FuncMap{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires even number of args")
			}
			m := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[key] = values[i+1]
			}
			return m, nil
		},
	}).ParseFS(staticFS,
		"static/layout.tmpl",
		"static/partials/step-nav.tmpl",
		"static/partials/env-field.tmpl",
		"static/pages/entry.tmpl",
		"static/pages/wizard1.tmpl",
		"static/pages/wizard2.tmpl",
		"static/pages/wizard3.tmpl",
		"static/pages/wizard4.tmpl",
		"static/pages/wizard5.tmpl",
		"static/pages/keys.tmpl",
		"static/pages/import.tmpl",
		"static/pages/review.tmpl",
	)
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static sub FS: %w", err)
	}
	cacheStatic := "public, max-age=3600"
	staticHandler := cacheControlHandler(cacheStatic, http.FileServer(http.FS(subFS)))

	// renderPage writes the layout template with PageContent and Session set (multi-page mode).
	renderPage := func(w http.ResponseWriter, p *pageData, pageName string, sess *SessionData) {
		clone := *p
		clone.Page = pageName
		clone.PageContent = "content-" + pageName
		clone.Session = sess
		if sess != nil && sess.Options != nil && len(clone.Ports) > 0 {
			clone.PortValues = make(map[string]string)
			for _, port := range clone.Ports {
				if v, ok := sess.Options[port.OptionId]; ok && v != nil {
					clone.PortValues[port.OptionId] = fmt.Sprint(v)
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "base", &clone); err != nil {
			fmt.Fprintf(os.Stderr, "template execute: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}

	mux := http.NewServeMux()
	// Multi-page: entry
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sess, _ := GetSession(r.Context())
		renderPage(w, page, "entry", sess)
	})
	// Wizard steps: GET render, POST save session and redirect next
	for i := 1; i <= 5; i++ {
		step := i
		path := fmt.Sprintf("/wizard/step-%d", step)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			if r.Method == http.MethodPost {
				handleWizardStepPost(w, r, step)
				return
			}
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			sess, _ := GetSession(r.Context())
			renderPage(w, page, fmt.Sprintf("wizard-%d", step), sess)
		})
	}
	// Keys page
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keys" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		sess, _ := GetSession(r.Context())
		renderPage(w, page, "keys", sess)
	})
	mux.HandleFunc("/keys/apply", handleKeysApply)
	mux.HandleFunc("/import", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/import" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		sess, _ := GetSession(r.Context())
		renderPage(w, page, "import", sess)
	})
	mux.HandleFunc("/import/parse", handleImportParse)
	mux.HandleFunc("/import/apply", handleImportApply)
	mux.HandleFunc("/review", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/review" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		sess, _ := GetSession(r.Context())
		renderPage(w, page, "review", sess)
	})
	mux.HandleFunc("/generate", handleGeneratePost)

	mux.Handle("/static/", http.StripPrefix("/static", staticHandler))
	mux.HandleFunc("/api/profiles", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ps, err := loadProfiles()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		type profileInfo struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			Experimental bool   `json:"experimental"`
			Strict       bool   `json:"strict"`
		}
		out := make([]profileInfo, 0, len(ps.Names()))
		for _, name := range ps.Names() {
			p, _ := ps.Get(name)
			out = append(out, profileInfo{
				Name:         p.Name,
				Description:  p.Description,
				Experimental: p.Experimental,
				Strict:       p.Strict(),
			})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"profiles": out})
	})
	mux.HandleFunc("/api/parse", handleParse)
	mux.HandleFunc("/api/apply", handleApply)
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if len(req.Modes) == 0 {
			http.Error(w, "modes required", http.StatusBadRequest)
			return
		}
		full, err := composegen.LoadComposeFS(assetFS(), canonicalCompose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load compose: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		opts := reqOptionsToComposegen(req.Options)
		if len(page.Ports) > 0 {
			applyPortsConfigToOptions(opts, page.Ports)
		}
		// 仅当调用方显式传入 options.envOverrides（向导/JS 序列化）时补 uncheck 默认；
		// make gen 传 options:null，必须保留 canonical 默认（如 WARDEN_REDIS_ENABLED=true）。
		if req.Options != nil && req.Options.EnvOverrides != nil && opts != nil {
			if opts.EnvOverrides == nil {
				opts.EnvOverrides = make(map[string]string)
			}
			applyUncheckEnvDefaults(opts.EnvOverrides, page.ConfigSections)
		}
		envMeta, _ := composegen.LoadEnvMetaFS(assetFS(), "config/env-meta.yaml")
		gen, err := composegen.Generate(full, req.Modes, req.EnvOverride, opts, envMeta)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		res := map[string]interface{}{
			"composes": make(map[string]string),
			"env":      string(gen.Env),
		}
		for mode, yml := range gen.Composes {
			res["composes"].(map[string]string)[mode] = string(yml)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	addr := cfg.listenAddr
	srv := newSecureHTTPServer(addr, securityMiddleware(cfg, sessionMiddleware(cfg, mux)))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// No silent port hopping: if the requested address is unavailable we
		// surface it and stop, so the operator always knows exactly where the
		// UI is (or is not) listening.
		if strings.Contains(err.Error(), "address already in use") {
			return fmt.Errorf("listen %s: address already in use (choose another --listen or free the port; the Web UI will not silently switch ports)", addr)
		}
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	go func() {
		tick := time.NewTicker(5 * time.Minute)
		defer tick.Stop()
		for range tick.C {
			defaultStore.cleanupExpired()
		}
	}()
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		}
	}()
	scheme := "http"
	openAddr := browserAddr(addr)
	fmt.Printf("Web UI: %s://%s\n", scheme, openAddr)
	if !cfg.loopback {
		fmt.Printf("Remote access enabled. Access token required.\n")
		fmt.Printf("  Open locally: %s://%s/?token=%s\n", scheme, openAddr, cfg.token)
		fmt.Printf("  For off-host access, use an HTTPS reverse proxy and replace the URL host.\n")
		fmt.Printf("  Or send header: Authorization: Bearer %s\n", cfg.token)
	} else if cfg.token != "" {
		fmt.Printf("Access token required: %s\n", cfg.token)
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	fmt.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
