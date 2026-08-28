// Package composegen: container-port lookup sourced from the component manifest.
//
// Historically the container ports (Herald :8082, Warden :8081, Stargate :80,
// …) were hardcoded inline in applyOptions, duplicating config/ports.yaml and
// compose/canonical. Per the upgrade rules (M-01) the single source of truth is
// config/components.yaml. The CLI loads the manifest at startup and calls
// SetContainerPorts so generation uses manifest-derived ports; the defaults
// below match the current (pre-migration) values so composegen still behaves
// correctly if the manifest is unavailable (e.g. in isolated unit tests).
package composegen

import (
	"strconv"
	"sync"
)

// defaultContainerPorts are the pre-migration container ports, kept in sync with
// config/components.yaml (the drift test enforces this). Used as a fallback when
// SetContainerPorts has not been called.
var defaultContainerPorts = map[string]int{
	"stargate":        80,
	"warden":          8081,
	"herald":          8082,
	"herald-dingtalk": 8083,
	"herald-totp":     8084,
	"herald-smtp":     8085,
	"herald-redis":    6379,
	"warden-redis":    6379,
}

var (
	containerPortsMu sync.RWMutex
	containerPorts   = defaultContainerPorts
)

// SetContainerPorts overrides the container-port lookup from the loaded
// component manifest. Passing an empty or nil map is a no-op (keeps defaults).
func SetContainerPorts(ports map[string]int) {
	if len(ports) == 0 {
		return
	}
	merged := make(map[string]int, len(defaultContainerPorts)+len(ports))
	for k, v := range defaultContainerPorts {
		merged[k] = v
	}
	for k, v := range ports {
		if v > 0 {
			merged[k] = v
		}
	}
	containerPortsMu.Lock()
	containerPorts = merged
	containerPortsMu.Unlock()
}

// containerPortStr returns the container port for a service as a string (e.g.
// "8082"), falling back to fallback when the service is unknown.
func containerPortStr(service, fallback string) string {
	containerPortsMu.RLock()
	p, ok := containerPorts[service]
	containerPortsMu.RUnlock()
	if !ok || p <= 0 {
		return fallback
	}
	return strconv.Itoa(p)
}
