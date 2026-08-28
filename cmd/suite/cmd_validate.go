// Package main: validate command — check that page config and merged config load without error, plus optional consistency checks.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/policy"
)

// knownScenarioOptionKeys 返回 scenarios.json options 中受支持的键（与 scenarioOptionSetters 一致）。
func knownScenarioOptionKeys() map[string]bool {
	m := make(map[string]bool)
	for k := range scenarioOptionSetters {
		m[k] = true
	}
	return m
}

func cmdValidate() error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	profileName := fs.String("profile", "", "validate against a deployment profile: development|test|production")
	strict := fs.Bool("strict", false, "treat profile policy violations as hard errors (implied by test/production)")
	var sets stringSliceFlag
	fs.Var(&sets, "set", "override an env value as KEY=VALUE (repeatable); also read from process env for known secret keys")
	if err := fs.Parse(cmdArgs); err != nil {
		return err
	}

	page, err := loadPageData(pageYAMLPath)
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	if page != nil {
		hasPortsType := false
		for _, sec := range page.ConfigSections {
			for _, opt := range sec.Options {
				if opt.Type == "ports" {
					hasPortsType = true
					break
				}
			}
		}
		if hasPortsType && len(page.Ports) == 0 {
			fmt.Fprintf(os.Stderr, "warning: config has \"ports\" option but config/ports.yaml is missing or empty; port table will be empty\n")
		}
	}

	// 一致性：canonical compose 与 env-meta
	meta, err := composegen.LoadEnvMetaFS(assetFS(), "config/env-meta.yaml")
	if err != nil {
		return fmt.Errorf("env-meta: %w", err)
	}
	if meta != nil {
		full, err := composegen.LoadComposeFS(assetFS(), canonicalCompose)
		if err != nil {
			return fmt.Errorf("canonical compose: %w", err)
		}
		vars := composegen.ExtractEnvVars(full)
		orderSet := make(map[string]bool)
		for _, k := range meta.OrderKeys() {
			orderSet[k] = true
		}
		for k := range meta.Vars {
			orderSet[k] = true
		}
		for k := range vars {
			if !orderSet[k] {
				fmt.Fprintf(os.Stderr, "warning: canonical compose env %q not in env-meta (add to config/env-meta.yaml)\n", k)
			}
		}
	}

	// 一致性：scenarios.json options 键集合
	b, err := readAsset("config/scenarios.json")
	if err == nil {
		var scenes map[string]struct {
			Options map[string]interface{} `json:"options"`
		}
		if err := json.Unmarshal(b, &scenes); err == nil {
			known := knownScenarioOptionKeys()
			for id, scene := range scenes {
				for optKey := range scene.Options {
					if !known[optKey] {
						fmt.Fprintf(os.Stderr, "warning: scenario %q has unknown option %q (add to scenarioOptionSetters in cmd_gen.go)\n", id, optKey)
					}
				}
			}
		}
	}

	// 部署 Profile 策略校验（可选）：与 CLI generate、Web UI 共用同一 policy 模型。
	// test/production 为 strict，违规即为硬错误；production 不可被 --strict=false 绕过。
	if *profileName != "" {
		prof, err := resolveProfile(*profileName)
		if err != nil {
			return err
		}
		findings := validateForProfile(prof, nil, collectUserEnv(sets))
		for _, f := range findings {
			fmt.Fprintln(os.Stderr, "  "+f.String())
		}
		// production 始终 strict；显式 --strict 亦提升为严格。
		hardFail := policy.HasErrors(findings)
		if hardFail {
			return fmt.Errorf("profile %q validation failed: %d policy violation(s)", prof.Name, countErrors(findings))
		}
		if *strict && len(findings) > 0 {
			return fmt.Errorf("profile %q validation produced %d finding(s) under --strict", prof.Name, len(findings))
		}
		fmt.Printf("profile %q OK\n", prof.Name)
		return nil
	}

	fmt.Println("config OK")
	return nil
}

func countErrors(findings []policy.Finding) int {
	n := 0
	for _, f := range findings {
		if f.IsError {
			n++
		}
	}
	return n
}
