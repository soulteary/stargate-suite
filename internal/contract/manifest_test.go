package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// loadManifestFromRoot reads and parses config/components.yaml from the repo root.
func loadManifestFromRoot(t *testing.T, root string) *Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ManifestPath)))
	if err != nil {
		t.Fatalf("read %s: %v", ManifestPath, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("parse %s: %v", ManifestPath, err)
	}
	return m
}

func TestComponentLockMatchesManifestSnapshot(t *testing.T) {
	root := repoRoot(t)
	manifest := loadManifestFromRoot(t, root)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(LockPath)))
	if err != nil {
		t.Fatalf("read %s: %v", LockPath, err)
	}
	lock, err := ParseLock(data)
	if err != nil {
		t.Fatalf("parse %s: %v", LockPath, err)
	}
	if err := ValidateLock(manifest, lock, false); err != nil {
		t.Fatalf("component lock drift: %v", err)
	}
}

// TestVerifiedComboMatchesComponents prevents the CLI's advertised verified
// matrix from drifting away from the images that generation actually uses.
func TestVerifiedComboMatchesComponents(t *testing.T) {
	root := repoRoot(t)
	m := loadManifestFromRoot(t, root)
	for name, verified := range m.VerifiedCombo {
		component, ok := m.Component(name)
		if !ok {
			t.Errorf("verifiedCombo references missing component %q", name)
			continue
		}
		if strings.TrimPrefix(verified, "v") != strings.TrimPrefix(component.Version, "v") {
			t.Errorf("verifiedCombo %s=%q does not match component version %q", name, verified, component.Version)
		}
	}
}

// coreImageEnvVar maps each core component to the env var that carries its
// image reference in env-meta.yaml / .env.example / compose/canonical.
var coreImageEnvVar = map[string]string{
	"stargate":        "STARGATE_IMAGE",
	"warden":          "WARDEN_IMAGE",
	"herald":          "HERALD_IMAGE",
	"herald-totp":     "HERALD_TOTP_IMAGE",
	"herald-dingtalk": "HERALD_DINGTALK_IMAGE",
	"herald-smtp":     "HERALD_SMTP_IMAGE",
}

// TestManifestIsAuthoritativeForImages asserts every authoritative source
// declares the exact image:version registered in config/components.yaml. This
// is the single-source-of-truth guard for M-01: if someone bumps a version in
// env-meta.yaml / .env.example / compose/canonical without updating the
// manifest (or vice versa), this fails.
func TestManifestIsAuthoritativeForImages(t *testing.T) {
	root := repoRoot(t)
	m := loadManifestFromRoot(t, root)

	envMeta := readFile(t, root, "config/env-meta.yaml")
	envExample := readFile(t, root, ".env.example")
	canonical := readFile(t, root, "compose/canonical/docker-compose.yml")
	configSections := readFile(t, root, "config/config-sections.yaml")
	composegen := readFile(t, root, "internal/composegen/composegen.go")

	for comp, envVar := range coreImageEnvVar {
		c, ok := m.Component(comp)
		if !ok {
			t.Errorf("manifest missing component %q", comp)
			continue
		}
		ref := c.Ref()
		if ref == "" {
			t.Errorf("component %q has empty image/version in manifest", comp)
			continue
		}

		// env-meta.yaml: `KEY: { ..., default: "<image:version>" }`
		if !containsImageDefault(envMeta, envVar, ref) {
			t.Errorf("env-meta.yaml %s default does not match manifest %q for %q", envVar, ref, comp)
		}
		// .env.example: `KEY=<image:version>` or `# KEY=<image:version>`.
		// Only core images are guaranteed present; optional channels may be
		// omitted. If the key appears at all, it must match the manifest.
		if envHasKey(envExample, envVar) && !containsEnvAssignment(envExample, envVar, ref) {
			t.Errorf(".env.example %s does not match manifest %q for %q", envVar, ref, comp)
		}
		// compose/canonical: `image: ${KEY:-<image:version>}`
		if !containsComposeDefault(canonical, envVar, ref) {
			t.Errorf("compose/canonical %s default does not match manifest %q for %q", envVar, ref, comp)
		}
		// The Web wizard posts config-section defaults as explicit overrides, so
		// these values must be covered by the same drift guard.
		if strings.Contains(configSections, "envName: "+envVar) && !containsConfigSectionDefault(configSections, envVar, ref) {
			t.Errorf("config-sections.yaml %s default does not match manifest %q for %q", envVar, ref, comp)
		}
		// DefaultEnvBody(nil) remains a supported fallback for callers without
		// env metadata. Guard its core image assignments as well.
		if envHasKey(composegen, envVar) && !containsEnvAssignment(composegen, envVar, ref) {
			t.Errorf("composegen built-in %s does not match manifest %q for %q", envVar, ref, comp)
		}
	}
}

// TestManifestIsAuthoritativeForPorts asserts config/ports.yaml container ports
// match the manifest, and the canonical compose healthchecks hit the manifest's
// container port + liveness path.
func TestManifestIsAuthoritativeForPorts(t *testing.T) {
	root := repoRoot(t)
	m := loadManifestFromRoot(t, root)

	portsYAML := readFile(t, root, "config/ports.yaml")
	canonical := readFile(t, root, "compose/canonical/docker-compose.yml")

	for comp, c := range m.Components {
		if c.ContainerPort <= 0 {
			continue
		}
		portStr := itoa(c.ContainerPort)

		// ports.yaml declares `serviceKey: <comp>` then `containerPort: "<port>"`.
		if !containsPortEntry(portsYAML, comp, portStr) {
			t.Errorf("config/ports.yaml containerPort for %q does not match manifest %s", comp, portStr)
		}

		// Herald ships a built-in health checker because its minimal runtime
		// image intentionally has no HTTP client. Other components probe their
		// manifest-declared port and liveness path over loopback.
		if c.LivenessPath != "" {
			if comp == "herald" {
				want := `["CMD", "/bin/herald", "-healthcheck"]`
				if !regexp.MustCompile(regexp.QuoteMeta(want)).MatchString(canonical) {
					t.Errorf("compose/canonical has no built-in healthcheck %q for %q", want, comp)
				}
				continue
			}
			want := `http://(?:localhost|127\.0\.0\.1):` + regexp.QuoteMeta(portStr+c.LivenessPath)
			if !regexp.MustCompile(want).MatchString(canonical) {
				t.Errorf("compose/canonical has no loopback HTTP healthcheck for %q (manifest port %s path %s)",
					comp, portStr, c.LivenessPath)
			}
		}
	}
}

// TestManifestNoRehardcodedContainerPorts guards that composegen no longer
// inlines core container ports as string literals; they must derive from the
// manifest via containerPortStr. Catches accidental re-hardcoding (M-01).
func TestManifestNoRehardcodedContainerPorts(t *testing.T) {
	root := repoRoot(t)
	src := readFile(t, root, "internal/composegen/composegen.go")
	// Patterns that would indicate a re-hardcoded host:container mapping like
	// `hostPort + ":8082"`. The legitimate manifest path uses containerPortStr.
	bad := []string{`":8082"`, `":8081"`, `":8083"`, `":8085"`}
	for _, b := range bad {
		if regexp.MustCompile(`\+\s*` + regexp.QuoteMeta(b)).MatchString(src) {
			t.Errorf("composegen.go re-hardcodes container port %s; use containerPortStr sourced from the manifest", b)
		}
	}
}

// TestBuildArtifactsStalenessAdvisory is a NON-FAILING advisory. build/* is a
// gitignored generated artifact, not a source of truth, so drift there must not
// fail CI; instead we surface it so the human knows to regenerate. This is the
// deliberate choice over regenerating build/* in this PR: build/* is not
// committed, and regenerating it is a separate concern (deployment profiles,
// PR 5+). The test still demonstrates it can DETECT the known build/* drift
// (Stargate v0.9.2 / Warden v0.10.0 / Herald v0.6.1, herald-dingtalk:latest).
func TestBuildArtifactsStalenessAdvisory(t *testing.T) {
	root := repoRoot(t)
	m := loadManifestFromRoot(t, root)

	buildFiles := []string{
		"build/image/docker-compose.yml",
		"build/build/docker-compose.yml",
		"build/traefik/docker-compose.yml",
		"build/traefik-herald/docker-compose.yml",
		"build/traefik-warden/docker-compose.yml",
		"build/traefik-stargate/docker-compose.yml",
	}
	stale := 0
	for _, rel := range buildFiles {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue // gitignored / not generated yet — nothing to advise on.
		}
		content := string(b)
		for comp, envVar := range coreImageEnvVar {
			c, ok := m.Component(comp)
			if !ok || c.Ref() == "" {
				continue
			}
			// Detect the image line for this component regardless of the pinned tag.
			imgLine := regexp.MustCompile(`(?m)^\s*image:\s*(?:\$\{` + regexp.QuoteMeta(envVar) + `:-)?` +
				regexp.QuoteMeta(c.Image) + `:([^}\s]+)`)
			for _, mm := range imgLine.FindAllStringSubmatch(content, -1) {
				got := c.Image + ":" + mm[1]
				if got != c.Ref() {
					t.Logf("advisory: %s pins %q but manifest says %q (regenerate build/* from canonical)", rel, got, c.Ref())
					stale++
				}
			}
		}
	}
	if stale > 0 {
		t.Logf("advisory: %d stale image reference(s) in build/*; run the generator to refresh (non-fatal, build/* is a generated artifact)", stale)
	}
}

// --- small string helpers (avoid pulling strconv/strings into many sites) ---

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func containsImageDefault(content, envVar, ref string) bool {
	// `HERALD_IMAGE: { ..., default: "ghcr.io/soulteary/herald:v0.9.0" }`
	re := regexp.MustCompile(regexp.QuoteMeta(envVar) + `\s*:\s*\{[^}]*default:\s*"` + regexp.QuoteMeta(ref) + `"`)
	return re.MatchString(content)
}

func containsEnvAssignment(content, envVar, ref string) bool {
	// `KEY=ref` or `# KEY=ref` (optional/commented channels).
	re := regexp.MustCompile(`(?m)^\s*#?\s*` + regexp.QuoteMeta(envVar) + `=` + regexp.QuoteMeta(ref) + `\s*$`)
	return re.MatchString(content)
}

func envHasKey(content, envVar string) bool {
	re := regexp.MustCompile(`(?m)^\s*#?\s*` + regexp.QuoteMeta(envVar) + `=`)
	return re.MatchString(content)
}

func containsComposeDefault(content, envVar, ref string) bool {
	// `image: ${KEY:-ref}`
	re := regexp.MustCompile(`\$\{` + regexp.QuoteMeta(envVar) + `:-` + regexp.QuoteMeta(ref) + `\}`)
	return re.MatchString(content)
}

func containsConfigSectionDefault(content, envVar, ref string) bool {
	// An image option block declares envName followed by its default value.
	re := regexp.MustCompile(`envName:\s*` + regexp.QuoteMeta(envVar) +
		`\b[\s\S]{0,300}?default:\s*"` + regexp.QuoteMeta(ref) + `"`)
	return re.MatchString(content)
}

func containsPortEntry(content, serviceKey, port string) bool {
	// `serviceKey: <name>` ... `containerPort: "<port>"` within the same block.
	re := regexp.MustCompile(`serviceKey:\s*` + regexp.QuoteMeta(serviceKey) +
		`\b[\s\S]{0,200}?containerPort:\s*"` + regexp.QuoteMeta(port) + `"`)
	return re.MatchString(content)
}
