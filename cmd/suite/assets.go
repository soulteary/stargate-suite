// Package main: asset resolution. Config and the canonical compose file are
// embedded in the binary (see the module-root assets package) so that
// serve/validate/generate run without the repository source tree. An optional
// --config-dir flag overlays an on-disk config directory over the embedded
// assets, preserving the ability to customize configuration externally.
package main

import (
	"io/fs"
	"os"
	"path"
	"strings"

	assets "github.com/soulteary/the-gate"
)

// configDirOverride is the value of --config-dir; empty means use embedded assets only.
var configDirOverride string

// assetFS returns the filesystem used to read config/* and compose/canonical/*.
// Paths are always the repository-relative, slash-separated form
// (e.g. "config/page.yaml", "compose/canonical/docker-compose.yml").
//
//   - Without --config-dir: the embedded FS.
//   - With --config-dir=DIR: reads under "config/" are served from DIR first
//     (DIR mirrors the contents of the repo's config/ directory), falling back
//     to embedded assets for anything not present on disk (e.g. compose/).
func assetFS() fs.FS {
	if strings.TrimSpace(configDirOverride) == "" {
		return assets.Embedded()
	}
	return overlayFS{
		override: os.DirFS(configDirOverride),
		base:     assets.Embedded(),
	}
}

// overlayFS serves entries under the "config/" prefix from an on-disk override
// directory when present, otherwise falls back to the embedded base FS. The
// override directory is treated as the content of "config/", so a request for
// "config/page.yaml" maps to "<override>/page.yaml".
type overlayFS struct {
	override fs.FS
	base     fs.FS
}

const configPrefix = "config/"

func (o overlayFS) Open(name string) (fs.File, error) {
	if rel, ok := strings.CutPrefix(name, configPrefix); ok {
		// Directory root "config" itself: fall through to base for listing.
		if rel != "" {
			if f, err := o.override.Open(rel); err == nil {
				return f, nil
			}
		}
	}
	return o.base.Open(name)
}

// readAsset reads a repository-relative asset path (slash-separated) from the
// active asset filesystem.
func readAsset(name string) ([]byte, error) {
	return fs.ReadFile(assetFS(), path.Clean(name))
}
