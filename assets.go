// Package assets bundles the default runtime configuration and the canonical
// compose file into the binary via go:embed, so that `serve` / `validate` /
// `generate` work without the repository source tree being present on disk
// (fixes B-02). This file lives at the module root because go:embed cannot
// reference parent directories, and the assets (config/, compose/canonical/)
// are siblings of this file at the repository root.
//
// The embedded filesystem preserves the repository-root layout: entries are
// addressed as "config/..." and "compose/canonical/..." exactly as they are on
// disk. This keeps the relative paths used throughout the CLI identical whether
// assets are served from the embedded FS or from an on-disk override.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:config compose/canonical/docker-compose.yml
var embedded embed.FS

// Embedded returns the read-only filesystem containing the default config
// directory and the canonical compose file, rooted at the repository layout
// (e.g. "config/page.yaml", "compose/canonical/docker-compose.yml").
func Embedded() fs.FS {
	return embedded
}
