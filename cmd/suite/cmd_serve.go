// Package main: serve command and Web UI (loadPageData, /api/generate handler).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/the-gate/internal/composegen"
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

// parseEnvText 将 .env 文本解析为 KEY=VALUE 映射（每行一条，空行与 # 开头忽略）。
func parseEnvText(env string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(env, "\n") {
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
		// 去除可选的引号
		if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) || (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			out[key] = val
		}
	}
	return out
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
func sessionMiddleware(h http.Handler) http.Handler {
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
				SameSite: http.SameSiteLaxMode,
			})
		}
		r = r.WithContext(WithSessionID(WithSession(r.Context(), data), sid))
		h.ServeHTTP(w, r)
	})
}

// loadI18nFragment 加载单语言文案，如 config/i18n/zh.yaml，返回 map[string]string（顶层 key 为 zh/en）。
func loadI18nFragment(path string) (lang string, entries map[string]string, err error) {
	data, err := os.ReadFile(path)
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
	return "", nil, fmt.Errorf("empty i18n file: %s", path)
}

func loadPageData(yamlPath string) (*pageData, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	var raw pageYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	configDir := filepath.Dir(yamlPath)
	rootDir := filepath.Dir(configDir)

	// 拆分布局：从独立文件合并 configSections / i18n / services / providers
	if len(raw.ConfigSections) == 0 {
		path := filepath.Join(configDir, "config-sections.yaml")
		if b, err := os.ReadFile(path); err == nil {
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
			path := filepath.Join(configDir, "i18n", name+".yaml")
			lang, entries, err := loadI18nFragment(path)
			if err == nil && lang != "" {
				raw.I18N[lang] = entries
			}
		}
	}
	if len(raw.Services) == 0 {
		path := filepath.Join(configDir, "services.yaml")
		if b, err := os.ReadFile(path); err == nil {
			var frag struct {
				Services []pageService `yaml:"services"`
			}
			if err := yaml.Unmarshal(b, &frag); err == nil && len(frag.Services) > 0 {
				raw.Services = frag.Services
			}
		}
	}
	if len(raw.Providers) == 0 {
		path := filepath.Join(configDir, "providers.yaml")
		if b, err := os.ReadFile(path); err == nil {
			var frag struct {
				Providers []pageService `yaml:"providers"`
			}
			if err := yaml.Unmarshal(b, &frag); err == nil {
				raw.Providers = frag.Providers
			}
		}
	}

	var keysStepVars []envVar
	keysStepPath := filepath.Join(configDir, "keys-step.yaml")
	if b, err := os.ReadFile(keysStepPath); err == nil {
		var frag keysStepYAML
		if err := yaml.Unmarshal(b, &frag); err == nil && len(frag.KeysStepVars) > 0 {
			keysStepVars = frag.KeysStepVars
		}
	}

	jsonI18N, err := json.Marshal(raw.I18N)
	if err != nil {
		return nil, err
	}
	scenarios, err := loadScenarioPresets(rootDir)
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
			Modes        []string               `json:"modes"`
			Options      map[string]interface{} `json:"options"`
			EnvOverrides map[string]string      `json:"envOverrides"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
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
			if modes := r.Form["mode"]; len(modes) > 0 {
				sess.Modes = modes
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
					if val == "true" || val == "on" || val == "1" {
						sess.Options[k] = true
					} else if val == "false" || val == "off" || val == "0" {
						sess.Options[k] = false
					} else {
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
	if strings.TrimSpace(req.Compose) == "" {
		http.Redirect(w, r, "/import", http.StatusFound)
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
	for k, v := range parseEnvText(req.Env) {
		envVars[k] = v
	}
	sess, ok := GetSession(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	sess.ImportApplied = &ImportApplied{
		EnvVars:        envVars,
		SuggestedModes: suggestModes(services),
		SuggestedScene: suggestScene(services, envVars),
	}
	SaveSession(r.Context(), sess)
	http.Redirect(w, r, "/wizard/step-1", http.StatusFound)
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
	root := projectRoot()
	fullPath := filepath.Join(root, canonicalCompose)
	full, err := composegen.LoadCompose(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load compose: %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	opts := sessionToComposegenOptions(sess)
	envBody := ""
	for k, v := range sess.EnvOverrides {
		envBody += k + "=" + v + "\n"
	}
	for k, v := range sess.KeysOverrides {
		envBody += k + "=" + v + "\n"
	}
	envMeta, _ := composegen.LoadEnvMeta(filepath.Join(root, "config", "env-meta.yaml"))
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

func cmdServe() error {
	root := projectRoot()
	pagePath := filepath.Join(root, pageYAMLPath)
	page, err := loadPageData(pagePath)
	if err != nil {
		if cwd, e := os.Getwd(); e == nil {
			fallback := filepath.Join(cwd, pageYAMLPath)
			page, err = loadPageData(fallback)
		}
		if err != nil {
			return fmt.Errorf("load page config (tried %s and ./%s): %w", pagePath, pageYAMLPath, err)
		}
	}
	tmpl, err := template.ParseFS(staticFS,
		"static/layout.tmpl",
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
		root := projectRoot()
		fullPath := filepath.Join(root, canonicalCompose)
		full, err := composegen.LoadCompose(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load compose: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		opts := reqOptionsToComposegen(req.Options)
		envMeta, _ := composegen.LoadEnvMeta(filepath.Join(root, "config", "env-meta.yaml"))
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
	addr := ":" + servePort
	srv := &http.Server{Addr: addr, Handler: sessionMiddleware(mux)}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			start, _ := strconv.Atoi(servePort)
			if start <= 0 {
				start = 8085
			}
			for p := start + 1; p < start+10; p++ {
				tryAddr := ":" + strconv.Itoa(p)
				listener, err = net.Listen("tcp", tryAddr)
				if err == nil {
					fmt.Fprintf(os.Stderr, "Port %s in use, using %s instead.\n", addr, tryAddr)
					addr = tryAddr
					break
				}
			}
		}
		if err != nil {
			return fmt.Errorf("listen %s: %w", addr, err)
		}
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
	fmt.Printf("Web UI: http://localhost%s\n", addr)
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
