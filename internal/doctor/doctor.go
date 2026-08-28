// Package doctor provides read-only diagnostics for a generated deployment. It
// never mutates state: it parses a docker-compose.yml, cross-checks the images
// against the component manifest (config/components.yaml — the single source of
// truth, M-01), reports host-published ports and whether they are already in
// use locally, lists declared networks, and — when explicitly asked — probes
// liveness/readiness endpoints.
//
// The output is a structured Report with a stable set of Check codes so the CLI
// can render it as text or JSON (`doctor --json`) and CI/Cursor can parse it.
// Doctor is intentionally dependency-light (stdlib + the compose parser +
// manifest loader) so it runs anywhere the CLI runs, without Docker.
package doctor

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/soulteary/stargate-suite/internal/composegen"
	"github.com/soulteary/stargate-suite/internal/contract"
)

// Status is the severity of a single diagnostic check.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Stable check codes. Grouped by the diagnostic they belong to so callers can
// assert on them without matching human-readable messages.
const (
	CodeComposeParse    = "COMPOSE_PARSE"
	CodeComposeServices = "COMPOSE_SERVICES"
	CodeImageManifest   = "IMAGE_MANIFEST_MATCH"
	CodePortPublished   = "PORT_PUBLISHED"
	CodePortInUse       = "PORT_IN_USE"
	CodeNetwork         = "NETWORK_DECLARED"
	CodeHealthReachable = "HEALTH_REACHABLE"
	CodeReadinessProbe  = "READINESS_PROBE"
	CodeManifestLoad    = "MANIFEST_LOAD"
)

// Check is a single diagnostic result. Detail carries structured context (e.g.
// service name, image, port) so JSON consumers do not have to parse Message.
type Check struct {
	Code    string            `json:"code"`
	Status  Status            `json:"status"`
	Message string            `json:"message"`
	Detail  map[string]string `json:"detail,omitempty"`
}

// Report is the full diagnostic result for one compose file.
type Report struct {
	Compose string  `json:"compose"`
	Checks  []Check `json:"checks"`
	// Summary counts by status for a quick machine-readable verdict.
	OK   int `json:"ok"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

// HasFailures reports whether any check failed (drives the CLI exit code).
func (r *Report) HasFailures() bool { return r.Fail > 0 }

// add appends a check and updates the summary counters.
func (r *Report) add(c Check) {
	r.Checks = append(r.Checks, c)
	switch c.Status {
	case StatusOK:
		r.OK++
	case StatusWarn:
		r.Warn++
	case StatusFail:
		r.Fail++
	}
}

// Options controls which diagnostics run.
type Options struct {
	// ComposePath is the on-disk path to the docker-compose.yml under diagnosis
	// (used only for the report label).
	ComposePath string
	// ComposeBytes is the raw compose content.
	ComposeBytes []byte
	// ManifestFS + ManifestPath locate config/components.yaml (single source of
	// truth). When nil, image/manifest cross-checks are skipped with a warning.
	ManifestFS   fs.FS
	ManifestPath string
	// Probe enables active liveness/readiness HTTP probes against published
	// ports on the loopback host. Off by default: doctor is read-only and must
	// not require running services to produce a useful report.
	Probe bool
	// ProbeTimeout bounds each HTTP probe.
	ProbeTimeout time.Duration
	// dialTimeout bounds the port-in-use TCP check (kept small).
	dialTimeout time.Duration
}

// Run executes the read-only diagnostics described by opts and returns a Report.
// It never returns an error for "unhealthy" findings — those are Checks with
// StatusWarn/StatusFail. A returned error means doctor itself could not run
// (e.g. compose could not be parsed at all).
func Run(opts Options) (*Report, error) {
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = 3 * time.Second
	}
	if opts.dialTimeout <= 0 {
		opts.dialTimeout = 300 * time.Millisecond
	}
	rep := &Report{Compose: opts.ComposePath}

	parsed, err := composegen.ParseCompose(opts.ComposeBytes)
	if err != nil {
		rep.add(Check{Code: CodeComposeParse, Status: StatusFail, Message: fmt.Sprintf("compose parse failed: %v", err)})
		// A compose we cannot parse yields no further useful checks, but we
		// still return a Report (with the failure recorded) rather than an
		// error, so the CLI can render it uniformly.
		return rep, nil
	}
	rep.add(Check{Code: CodeComposeParse, Status: StatusOK, Message: "compose parsed"})

	services := serviceMap(parsed)
	if len(services) == 0 {
		rep.add(Check{Code: CodeComposeServices, Status: StatusFail, Message: "compose declares no services"})
		return rep, nil
	}
	rep.add(Check{Code: CodeComposeServices, Status: StatusOK, Message: fmt.Sprintf("%d service(s) declared", len(services)), Detail: map[string]string{"count": strconv.Itoa(len(services))}})

	var manifest *contract.Manifest
	if opts.ManifestFS != nil {
		mp := opts.ManifestPath
		if mp == "" {
			mp = contract.ManifestPath
		}
		manifest, err = contract.LoadManifest(opts.ManifestFS, mp)
		if err != nil {
			rep.add(Check{Code: CodeManifestLoad, Status: StatusWarn, Message: fmt.Sprintf("could not load component manifest: %v", err)})
		}
	} else {
		rep.add(Check{Code: CodeManifestLoad, Status: StatusWarn, Message: "no component manifest provided; image cross-checks skipped"})
	}

	checkImages(rep, services, manifest)
	published := checkPorts(rep, services, opts.dialTimeout)
	checkNetworks(rep, parsed)

	if opts.Probe {
		probeHealth(rep, services, published, manifest, opts.ProbeTimeout)
	}
	return rep, nil
}

// serviceMap returns the services block of a parsed compose (name -> raw map).
func serviceMap(parsed map[string]interface{}) map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{})
	svc, ok := parsed["services"].(map[string]interface{})
	if !ok {
		return out
	}
	for name, v := range svc {
		if m, ok := v.(map[string]interface{}); ok {
			out[name] = m
		}
	}
	return out
}

// sortedNames returns service names in stable order for deterministic reports.
func sortedNames(services map[string]map[string]interface{}) []string {
	names := make([]string, 0, len(services))
	for n := range services {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// checkImages cross-checks each service image against the manifest. A service
// whose image does not match the manifest ref is a warning (the compose may be
// intentionally pinned to a local build), while a service that references a
// suite component at a DIFFERENT version than the manifest is flagged so drift
// is visible.
func checkImages(rep *Report, services map[string]map[string]interface{}, manifest *contract.Manifest) {
	for _, name := range sortedNames(services) {
		image, _ := services[name]["image"].(string)
		image = strings.TrimSpace(image)
		if image == "" {
			// build-from-source services have no image; not an error.
			continue
		}
		if manifest == nil {
			continue
		}
		comp, ok := manifest.Component(name)
		if !ok {
			continue
		}
		ref := comp.Ref()
		if ref == "" {
			continue
		}
		resolved := resolveComposeImage(image)
		detail := map[string]string{"service": name, "image": image, "resolved": resolved, "manifest": ref}
		if resolved == ref {
			rep.add(Check{Code: CodeImageManifest, Status: StatusOK, Message: fmt.Sprintf("%s image matches manifest (%s)", name, ref), Detail: detail})
		} else {
			rep.add(Check{Code: CodeImageManifest, Status: StatusWarn, Message: fmt.Sprintf("%s image %q differs from manifest %q", name, resolved, ref), Detail: detail})
		}
	}
}

// resolveComposeImage resolves a compose image field to a concrete ref for
// comparison. Compose interpolates `${VAR:-default}` / `${VAR:default}` at
// deploy time; for a static manifest cross-check we use the default (what a
// bare `docker compose up` with no override would pull). A plain image is
// returned unchanged.
func resolveComposeImage(image string) string {
	image = strings.TrimSpace(image)
	if !strings.HasPrefix(image, "${") || !strings.HasSuffix(image, "}") {
		return image
	}
	inner := image[2 : len(image)-1]
	// Compose default operators: `${VAR:-default}` (unset or empty) and
	// `${VAR-default}` (unset). Check the longer `:-` first so it isn't
	// mis-split by `-`. Other operators (`:?`, `:+`) have no static default.
	if i := strings.Index(inner, ":-"); i >= 0 {
		return strings.TrimSpace(inner[i+2:])
	}
	if i := strings.Index(inner, "-"); i >= 0 {
		return strings.TrimSpace(inner[i+1:])
	}
	// ${VAR} with no default → cannot resolve statically; return as-is.
	return image
}

// publishedPort records a host-published port for a service.
type publishedPort struct {
	service   string
	hostPort  int
	container string
}

// checkPorts reports each host-published port and whether it is already in use
// locally (a TCP connect on 127.0.0.1 succeeds). A port already in use is a
// warning: doctor cannot know whether it is this stack or an unrelated process.
func checkPorts(rep *Report, services map[string]map[string]interface{}, dialTimeout time.Duration) []publishedPort {
	var out []publishedPort
	for _, name := range sortedNames(services) {
		ports := extractPorts(services[name]["ports"])
		for _, p := range ports {
			detail := map[string]string{"service": name, "hostPort": strconv.Itoa(p.hostPort), "containerPort": p.container}
			rep.add(Check{Code: CodePortPublished, Status: StatusOK, Message: fmt.Sprintf("%s publishes host port %d -> %s", name, p.hostPort, p.container), Detail: detail})
			p.service = name
			out = append(out, p)
			if portInUse(p.hostPort, dialTimeout) {
				rep.add(Check{Code: CodePortInUse, Status: StatusWarn, Message: fmt.Sprintf("host port %d (%s) already in use", p.hostPort, name), Detail: detail})
			}
		}
	}
	return out
}

// extractPorts parses a compose "ports" value into host/container pairs. It
// accepts the short syntax ("8080:80", "127.0.0.1:8080:80", "80") and the long
// mapping syntax ({published: 8080, target: 80}).
func extractPorts(v interface{}) []publishedPort {
	var out []publishedPort
	list, ok := v.([]interface{})
	if !ok {
		return out
	}
	for _, item := range list {
		switch t := item.(type) {
		case string:
			if pp, ok := parseShortPort(t); ok {
				out = append(out, pp)
			}
		case map[string]interface{}:
			pub := fmt.Sprintf("%v", t["published"])
			tgt := fmt.Sprintf("%v", t["target"])
			if hp, err := strconv.Atoi(strings.TrimSpace(pub)); err == nil {
				out = append(out, publishedPort{hostPort: hp, container: strings.TrimSpace(tgt)})
			}
		}
	}
	return out
}

// parseShortPort parses "H:C", "IP:H:C" or bare "C" (no host publish → skipped).
func parseShortPort(s string) (publishedPort, bool) {
	s = strings.TrimSpace(s)
	// strip any /proto suffix.
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 1:
		// bare container port, not host-published.
		return publishedPort{}, false
	case 2:
		if hp, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			return publishedPort{hostPort: hp, container: strings.TrimSpace(parts[1])}, true
		}
	case 3:
		if hp, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			return publishedPort{hostPort: hp, container: strings.TrimSpace(parts[2])}, true
		}
	}
	return publishedPort{}, false
}

// portInUse reports whether a TCP connect to 127.0.0.1:port succeeds quickly.
func portInUse(port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// checkNetworks lists the top-level networks declared in the compose.
func checkNetworks(rep *Report, parsed map[string]interface{}) {
	nets, ok := parsed["networks"].(map[string]interface{})
	if !ok || len(nets) == 0 {
		rep.add(Check{Code: CodeNetwork, Status: StatusWarn, Message: "no top-level networks declared"})
		return
	}
	names := make([]string, 0, len(nets))
	for n := range nets {
		names = append(names, n)
	}
	sort.Strings(names)
	rep.add(Check{Code: CodeNetwork, Status: StatusOK, Message: fmt.Sprintf("networks: %s", strings.Join(names, ", ")), Detail: map[string]string{"networks": strings.Join(names, ",")}})
}

// probeHealth actively probes each service's liveness (and readiness, when the
// manifest declares one) endpoint on its published host port. Only runs when
// Options.Probe is true. Unreachable endpoints are warnings, not failures:
// doctor is a diagnostic aid, not a gate.
func probeHealth(rep *Report, services map[string]map[string]interface{}, published []publishedPort, manifest *contract.Manifest, timeout time.Duration) {
	if manifest == nil {
		return
	}
	// Map service -> first published host port for probing.
	hostPort := make(map[string]int)
	for _, p := range published {
		if _, seen := hostPort[p.service]; !seen {
			hostPort[p.service] = p.hostPort
		}
	}
	client := &http.Client{Timeout: timeout}
	for _, name := range sortedNames(services) {
		comp, ok := manifest.Component(name)
		if !ok {
			continue
		}
		port, ok := hostPort[name]
		if !ok {
			continue // not host-published; cannot probe from host.
		}
		if comp.LivenessPath != "" {
			probeOne(rep, client, CodeHealthReachable, name, port, comp.LivenessPath)
		}
		if comp.ReadinessPath != "" {
			probeOne(rep, client, CodeReadinessProbe, name, port, comp.ReadinessPath)
		}
	}
}

func probeOne(rep *Report, client *http.Client, code, name string, port int, urlPath string) {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, urlPath)
	detail := map[string]string{"service": name, "url": url}
	resp, err := client.Get(url)
	if err != nil {
		rep.add(Check{Code: code, Status: StatusWarn, Message: fmt.Sprintf("%s %s unreachable: %v", name, urlPath, err), Detail: detail})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	detail["status"] = strconv.Itoa(resp.StatusCode)
	// Readiness may legitimately be 503 when a dependency is down; that is a
	// warning (service up, not ready), never a doctor failure.
	switch {
	case resp.StatusCode == http.StatusOK:
		rep.add(Check{Code: code, Status: StatusOK, Message: fmt.Sprintf("%s %s → 200", name, urlPath), Detail: detail})
	case resp.StatusCode == http.StatusServiceUnavailable && code == CodeReadinessProbe:
		rep.add(Check{Code: code, Status: StatusWarn, Message: fmt.Sprintf("%s %s → 503 (not ready)", name, urlPath), Detail: detail})
	default:
		rep.add(Check{Code: code, Status: StatusWarn, Message: fmt.Sprintf("%s %s → %d", name, urlPath, resp.StatusCode), Detail: detail})
	}
}
