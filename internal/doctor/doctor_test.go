package doctor

import (
	"testing"
	"testing/fstest"
	"time"
)

// testManifestFS returns an in-memory FS with a minimal components.yaml so the
// image cross-check has a manifest to compare against.
func testManifestFS() fstest.MapFS {
	return fstest.MapFS{
		"config/components.yaml": {Data: []byte(`schemaVersion: 1
components:
  stargate:
    image: ghcr.io/soulteary/stargate
    version: 1.0.0
    contractVersion: v1
    containerPort: 8080
    livenessPath: /healthz
    readinessPath: /readyz
  herald:
    image: ghcr.io/soulteary/herald
    version: 1.1.0
    contractVersion: v1
    containerPort: 8082
    livenessPath: /healthz
verifiedCombo:
  stargate: v1.0.0
  herald: v1.1.0
`)},
	}
}

func findCheck(rep *Report, code string) (Check, bool) {
	for _, c := range rep.Checks {
		if c.Code == code {
			return c, true
		}
	}
	return Check{}, false
}

func countCode(rep *Report, code string) int {
	n := 0
	for _, c := range rep.Checks {
		if c.Code == code {
			n++
		}
	}
	return n
}

// TestRunParsesAndMatchesManifest asserts a well-formed compose whose images
// resolve to the manifest refs produces only OK image checks (interpolation
// defaults are resolved before comparison).
func TestRunParsesAndMatchesManifest(t *testing.T) {
	compose := []byte(`services:
  stargate:
    image: ${STARGATE_IMAGE:-ghcr.io/soulteary/stargate:1.0.0}
    ports:
      - "8080:8080"
  herald:
    image: ghcr.io/soulteary/herald:1.1.0
networks:
  auth-internal:
    internal: true
`)
	rep, err := Run(Options{
		ComposePath:  "test.yml",
		ComposeBytes: compose,
		ManifestFS:   testManifestFS(),
		ManifestPath: "config/components.yaml",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if rep.Fail != 0 {
		t.Errorf("expected no failures, got %d: %+v", rep.Fail, rep.Checks)
	}
	if c, ok := findCheck(rep, CodeComposeParse); !ok || c.Status != StatusOK {
		t.Errorf("compose parse must be OK")
	}
	// Both images resolve to manifest refs → 2 OK image checks, 0 warn.
	for _, c := range rep.Checks {
		if c.Code == CodeImageManifest && c.Status != StatusOK {
			t.Errorf("image check must be OK, got %v: %s", c.Status, c.Message)
		}
	}
	if countCode(rep, CodeImageManifest) != 2 {
		t.Errorf("expected 2 image checks, got %d", countCode(rep, CodeImageManifest))
	}
}

// TestRunImageDriftIsWarning asserts an image at a different version than the
// manifest is a warning (drift is visible), not a hard failure.
func TestRunImageDriftIsWarning(t *testing.T) {
	compose := []byte(`services:
  herald:
    image: ghcr.io/soulteary/herald:v0.6.1
`)
	rep, err := Run(Options{ComposeBytes: compose, ManifestFS: testManifestFS(), ManifestPath: "config/components.yaml"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c, ok := findCheck(rep, CodeImageManifest)
	if !ok {
		t.Fatalf("expected an image check")
	}
	if c.Status != StatusWarn {
		t.Errorf("image drift must be a warning, got %v", c.Status)
	}
	if rep.HasFailures() {
		t.Errorf("image drift must not be a hard failure")
	}
}

// TestRunUnparseableComposeFails asserts a compose that cannot be parsed yields
// a hard failure (and a Report, not an error).
func TestRunUnparseableComposeFails(t *testing.T) {
	rep, err := Run(Options{ComposeBytes: []byte("this: [is: not: valid: yaml")})
	if err != nil {
		t.Fatalf("Run should return a report, not an error: %v", err)
	}
	if !rep.HasFailures() {
		t.Errorf("unparseable compose must fail")
	}
	if c, ok := findCheck(rep, CodeComposeParse); !ok || c.Status != StatusFail {
		t.Errorf("expected COMPOSE_PARSE failure")
	}
}

// TestRunNoServicesFails asserts a compose without services is a hard failure.
func TestRunNoServicesFails(t *testing.T) {
	rep, err := Run(Options{ComposeBytes: []byte("networks: {}\n")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.HasFailures() {
		t.Errorf("compose with no services must fail")
	}
}

// TestResolveComposeImage covers the interpolation-default unwrapping used by
// the image cross-check.
func TestResolveComposeImage(t *testing.T) {
	cases := map[string]string{
		"${HERALD_IMAGE:-ghcr.io/soulteary/herald:1.1.0}": "ghcr.io/soulteary/herald:1.1.0",
		"${IMG-plain:tag}": "plain:tag",
		"redis:8.4-alpine": "redis:8.4-alpine",
		"${NO_DEFAULT}":    "${NO_DEFAULT}",
	}
	for in, want := range cases {
		if got := resolveComposeImage(in); got != want {
			t.Errorf("resolveComposeImage(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestExtractPorts covers short/long syntax and bare (non-published) ports.
func TestExtractPorts(t *testing.T) {
	v := []interface{}{
		"8080:80",
		"127.0.0.1:8082:8082",
		"6379", // bare, not host-published
		map[string]interface{}{"published": "9000", "target": "9000"},
	}
	got := extractPorts(v)
	if len(got) != 3 {
		t.Fatalf("expected 3 published ports, got %d: %+v", len(got), got)
	}
	if got[0].hostPort != 8080 || got[1].hostPort != 8082 || got[2].hostPort != 9000 {
		t.Errorf("unexpected host ports: %+v", got)
	}
}

// TestNetworksWarningWhenAbsent asserts a compose without top-level networks
// produces a warning (not a failure).
func TestNetworksWarningWhenAbsent(t *testing.T) {
	compose := []byte(`services:
  herald:
    image: ghcr.io/soulteary/herald:1.1.0
`)
	rep, err := Run(Options{ComposeBytes: compose, ManifestFS: testManifestFS(), ManifestPath: "config/components.yaml", dialTimeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c, ok := findCheck(rep, CodeNetwork)
	if !ok || c.Status != StatusWarn {
		t.Errorf("absent networks must be a warning; got %+v ok=%v", c, ok)
	}
}
