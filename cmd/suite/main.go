// Package main 提供与 Makefile 等效的 CLI，用于 the-gate 集成测试项目的编排与测试。
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/soulteary/cli-kit/configutil"
	"github.com/soulteary/cli-kit/flagutil"
)

//go:embed static
var staticFS embed.FS

const (
	pageYAMLPath         = "config/page.yaml"
	presetsPath          = "config/presets.json"
	fallbackCompose      = "compose/example/image/docker-compose.yml"
	canonicalCompose     = "compose/canonical/docker-compose.yml"
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
	Providers      []pageService         `yaml:"providers"`
}

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
	ShowWhenOption string      `yaml:"showWhenOption"`
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
	NameKey  string        `yaml:"nameKey"` // 可选，用于 i18n 显示名称（如 providers）
	Open     bool          `yaml:"open"`
	Sections []pageSection `yaml:"sections"`
}

type pageSection struct {
	TitleKey string   `yaml:"titleKey"`
	EnvVars  []envVar `yaml:"envVars"`
}

type envVar struct {
	Env            string         `yaml:"env"`
	Type           string         `yaml:"type"`
	LabelKey       string         `yaml:"labelKey"`
	DescKey        string         `yaml:"descKey"`
	Default        interface{}    `yaml:"default"`
	Placeholder    string         `yaml:"placeholder"`
	Min            int            `yaml:"min"`
	Max            int            `yaml:"max"`
	Options        []selectOption `yaml:"options"`
	ShowWhenEnv    string         `yaml:"showWhenEnv"`
	ShowWhenOption string         `yaml:"showWhenOption"`
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
	Providers      []pageService                `yaml:"providers"`
}

var resolvedComposeFile string
var genOutDir, genModeArg string
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

	presetPath := filepath.Join(projectRoot(), presetsPath)
	presets, _ := loadPresets(presetPath)
	defaultPath := fallbackCompose
	if presets != nil {
		if p := strings.TrimSpace(presets["default"]); p != "" {
			defaultPath = p
		}
	}

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
