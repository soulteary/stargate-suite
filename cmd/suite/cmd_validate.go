// Package main: validate command — check that page config and merged config load without error, plus optional consistency checks.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/soulteary/the-gate/internal/composegen"
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

	fmt.Println("config OK")
	return nil
}
