// Package main 提供与 Makefile 等效的 CLI，用于 the-gate 集成测试项目的编排与测试。
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/cli-kit/configutil"
	"github.com/soulteary/cli-kit/flagutil"
	"github.com/soulteary/the-gate/internal/composegen"
	"gopkg.in/yaml.v3"
)

//go:embed static
var staticFS embed.FS

const (
	pageYAMLPath         = "config/page.yaml"
	presetsPath          = "config/presets.json"
	fallbackCompose      = "compose/example/image/docker-compose.yml"
	canonicalCompose     = "compose/canonical/docker-compose.yml"
	defaultEnvBody       = "" // 使用 composegen.DefaultEnvBody 或从 compose 推断
	buildDirRelative     = "build"
	maxGenerateBodyBytes = 1 << 20 // 1MB for /api/generate request body
)

// pageData 与 config/page.yaml 对应，用于渲染 index 模板。
type pageData struct {
	I18N           template.JS           `json:"-"`
	Title          string                `yaml:"-"`
	Lang           string                `yaml:"-"`
	Modes          []pageMode            `yaml:"modes"`
	ConfigSections []configOptionSection `yaml:"configSections"`
	Services       []pageService         `yaml:"services"`
}

// configOptionSection 配置选项分组（类似服务特性中的 env-group）。
type configOptionSection struct {
	TitleKey string         `yaml:"titleKey"`
	Options  []configOption `yaml:"options"`
}

type pageMode struct {
	Value    string `yaml:"value"`
	LabelKey string `yaml:"labelKey"`
	DescKey  string `yaml:"descKey"`
}

type configOption struct {
	Type           string      `yaml:"type"`
	Id             string      `yaml:"id"`
	Name           string      `yaml:"name"`
	EnvName        string      `yaml:"envName"`
	LabelKey       string      `yaml:"labelKey"`
	DescKey        string      `yaml:"descKey"`
	PlaceholderKey string      `yaml:"placeholderKey"`
	Placeholder    string      `yaml:"placeholder"`
	Default        interface{} `yaml:"default"`
	TitleKey       string      `yaml:"titleKey"`
	Value          string      `yaml:"value"`
	Paths          []redisPath `yaml:"paths"`
	ShowWhenOption string      `yaml:"showWhenOption"` // 仅当某 checkbox 勾选时显示
	Min            int         `yaml:"min"`
	Max            int         `yaml:"max"`
}

type redisPath struct {
	Env         string `yaml:"env"`
	Id          string `yaml:"id"`
	LabelKey    string `yaml:"labelKey"`
	DescKey     string `yaml:"descKey"`
	Default     string `yaml:"default"`
	Placeholder string `yaml:"placeholder"`
}

type pageService struct {
	Id       string        `yaml:"id"`
	Name     string        `yaml:"name"`
	Open     bool          `yaml:"open"`
	Sections []pageSection `yaml:"sections"`
}

type pageSection struct {
	TitleKey string   `yaml:"titleKey"`
	EnvVars  []envVar `yaml:"envVars"`
}

type envVar struct {
	Env         string         `yaml:"env"`
	Type        string         `yaml:"type"`
	LabelKey    string         `yaml:"labelKey"`
	DescKey     string         `yaml:"descKey"`
	Default     interface{}    `yaml:"default"`
	Placeholder string         `yaml:"placeholder"`
	Min         int            `yaml:"min"`
	Max         int            `yaml:"max"`
	Options     []selectOption `yaml:"options"`
	ShowWhenEnv string         `yaml:"showWhenEnv"` // 仅当某 env 为 true 时显示
}

type selectOption struct {
	Value    string `yaml:"value"`
	LabelKey string `yaml:"labelKey"`
}

type pageYAML struct {
	I18N           map[string]map[string]string `yaml:"i18n"`
	Modes          []pageMode                   `yaml:"modes"`
	ConfigSections []configOptionSection        `yaml:"configSections"`
	Services       []pageService                `yaml:"services"`
}

// cacheControlHandler wraps h and sets Cache-Control on successful responses.
func cacheControlHandler(value string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		h.ServeHTTP(w, r)
	})
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
	// I18N as JSON for window.I18N (trusted YAML source, no user input)
	jsonI18N, err := json.Marshal(raw.I18N)
	if err != nil {
		return nil, err
	}
	title := "Stargate Suite - Compose 生成"
	if t, ok := raw.I18N["zh"]["title"]; ok && t != "" {
		title = t
	}
	return &pageData{
		I18N:           template.JS(jsonI18N),
		Title:          title,
		Lang:           "zh-CN",
		Modes:          raw.Modes,
		ConfigSections: raw.ConfigSections,
		Services:       raw.Services,
	}, nil
}

// resolvedComposeFile 由 main 在解析全局 flag 后设置，供使用默认 compose 的命令读取。
var resolvedComposeFile string

// genOutDir、genModeArg 由 main 在解析 gen 命令时设置，供 cmdGen 使用。
var genOutDir, genModeArg string

// servePort 由 main 在解析 serve 命令时设置。
var servePort string

func loadPresets(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// composeFile 返回当前解析后的 compose 文件路径。main() 保证 resolvedComposeFile 非空。
func composeFile() string {
	return resolvedComposeFile
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type command struct {
	name, desc string
	fn         func() error
}

var commands []command

func getCommands() []command {
	if len(commands) == 0 {
		commands = []command{
			{"help", "Show help information", cmdHelp},
			{"up", "Start all services（默认 compose/image）", cmdUp},
			{"up-build", "Start all services（从源码构建，compose/build）", cmdUpBuild},
			{"up-image", "Start all services（预构建镜像，compose/image）", cmdUpImage},
			{"up-traefik", "Start all services（接入 Traefik，三合一 compose）", cmdUpTraefik},
			{"net-traefik-split", "Create networks for split Traefik compose（三分开前执行一次）", cmdNetTraefikSplit},
			{"up-traefik-herald", "Start Herald only（三分开）", cmdUpTraefikHerald},
			{"up-traefik-warden", "Start Warden only（三分开）", cmdUpTraefikWarden},
			{"up-traefik-stargate", "Start Stargate + protected-service only（三分开，依赖 Herald/Warden 已启动）", cmdUpTraefikStargate},
			{"down", "Stop all services（默认与 up 一致，使用 COMPOSE_FILE）", cmdDown},
			{"down-build", "Stop compose/build 启动的服务", cmdDownBuild},
			{"down-image", "Stop compose/image 启动的服务", cmdDownImage},
			{"down-traefik", "Stop compose/traefik 三合一启动的服务", cmdDownTraefik},
			{"down-traefik-herald", "Stop Herald（三分开）", cmdDownTraefikHerald},
			{"down-traefik-warden", "Stop Warden（三分开）", cmdDownTraefikWarden},
			{"down-traefik-stargate", "Stop Stargate（三分开）", cmdDownTraefikStargate},
			{"logs", "View service logs", cmdLogs},
			{"ps", "View service status", cmdPs},
			{"test", "Run end-to-end tests", cmdTest},
			{"test-wait", "Wait for services to be ready then run tests (recommended)", cmdTestWait},
			{"clean", "Clean services and data volumes", cmdClean},
			{"restart", "Restart all services", cmdRestart},
			{"restart-warden", "Restart Warden service", cmdRestartWarden},
			{"restart-herald", "Restart Herald service", cmdRestartHerald},
			{"restart-stargate", "Restart Stargate service", cmdRestartStargate},
			{"health", "Check service health status", cmdHealth},
			{"gen", "Generate docker-compose.yml and .env for mode(s) into build dir (use -o to set output dir)", cmdGen},
			{"gen-split", "从 canonical 生成三分开 compose 到 build/（traefik-herald/warden/stargate）", cmdGenSplit},
			{"serve", "Start web UI for compose generation (default :8085)", cmdServe},
		}
	}
	return commands
}

func cmdHelp() error {
	fmt.Println("the-gate End-to-End Integration Test Project")
	fmt.Println()
	fmt.Printf("Compose 示例见 compose/ 目录，默认使用: %s\n", composeFile())
	fmt.Println("  可通过 -f/--file、COMPOSE_FILE、--preset 覆盖，详见 config/README.md")
	fmt.Println()
	fmt.Println("Available commands:")
	for _, c := range getCommands() {
		fmt.Printf("  %-22s %s\n", c.name, c.desc)
	}
	return nil
}

func cmdUp() error {
	return run("docker", "compose", "-f", composeFile(), "up", "-d")
}

func buildComposePath(mode string) string {
	return filepath.Join(projectRoot(), buildDirRelative, mode, "docker-compose.yml")
}

func cmdUpBuild() error {
	return run("docker", "compose", "-f", buildComposePath("build"), "up", "-d", "--build")
}

func cmdUpImage() error {
	return run("docker", "compose", "-f", buildComposePath("image"), "up", "-d")
}

func cmdUpTraefik() error {
	return run("docker", "compose", "-f", buildComposePath("traefik"), "up", "-d")
}

func cmdNetTraefikSplit() error {
	for _, net := range []string{"the-gate-network", "traefik"} {
		cmd := exec.Command("docker", "network", "create", net)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // 忽略错误，网络已存在时也会返回非零
	}
	return nil
}

func cmdUpTraefikHerald() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-herald"), "up", "-d")
}

func cmdUpTraefikWarden() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-warden"), "up", "-d")
}

func cmdUpTraefikStargate() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-stargate"), "up", "-d")
}

func cmdDown() error {
	return run("docker", "compose", "-f", composeFile(), "down")
}

func cmdDownBuild() error {
	return run("docker", "compose", "-f", buildComposePath("build"), "down")
}

func cmdDownImage() error {
	return run("docker", "compose", "-f", buildComposePath("image"), "down")
}

func cmdDownTraefik() error {
	return run("docker", "compose", "-f", buildComposePath("traefik"), "down")
}

func cmdDownTraefikHerald() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-herald"), "down")
}

func cmdDownTraefikWarden() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-warden"), "down")
}

func cmdDownTraefikStargate() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-stargate"), "down")
}

func cmdLogs() error {
	return run("docker", "compose", "-f", composeFile(), "logs", "-f")
}

func cmdPs() error {
	return run("docker", "compose", "-f", composeFile(), "ps")
}

func cmdTest() error {
	return run("go", "test", "-v", "./e2e/...")
}

// testWaitTimeout 为 test-wait 轮询健康检查的超时时间，可通过环境变量 TEST_WAIT_TIMEOUT 覆盖（如 60s、1m）。
var testWaitTimeout = 60 * time.Second

// waitForServicesReady 轮询 Stargate/Warden/Herald 健康检查，全部就绪返回 true，超时返回 false。
func waitForServicesReady(timeout time.Duration) bool {
	checks := []struct {
		name, url string
	}{
		{"Stargate", "http://localhost:8080/_auth"},
		{"Warden", "http://localhost:8081/health"},
		{"Herald", "http://localhost:8082/healthz"},
	}
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allOk := true
		for _, c := range checks {
			resp, err := client.Get(c.url)
			if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
				allOk = false
				if resp != nil {
					_ = resp.Body.Close()
				}
				break
			}
			_ = resp.Body.Close()
		}
		if allOk {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func cmdTestWait() error {
	if s := strings.TrimSpace(os.Getenv("TEST_WAIT_TIMEOUT")); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			testWaitTimeout = d
		}
	}
	fmt.Printf("Waiting for services to be ready (timeout %s)...\n", testWaitTimeout)
	if !waitForServicesReady(testWaitTimeout) {
		fmt.Fprintf(os.Stderr, "Services did not become ready within %s. Run make health to check.\n", testWaitTimeout)
		return fmt.Errorf("services not ready")
	}
	fmt.Println("All services ready. Running tests.")
	return run("go", "test", "-v", "./e2e/...")
}

func cmdClean() error {
	return run("docker", "compose", "-f", composeFile(), "down", "-v")
}

func cmdRestart() error {
	return run("docker", "compose", "-f", composeFile(), "restart")
}

func cmdRestartWarden() error {
	return run("docker", "compose", "-f", composeFile(), "restart", "warden")
}

func cmdRestartHerald() error {
	return run("docker", "compose", "-f", composeFile(), "restart", "herald")
}

func cmdRestartStargate() error {
	return run("docker", "compose", "-f", composeFile(), "restart", "stargate")
}

func cmdHealth() error {
	client := &http.Client{Timeout: 5 * time.Second}
	checks := []struct {
		name, url string
	}{
		{"Stargate", "http://localhost:8080/_auth"},
		{"Warden", "http://localhost:8081/health"},
		{"Herald", "http://localhost:8082/healthz"},
	}
	for _, c := range checks {
		fmt.Printf("Checking %s...\n", c.name)
		resp, err := client.Get(c.url)
		if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
			fmt.Printf("✗ %s Unhealthy\n", c.name)
		} else {
			_ = resp.Body.Close()
			fmt.Printf("✓ %s Healthy\n", c.name)
		}
	}
	return nil
}

// projectRoot 返回项目根目录（包含 compose 与 config 的目录），否则返回当前工作目录。
func projectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "compose")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "config")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd
}

func cmdGen() error {
	outDir := genOutDir
	if outDir == "" {
		outDir = buildDirRelative
	}
	root := projectRoot()
	outBase := filepath.Join(root, outDir)
	modeArg := strings.TrimSpace(genModeArg)
	if modeArg == "" {
		modeArg = "all"
	}
	var exampleModes, traefikModes []string
	switch modeArg {
	case "image", "build":
		exampleModes = []string{modeArg}
	case "traefik":
		traefikModes = []string{"traefik", "traefik-herald", "traefik-warden", "traefik-stargate"}
	case "", "all":
		exampleModes = []string{"image", "build"}
		traefikModes = []string{"traefik", "traefik-herald", "traefik-warden", "traefik-stargate"}
	default:
		// 支持多选，如 gen traefik traefik-herald
		if strings.HasPrefix(modeArg, "traefik") {
			traefikModes = []string{modeArg}
		} else if modeArg == "image" || modeArg == "build" {
			exampleModes = []string{modeArg}
		} else {
			fmt.Fprintf(os.Stderr, "Unknown mode %q. Use: image, build, traefik, traefik-herald, traefik-warden, traefik-stargate, or all\n", modeArg)
			return fmt.Errorf("unknown gen mode: %s", modeArg)
		}
	}
	envBody := ""
	if b, err := os.ReadFile(filepath.Join(root, ".env")); err == nil && len(b) > 0 {
		envBody = string(b)
	}
	if envBody == "" {
		envBody = composegen.DefaultEnvBody()
	}
	for _, mode := range exampleModes {
		if err := genModeExample(root, outBase, mode, envBody); err != nil {
			return err
		}
	}
	if len(traefikModes) > 0 {
		fullPath := filepath.Join(root, canonicalCompose)
		full, err := composegen.LoadCompose(fullPath)
		if err != nil {
			return fmt.Errorf("load canonical compose: %w", err)
		}
		opts := genOptionsFromEnv()
		gen, err := composegen.Generate(full, traefikModes, envBody, opts)
		if err != nil {
			return err
		}
		if err := writeGenerated(outBase, gen, traefikModes); err != nil {
			return err
		}
	}
	allModes := append(exampleModes, traefikModes...)
	fmt.Printf("Generated %s for mode(s): %s\n", outDir, strings.Join(allModes, ", "))
	return nil
}

// genOptionsFromEnv 从环境变量构建 composegen.Options（USE_NAMED_VOLUME、HERALD_REDIS_DATA_PATH、WARDEN_REDIS_DATA_PATH 等）。
func genOptionsFromEnv() *composegen.Options {
	useNamed := true
	if v := strings.TrimSpace(os.Getenv("USE_NAMED_VOLUME")); v == "0" || strings.EqualFold(v, "false") {
		useNamed = false
	}
	opts := &composegen.Options{
		HealthCheck:         true,
		TraefikNetwork:      true,
		TraefikNetworkName:  "traefik",
		ExposePorts:         true,
		ContainerNamePrefix: "the-gate-",
		UseNamedVolume:      useNamed,
		HeraldRedisDataPath: strings.TrimSpace(os.Getenv("HERALD_REDIS_DATA_PATH")),
		WardenRedisDataPath: strings.TrimSpace(os.Getenv("WARDEN_REDIS_DATA_PATH")),
	}
	if opts.HeraldRedisDataPath == "" {
		opts.HeraldRedisDataPath = "./data/herald-redis"
	}
	if opts.WardenRedisDataPath == "" {
		opts.WardenRedisDataPath = "./data/warden-redis"
	}
	return opts
}

func genModeExample(projectRoot, outBase, mode, envBody string) error {
	dir := filepath.Join(outBase, mode)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0644); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	src := filepath.Join(projectRoot, "compose", "example", mode, "docker-compose.yml")
	return copyFile(src, filepath.Join(dir, "docker-compose.yml"))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

// writeGenerated 将 gen 中各 mode 的 docker-compose.yml 与 .env 写入 outBase/<mode>/，供 cmdGen 与 cmdGenSplit 共用。
func writeGenerated(outBase string, gen *composegen.Generated, modes []string) error {
	for _, mode := range modes {
		dir := filepath.Join(outBase, mode)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		ymlPath := filepath.Join(dir, "docker-compose.yml")
		if err := os.WriteFile(ymlPath, gen.Composes[mode], 0644); err != nil {
			return fmt.Errorf("write %s: %w", ymlPath, err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".env"), gen.Env, 0644); err != nil {
			return fmt.Errorf("write .env: %w", err)
		}
	}
	return nil
}

// generateRequest 为 /api/generate 的请求体。
type generateRequest struct {
	Modes       []string               `json:"modes"`
	EnvOverride string                 `json:"envOverride"`
	Options     *composeGenOptionsJSON `json:"options"`
}

// composeGenOptionsJSON 与 composegen.Options 对应，用于 JSON 解析。
type composeGenOptionsJSON struct {
	HealthCheck            *bool             `json:"healthCheck"`
	HealthCheckInterval    string            `json:"healthCheckInterval"`
	HealthCheckStartPeriod string            `json:"healthCheckStartPeriod"`
	TraefikNetwork         *bool             `json:"traefikNetwork"`
	TraefikNetworkName     string            `json:"traefikNetworkName"`
	ExposePorts            *bool             `json:"exposePorts"`
	PortHerald             string            `json:"portHerald"`
	PortWarden             string            `json:"portWarden"`
	PortHeraldRedis        string            `json:"portHeraldRedis"`
	ContainerNamePrefix    string            `json:"containerNamePrefix"`
	EnvOverrides           map[string]string `json:"envOverrides"`
	UseNamedVolume         *bool             `json:"useNamedVolume"`
	HeraldRedisDataPath    string            `json:"heraldRedisDataPath"`
	WardenRedisDataPath    string            `json:"wardenRedisDataPath"`
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
	return opts
}

func cmdServe() error {
	root := projectRoot()
	pagePath := filepath.Join(root, pageYAMLPath)
	page, err := loadPageData(pagePath)
	if err != nil {
		// Fallback: try config/page.yaml relative to current working directory
		if cwd, e := os.Getwd(); e == nil {
			fallback := filepath.Join(cwd, pageYAMLPath)
			page, err = loadPageData(fallback)
		}
		if err != nil {
			return fmt.Errorf("load page config (tried %s and ./%s): %w", pagePath, pageYAMLPath, err)
		}
	}
	tmpl, err := template.ParseFS(staticFS, "static/index.html.tmpl")
	if err != nil {
		return fmt.Errorf("parse index template: %w", err)
	}
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static sub FS: %w", err)
	}
	cacheStatic := "public, max-age=3600"
	staticHandler := cacheControlHandler(cacheStatic, http.FileServer(http.FS(subFS)))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, page); err != nil {
			fmt.Fprintf(os.Stderr, "template execute: %v\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	})
	mux.Handle("/static/", http.StripPrefix("/static", staticHandler))
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
		gen, err := composegen.Generate(full, req.Modes, req.EnvOverride, opts)
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
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

func findCommand(name string) *command {
	list := getCommands()
	for i := range list {
		if list[i].name == name {
			return &list[i]
		}
	}
	return nil
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	_ = fs.String("f", "", "compose file path")
	_ = fs.String("preset", "", "preset name from config/presets.json")
	_ = fs.String("o", "build", "output directory for gen command (default: build)")
	_ = fs.String("port", "8085", "port for serve command (default: 8085)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	args := fs.Args()
	cmdName := "help"
	if len(args) > 0 {
		cmdName = strings.TrimSpace(args[0])
	}

	// 只读一次 presets，用于 default 与 --preset
	presetPath := filepath.Join(projectRoot(), presetsPath)
	presets, _ := loadPresets(presetPath)
	defaultPath := fallbackCompose
	if presets != nil {
		if p := strings.TrimSpace(presets["default"]); p != "" {
			defaultPath = p
		}
	}

	// 解析 compose 路径：-f > --preset > COMPOSE_FILE > default
	resolvedComposeFile = configutil.ResolveString(fs, "f", "COMPOSE_FILE", defaultPath, true)
	if !flagutil.HasFlag(fs, "f") && flagutil.HasFlag(fs, "preset") {
		pname := strings.TrimSpace(flagutil.GetString(fs, "preset", ""))
		if pname != "" {
			if presets == nil {
				fmt.Fprintf(os.Stderr, "Failed to load presets from %s\n", presetPath)
				os.Exit(1)
			}
			if p, ok := presets[pname]; ok && strings.TrimSpace(p) != "" {
				resolvedComposeFile = strings.TrimSpace(p)
			} else {
				fmt.Fprintf(os.Stderr, "Unknown preset: %q\n", pname)
				os.Exit(1)
			}
		}
	}
	if resolvedComposeFile == "" {
		resolvedComposeFile = defaultPath
	}

	if cmdName == "gen" || cmdName == "gen-split" {
		genOutDir = strings.TrimSpace(configutil.ResolveString(fs, "o", "GEN_OUT_DIR", buildDirRelative, true))
	}
	if cmdName == "gen" && len(args) > 1 {
		genModeArg = strings.TrimSpace(args[1])
	}
	if cmdName == "serve" {
		servePort = strings.TrimSpace(configutil.ResolveString(fs, "port", "SERVE_PORT", "8085", true))
		if servePort == "" {
			servePort = "8085"
		}
	}

	c := findCommand(cmdName)
	if c == nil {
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n", cmdName)
		fmt.Fprintf(os.Stderr, "Run %s help for usage.\n", os.Args[0])
		os.Exit(1)
	}

	if err := c.fn(); err != nil {
		os.Exit(1)
	}
}
