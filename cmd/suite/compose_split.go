// Package main: 从 canonical compose 生成三分开 compose 到 build 目录（与 gen 共用 composegen、env 与 opts）。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/the-gate/internal/composegen"
)

func cmdGenSplit() error {
	root := projectRoot()
	outDir := genOutDir
	if outDir == "" {
		outDir = buildDirRelative
	}
	outBase := filepath.Join(root, outDir)
	fullPath := filepath.Join(root, canonicalCompose)
	full, err := composegen.LoadCompose(fullPath)
	if err != nil {
		return fmt.Errorf("read canonical compose: %w", err)
	}
	envBody := ""
	if b, err := os.ReadFile(filepath.Join(root, ".env")); err == nil && len(b) > 0 {
		envBody = string(b)
	}
	if envBody == "" {
		envBody = composegen.DefaultEnvBody()
	}
	opts := genOptionsFromEnv()
	modes := []string{"traefik-herald", "traefik-warden", "traefik-stargate"}
	gen, err := composegen.Generate(full, modes, envBody, opts)
	if err != nil {
		return err
	}
	if err := writeGenerated(outBase, gen, modes); err != nil {
		return err
	}
	fmt.Printf("gen-split: %s -> %s\n", canonicalCompose, strings.Join(modes, ", "))
	return nil
}
