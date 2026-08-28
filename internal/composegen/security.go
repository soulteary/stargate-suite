// Package composegen: network segmentation + container least-privilege
// transforms (PR 6, issues S-01/S-03). These are profile-driven security
// hardenings applied by both the CLI and Web UI through the shared Options,
// so neither path reimplements security policy.
package composegen

// Segmented internal networks. The flat the-gate-network is replaced by
// purpose-scoped networks so a compromised channel adapter cannot reach the
// warden data plane, and Redis is only reachable by its owning service.
//
//	edgeNetwork      : the external reverse-proxy (Traefik) ingress network.
//	authInternalNet  : Stargate <-> Warden / Herald control-plane calls.
//	wardenDataNet    : Warden  <-> its Redis (Warden data plane).
//	heraldDataNet    : Herald  <-> its Redis + channel adapters (Herald data plane).
const (
	authInternalNet = "auth-internal"
	wardenDataNet   = "warden-data"
	heraldDataNet   = "herald-data"
)

// serviceNetworks maps each logical service to the segmented internal networks
// it must join. The edge (Traefik) network is handled separately via the
// existing TraefikNetwork option so external ingress stays configurable.
var serviceNetworks = map[string][]string{
	"stargate":        {authInternalNet},
	"warden":          {authInternalNet, wardenDataNet},
	"warden-redis":    {wardenDataNet},
	"herald":          {authInternalNet, heraldDataNet},
	"herald-redis":    {heraldDataNet},
	"herald-totp":     {authInternalNet, heraldDataNet},
	"herald-dingtalk": {heraldDataNet},
	"herald-smtp":     {heraldDataNet},
	"owlmail":         {heraldDataNet},
	"stargate-redis":  {authInternalNet},
	// protected-service only ever talks to Traefik (edge); no internal net.
	"protected-service": {},
}

// segmentedNetworksUsed returns the set of internal networks actually needed by
// the services present in svcs, so we only declare networks that are in use
// (docker compose config rejects services referencing undeclared networks and
// warns on unused ones).
func segmentedNetworksUsed(svcs map[string]interface{}) map[string]bool {
	used := make(map[string]bool)
	for name := range svcs {
		for _, n := range serviceNetworks[name] {
			used[n] = true
		}
	}
	return used
}

// applyNetworkSegmentation rewrites service network membership and the top-level
// networks map onto the segmented scheme. It preserves the edge (Traefik)
// network membership already present on a service (added by the Traefik option),
// replacing only the flat the-gate-network entries.
func applyNetworkSegmentation(out map[string]interface{}, opts *Options) {
	if opts == nil || !opts.NetworkSegmentation {
		return
	}
	svcs, _ := out["services"].(map[string]interface{})
	if svcs == nil {
		return
	}
	edge := "traefik"
	if opts.TraefikNetworkName != "" {
		edge = opts.TraefikNetworkName
	}

	for name, s := range svcs {
		svc, _ := s.(map[string]interface{})
		if svc == nil {
			continue
		}
		// Detect whether the service currently sits on the edge/Traefik network
		// so we keep external ingress after replacing the flat network.
		onEdge := false
		if nl, ok := svc["networks"].([]interface{}); ok {
			for _, v := range nl {
				if str, _ := v.(string); str == edge || str == "traefik" {
					onEdge = true
				}
			}
		}
		var nets []interface{}
		if onEdge {
			nets = append(nets, edge)
		}
		for _, n := range serviceNetworks[name] {
			nets = append(nets, n)
		}
		if len(nets) == 0 {
			// Fallback: a service with no mapping keeps auth-internal so it is
			// not orphaned (should not happen for known services).
			nets = append(nets, authInternalNet)
		}
		svc["networks"] = nets
	}

	// Rebuild the top-level networks map: keep whatever edge/external network
	// the caller declared, then add the segmented internal bridges in use. If
	// the flat the-gate-network was declared external (split modes join a
	// pre-created shared network), the segmented internal networks are external
	// too so separately-launched compose files share them.
	external := false
	newNets := make(map[string]interface{})
	if existing, ok := out["networks"].(map[string]interface{}); ok {
		for k, v := range existing {
			if k == "the-gate-network" {
				if m, ok := v.(map[string]interface{}); ok {
					if ext, _ := m["external"].(bool); ext {
						external = true
					}
				}
				continue // replaced by segmented networks
			}
			newNets[k] = v
		}
	}
	used := segmentedNetworksUsed(svcs)
	for _, n := range []string{authInternalNet, wardenDataNet, heraldDataNet} {
		if used[n] {
			if _, taken := newNets[n]; !taken {
				if external {
					newNets[n] = map[string]interface{}{"external": true}
				} else {
					newNets[n] = map[string]interface{}{"driver": "bridge"}
				}
			}
		}
	}
	out["networks"] = newNets
}

// applyLeastPrivilege adds least-privilege container hardening to applicable
// services: drop all Linux capabilities, disallow privilege escalation, and
// (when ReadOnlyRootFS) mount a read-only root filesystem with a writable
// tmpfs for /tmp. Redis keeps its named data volume writable via the mount.
func applyLeastPrivilege(out map[string]interface{}, opts *Options) {
	if opts == nil || !opts.LeastPrivilege {
		return
	}
	svcs, _ := out["services"].(map[string]interface{})
	if svcs == nil {
		return
	}
	for name, s := range svcs {
		svc, _ := s.(map[string]interface{})
		if svc == nil {
			continue
		}
		svc["cap_drop"] = []interface{}{"ALL"}
		svc["security_opt"] = []interface{}{"no-new-privileges:true"}
		if opts.ReadOnlyRootFS {
			svc["read_only"] = true
			svc["tmpfs"] = tmpfsFor(name)
		}
	}
}

// tmpfsFor returns the writable tmpfs mounts a read-only service needs. Redis
// persists to its /data volume (writable independent of read_only), and the Go
// services (Stargate/Warden/Herald and adapters) only write scratch to /tmp, so
// a writable /tmp is sufficient for every current service.
func tmpfsFor(_ string) []interface{} {
	return []interface{}{"/tmp"}
}
