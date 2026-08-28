// Package main 提供 suite CLI（help、version、generate、validate、doctor、serve）
// 与 Web UI；CLI 与 Web UI 共用同一套 compose 生成与策略校验模型。
package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
)

//go:embed static
var staticFS embed.FS

const (
	pageYAMLPath         = "config/page.yaml"
	canonicalCompose     = "compose/canonical/docker-compose.yml"
	maxGenerateBodyBytes = 1 << 20 // 1MB for /api/generate request body
)

// resolveString applies the CLI configuration precedence used by suite:
// an explicitly supplied flag wins, then a non-empty environment value, then
// the default. Environment whitespace is trimmed; explicit flag values are
// returned exactly as parsed by flag.FlagSet.
func resolveString(fs *flag.FlagSet, flagName, envKey, defaultValue string) string {
	flagSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == flagName {
			flagSet = true
		}
	})
	if flagSet {
		if f := fs.Lookup(flagName); f != nil {
			return f.Value.String()
		}
	}
	if value, ok := os.LookupEnv(envKey); ok {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return defaultValue
}

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
	Ports          []portDef             `yaml:"-"` // 从 config/ports.yaml 加载，集中展示与配置
	PortValues     map[string]string     `yaml:"-"` // optionId -> 当前值，用于 Session 回填端口表
	Profiles       []pageProfile         `yaml:"-"` // 从 config/profiles.yaml 加载，供 UI 第一步选择部署 Profile
}

// pageProfile 是 config/profiles.yaml 中一个部署 Profile 的展示信息，供向导第一步选择。
type pageProfile struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Experimental bool   `json:"experimental"`
	Strict       bool   `json:"strict"`
}

// portDef 与 config/ports.yaml 单条一致，用于向导 step-2 端口表格展示与表单绑定。
type portDef struct {
	ServiceKey      string `yaml:"serviceKey"`
	OptionId        string `yaml:"optionId"`
	ContainerPort   string `yaml:"containerPort"`
	DefaultHostPort string `yaml:"defaultHostPort"`
	LabelKey        string `yaml:"labelKey"`
	DescKey         string `yaml:"descKey"`
	ShowWhenOption  string `yaml:"showWhenOption"`
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
	UncheckValue   string         `yaml:"uncheckValue"`   // 用于 checkbox 提交为 env 时，未勾选时提交的值
	DisablesOption string         `yaml:"disablesOption"` // 勾选此项时互斥禁用/取消另一个 option（前端联动）
}

// IsChecked 返回该 checkbox 在给定会话状态下是否应勾选。
// 优先读 Options[Id/Name]；env 型 checkbox（如 SESSION_STORAGE_ENABLED）会落入 EnvOverrides，一并回填。
func (o configOption) IsChecked(sess *SessionData) bool {
	if sess != nil {
		if sess.Options != nil {
			if v, ok := sess.Options[o.Id]; ok {
				return optionValueTruthy(v)
			}
			if o.Name != "" && o.Name != o.Id {
				if v, ok := sess.Options[o.Name]; ok {
					return optionValueTruthy(v)
				}
			}
		}
		if sess.EnvOverrides != nil {
			for _, key := range []string{o.EnvName, o.Name} {
				if key == "" {
					continue
				}
				if v, ok := sess.EnvOverrides[key]; ok {
					return optionValueTruthy(v)
				}
			}
		}
	}
	return truthyDefault(o.Default)
}

// optionValueTruthy 判断 session option 值（bool/string/其它）是否为真。
func optionValueTruthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "on"
	default:
		return false
	}
}

// truthyDefault 判断 YAML default（bool/string）是否为真。
func truthyDefault(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	default:
		return false
	}
}

func formatSessionScalar(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// SessionValue 返回该 option 在会话中记录的字符串值；无记录时返回空串，供模板回退到 Default。
func (o configOption) SessionValue(sess *SessionData) string {
	if sess == nil {
		return ""
	}
	if sess.Options != nil {
		if v, ok := sess.Options[o.Id]; ok {
			return formatSessionScalar(v)
		}
		if o.Name != "" {
			if v, ok := sess.Options[o.Name]; ok {
				return formatSessionScalar(v)
			}
		}
	}
	if sess.EnvOverrides != nil {
		for _, key := range []string{o.EnvName, o.Name} {
			if key == "" {
				continue
			}
			if v, ok := sess.EnvOverrides[key]; ok {
				return v
			}
		}
	}
	return ""
}

// applyUncheckEnvDefaults 对 ConfigSections 中声明了 uncheckValue 的 env，若 EnvOverrides 未显式提供则补上 uncheckValue。
// 用于向导/JSON 路径：未勾选的 checkbox 不会出现在 payload，需显式落 false（如 WARDEN_REDIS_ENABLED）。
// 调用方需确保仅在「表单/序列化已产生 EnvOverrides」时调用，避免 make gen（options:null）被改写 canonical 默认。
func applyUncheckEnvDefaults(env map[string]string, sections []configOptionSection) {
	if env == nil {
		return
	}
	for _, sec := range sections {
		for _, opt := range sec.Options {
			if opt.UncheckValue == "" || opt.EnvName == "" {
				continue
			}
			if _, ok := env[opt.EnvName]; !ok {
				env[opt.EnvName] = opt.UncheckValue
			}
		}
	}
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
	GenType        string         `yaml:"genType"` // 密钥自动生成类型：apiKey|hmacSecret|hmacKeys|aes32|password；为空表示仅手填
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

// cmdArgs holds CLI args after the subcommand name, for subcommands that parse
// their own flag sets (generate, validate).
var cmdArgs []string

type command struct {
	name, desc string
	fn         func() error
}

var commands []command

func getCommands() []command {
	if len(commands) == 0 {
		commands = []command{
			{"help", "Show help information", cmdHelp},
			{"version", "Show version, build metadata, and verified component combination", cmdVersion},
			{"generate", "Generate profile-aware compose + .env (--profile, --output, --modes, --json)", cmdGenerate},
			{"validate", "Validate config; with --profile [--strict] enforce deployment-profile policy", cmdValidate},
			{"doctor", "Read-only diagnostics for a generated compose (--compose, --json, --probe)", cmdDoctor},
			{"serve", "Start web UI for compose generation (loopback-only by default; --listen/--allow-remote/--token)", cmdServe},
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
	fmt.Println("CLI generate/validate/doctor share the same Go generation + policy model as the Web UI (make serve). E2E: scripts/run-e2e.sh. Service lifecycle: Makefile (make up, make down) or docker compose.")
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
	_ = fs.String("port", "8085", "port for serve command (default: 8085)")
	fs.StringVar(&configDirOverride, "config-dir", "", "override embedded config assets with an on-disk config directory (default: use embedded assets)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	configDirOverride = strings.TrimSpace(resolveString(fs, "config-dir", "CONFIG_DIR", ""))

	args := fs.Args()
	cmdName := "help"
	if len(args) > 0 {
		cmdName = strings.TrimSpace(args[0])
		cmdArgs = args[1:]
	}

	if cmdName == "serve" {
		servePort = strings.TrimSpace(resolveString(fs, "port", "SERVE_PORT", "8085"))
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

	// Generation, validation, and serving require the authoritative component
	// manifest. Keep help/version available with their embedded/fallback data,
	// and let doctor report a malformed manifest as a diagnostic warning.
	if commandRequiresManifest(cmdName) {
		if err := applyManifestToComposegen(); err != nil {
			fmt.Fprintf(os.Stderr, "component manifest: %v\n", err)
			os.Exit(1)
		}
	}

	if err := c.fn(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func commandRequiresManifest(name string) bool {
	switch name {
	case "generate", "validate", "serve":
		return true
	default:
		return false
	}
}
