// Package main 提供 Web UI 与 compose 生成（help、validate、serve；配置生成仅通过网页）。
package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/cli-kit/configutil"
)

//go:embed static
var staticFS embed.FS

const (
	pageYAMLPath         = "config/page.yaml"
	canonicalCompose     = "compose/canonical/docker-compose.yml"
	maxGenerateBodyBytes = 1 << 20 // 1MB for /api/generate request body
)

// pageData 与 config/page.yaml 对应，用于渲染 index 模板。
type pageData struct {
	I18N           template.JS           `json:"-"`
	Scenarios      template.JS           `json:"-"`
	Title          string                `yaml:"-"`
	Lang           string                `yaml:"-"`
	Page           string                `yaml:"-"` // "entry", "wizard-1".."wizard-5", "keys", "import", "review"
	PageContent    string                `yaml:"-"` // template name for layout: "content-entry", "content-wizard-1", etc.
	Session        *SessionData          `yaml:"-"` // nil or wizard state for pre-fill
	Modes          []pageMode            `yaml:"modes"`
	ConfigSections []configOptionSection `yaml:"configSections"`
	Services       []pageService         `yaml:"services"`
	Providers      []pageService         `yaml:"providers"`
	KeysStepVars   []envVar              `yaml:"-"` // 从 config/keys-step.yaml 加载
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
	Type           string         `yaml:"type"`
	Id             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	EnvName        string         `yaml:"envName"`
	LabelKey       string         `yaml:"labelKey"`
	DescKey        string         `yaml:"descKey"`
	PlaceholderKey string         `yaml:"placeholderKey"`
	Placeholder    string         `yaml:"placeholder"`
	Default        interface{}    `yaml:"default"`
	TitleKey       string         `yaml:"titleKey"`
	Value          string         `yaml:"value"`
	Options        []selectOption `yaml:"options"`
	Paths          []redisPath    `yaml:"paths"`
	ShowWhenOption string         `yaml:"showWhenOption"`
	ShowWhenEnv    string         `yaml:"showWhenEnv"`
	FullRow        bool           `yaml:"fullRow"`
	Min            int            `yaml:"min"`
	Max            int            `yaml:"max"`
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

// keysStepYAML 对应 config/keys-step.yaml
type keysStepYAML struct {
	KeysStepVars []envVar `yaml:"keysStepVars"`
}

var servePort string

type command struct {
	name, desc string
	fn         func() error
}

var commands []command

func getCommands() []command {
	if len(commands) == 0 {
		commands = []command{
			{"help", "Show help information", cmdHelp},
			{"validate", "Validate that page config and merged config load without error", cmdValidate},
			{"serve", "Start web UI for compose generation (default :8085)", cmdServe},
		}
	}
	return commands
}

func cmdHelp() error {
	fmt.Println("stargate-suite — Web UI for compose generation")
	fmt.Println()
	fmt.Println("Available commands:")
	for _, c := range getCommands() {
		fmt.Printf("  %-22s %s\n", c.name, c.desc)
	}
	fmt.Println()
	fmt.Println("Compose generation is done in the Web UI (make serve). E2E: scripts/run-e2e.sh. Service lifecycle: Makefile (make up, make down) or docker compose.")
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
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
