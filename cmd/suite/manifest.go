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

// applyManifestToComposegen loads the authoritative manifest and pushes its
// container ports into composegen. Invalid overrides are fatal so generation
// cannot silently fall back to stale built-in values.
func applyManifestToComposegen() error {
	m, err := loadManifest()
	if err != nil {
		return err
	}
	ports := make(map[string]int, len(m.Components))
	for name, c := range m.Components {
		ports[name] = c.ContainerPort
	}
	composegen.SetContainerPorts(ports)
	return nil
}
