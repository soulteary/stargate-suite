// Package main: validate command — check that page config and merged config load without error.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func cmdValidate() error {
	root := projectRoot()
	pagePath := filepath.Join(root, pageYAMLPath)
	_, err := loadPageData(pagePath)
	if err != nil {
		if cwd, e := os.Getwd(); e == nil {
			fallback := filepath.Join(cwd, pageYAMLPath)
			_, err = loadPageData(fallback)
		}
	}
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	fmt.Println("config OK")
	return nil
}
