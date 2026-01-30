// Package main 提供与 Makefile 等效的 CLI，用于 the-gate 集成测试项目的编排与测试。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/soulteary/cli-kit/configutil"
	"github.com/soulteary/cli-kit/flagutil"
)

const (
	presetsPath     = "config/presets.json"
	fallbackCompose = "compose/image/docker-compose.yml"
	defaultEnvBody  = `# Container Image Version Configuration

# Herald Service Image
HERALD_IMAGE=ghcr.io/soulteary/herald:v0.4.1

# Warden Service Image
WARDEN_IMAGE=ghcr.io/soulteary/warden:v0.8.0

# Stargate Service Image
STARGATE_IMAGE=ghcr.io/soulteary/stargate:v0.7.1

# Redis Image Version
HERALD_REDIS_IMAGE=redis:7-alpine
WARDEN_REDIS_IMAGE=redis:7-alpine
`
)

// resolvedComposeFile 由 main 在解析全局 flag 后设置，供使用默认 compose 的命令读取。
var resolvedComposeFile string

// genOutDir、genModeArg 由 main 在解析 gen 命令时设置，供 cmdGen 使用。
var genOutDir, genModeArg string

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

// getDefaultComposePath 从 config/presets.json 读取 default 预设，失败则返回 fallbackCompose。
func getDefaultComposePath() string {
	path := presetsPath
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = filepath.Join(wd, presetsPath)
		}
	}
	presets, err := loadPresets(path)
	if err != nil || presets == nil {
		return fallbackCompose
	}
	if p := strings.TrimSpace(presets["default"]); p != "" {
		return p
	}
	return fallbackCompose
}

func composeFile() string {
	if resolvedComposeFile != "" {
		return resolvedComposeFile
	}
	return fallbackCompose
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

func cmdUpBuild() error {
	return run("docker", "compose", "-f", "compose/build/docker-compose.yml", "up", "-d", "--build")
}

func cmdUpImage() error {
	return run("docker", "compose", "-f", "compose/image/docker-compose.yml", "up", "-d")
}

func cmdUpTraefik() error {
	return run("docker", "compose", "-f", "compose/traefik/docker-compose.yml", "up", "-d")
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
	return run("docker", "compose", "-f", "compose/traefik-herald/docker-compose.yml", "up", "-d")
}

func cmdUpTraefikWarden() error {
	return run("docker", "compose", "-f", "compose/traefik-warden/docker-compose.yml", "up", "-d")
}

func cmdUpTraefikStargate() error {
	return run("docker", "compose", "-f", "compose/traefik-stargate/docker-compose.yml", "up", "-d")
}

func cmdDown() error {
	return run("docker", "compose", "-f", composeFile(), "down")
}

func cmdDownBuild() error {
	return run("docker", "compose", "-f", "compose/build/docker-compose.yml", "down")
}

func cmdDownImage() error {
	return run("docker", "compose", "-f", "compose/image/docker-compose.yml", "down")
}

func cmdDownTraefik() error {
	return run("docker", "compose", "-f", "compose/traefik/docker-compose.yml", "down")
}

func cmdDownTraefikHerald() error {
	return run("docker", "compose", "-f", "compose/traefik-herald/docker-compose.yml", "down")
}

func cmdDownTraefikWarden() error {
	return run("docker", "compose", "-f", "compose/traefik-warden/docker-compose.yml", "down")
}

func cmdDownTraefikStargate() error {
	return run("docker", "compose", "-f", "compose/traefik-stargate/docker-compose.yml", "down")
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

func cmdTestWait() error {
	fmt.Println("Waiting for services to be ready (3s)...")
	time.Sleep(3 * time.Second)
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
		outDir = "build"
	}
	root := projectRoot()
	modeArg := strings.TrimSpace(genModeArg)
	if modeArg == "" {
		modeArg = "all"
	}
	modes := []string{}
	switch modeArg {
	case "image", "build":
		modes = []string{modeArg}
	case "traefik":
		// traefik 含三合一 + 三分开，输出 4 个子目录
		modes = []string{"traefik", "traefik-herald", "traefik-warden", "traefik-stargate"}
	case "", "all":
		modes = []string{"image", "build", "traefik", "traefik-herald", "traefik-warden", "traefik-stargate"}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode %q. Use: image, build, traefik, or all\n", modeArg)
		return fmt.Errorf("unknown gen mode: %s", modeArg)
	}
	envBody := defaultEnvBody
	if b, err := os.ReadFile(filepath.Join(root, ".env")); err == nil && len(b) > 0 {
		envBody = string(b)
	}
	for _, mode := range modes {
		if err := genMode(root, filepath.Join(root, outDir), mode, envBody); err != nil {
			return err
		}
	}
	fmt.Printf("Generated %s for mode(s): %s\n", outDir, strings.Join(modes, ", "))
	return nil
}

func genMode(projectRoot, outBase, mode, envBody string) error {
	dir := filepath.Join(outBase, mode)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(envBody), 0644); err != nil {
		return fmt.Errorf("write %s: %w", envPath, err)
	}
	switch mode {
	case "image":
		return copyFile(
			filepath.Join(projectRoot, "compose", "image", "docker-compose.yml"),
			filepath.Join(dir, "docker-compose.yml"),
		)
	case "build":
		return copyFile(
			filepath.Join(projectRoot, "compose", "build", "docker-compose.yml"),
			filepath.Join(dir, "docker-compose.yml"),
		)
	case "traefik":
		return copyFile(
			filepath.Join(projectRoot, "compose", "traefik", "docker-compose.yml"),
			filepath.Join(dir, "docker-compose.yml"),
		)
	case "traefik-herald":
		return copyFile(
			filepath.Join(projectRoot, "compose", "traefik-herald", "docker-compose.yml"),
			filepath.Join(dir, "docker-compose.yml"),
		)
	case "traefik-warden":
		return copyFile(
			filepath.Join(projectRoot, "compose", "traefik-warden", "docker-compose.yml"),
			filepath.Join(dir, "docker-compose.yml"),
		)
	case "traefik-stargate":
		return copyFile(
			filepath.Join(projectRoot, "compose", "traefik-stargate", "docker-compose.yml"),
			filepath.Join(dir, "docker-compose.yml"),
		)
	default:
		return fmt.Errorf("unsupported mode: %s", mode)
	}
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
	defaultPath := getDefaultComposePath()

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	_ = fs.String("f", "", "compose file path")
	_ = fs.String("preset", "", "preset name from config/presets.json")
	_ = fs.String("o", "build", "output directory for gen command (default: build)")

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

	// 解析 compose 路径：-f > --preset > COMPOSE_FILE > default
	resolvedComposeFile = configutil.ResolveString(fs, "f", "COMPOSE_FILE", defaultPath, true)
	if !flagutil.HasFlag(fs, "f") && flagutil.HasFlag(fs, "preset") {
		pname := flagutil.GetString(fs, "preset", "")
		pname = strings.TrimSpace(pname)
		if pname != "" {
			presetPath := filepath.Join(".", presetsPath)
			if wd, err := os.Getwd(); err == nil {
				presetPath = filepath.Join(wd, presetsPath)
			}
			presets, err := loadPresets(presetPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to load presets: %v\n", err)
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

	if cmdName == "gen" {
		genOutDir = strings.TrimSpace(configutil.ResolveString(fs, "o", "GEN_OUT_DIR", "build", true))
		if len(args) > 1 {
			genModeArg = strings.TrimSpace(args[1])
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
