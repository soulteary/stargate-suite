// Package main: component manifest loading. config/components.yaml is the single
// source of truth for component versions, images, ports and health paths (M-01).
// This helper loads it from the active asset FS (embedded or --config-dir
// override) and feeds container ports into composegen so generation no longer
// hardcodes them.
package main

import (
	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/contract"
)

// loadManifest reads config/components.yaml from the active asset FS.
func loadManifest() (*contract.Manifest, error) {
	return contract.LoadManifest(assetFS(), contract.ManifestPath)
}

// applyManifestToComposegen loads the manifest and pushes its container ports
// into composegen. Failures are non-fatal: composegen keeps its built-in
// defaults (which match the manifest) so generation still works.
func applyManifestToComposegen() {
	m, err := loadManifest()
	if err != nil || m == nil {
		return
	}
	ports := make(map[string]int, len(m.Components))
	for name, c := range m.Components {
		if c.ContainerPort > 0 {
			ports[name] = c.ContainerPort
		}
	}
	composegen.SetContainerPorts(ports)
}
