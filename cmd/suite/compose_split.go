// Package main: 从 canonical compose 生成三分开 compose 到 build 目录（与 gen 共用 composegen）。
package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	modes := []string{"traefik-herald", "traefik-warden", "traefik-stargate"}
	gen, err := composegen.Generate(full, modes, "", nil)
	if err != nil {
		return err
	}
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
		fmt.Printf("Generated %s\n", ymlPath)
	}
	fmt.Printf("gen-split: %s -> build/traefik-herald, traefik-warden, traefik-stargate\n", canonicalCompose)
	return nil
}
