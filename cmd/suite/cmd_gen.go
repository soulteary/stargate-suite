// Package main: gen command and /api/generate request types.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/the-gate/internal/composegen"
)

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

type generateRequest struct {
	Modes       []string               `json:"modes"`
	EnvOverride string                 `json:"envOverride"`
	Options     *composeGenOptionsJSON `json:"options"`
}

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
