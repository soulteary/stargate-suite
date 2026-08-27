// Package contract holds cross-file consistency tests that guard against
// configuration drift between go.mod, CI workflows, the Dockerfile and docs.
package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// goVersion is a parsed Go language version (major.minor[.patch]).
type goVersion struct {
	major, minor, patch int
}

func (v goVersion) String() string {
	return strconv.Itoa(v.major) + "." + strconv.Itoa(v.minor) + "." + strconv.Itoa(v.patch)
}

// lessThan reports whether v is strictly older than o, comparing only
// major.minor (patch is ignored so that a builder image like
// "golang:1.27-alpine" is considered equal to go.mod's "1.27.0").
func (v goVersion) lessThan(o goVersion) bool {
	if v.major != o.major {
		return v.major < o.major
	}
	return v.minor < o.minor
}

func parseGoVersion(s string) (goVersion, bool) {
	m := regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`).FindStringSubmatch(s)
	if m == nil {
		return goVersion{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return goVersion{major, minor, patch}, true
}

// repoRoot walks up from the test's working directory until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above test directory")
		}
		dir = parent
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// goModVersion extracts the version from the `go` directive of go.mod.
func goModVersion(t *testing.T, root string) goVersion {
	t.Helper()
	content := readFile(t, root, "go.mod")
	m := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)`).FindStringSubmatch(content)
	if m == nil {
		t.Fatal("go.mod has no parseable `go` directive")
	}
	v, ok := parseGoVersion(m[1])
	if !ok {
		t.Fatalf("go.mod `go` directive not parseable: %q", m[1])
	}
	return v
}

func TestGoVersionNotLowerThanGoMod(t *testing.T) {
	root := repoRoot(t)
	base := goModVersion(t, root)

	// Each case extracts every Go version reference from a file and asserts
	// none of them are older than go.mod's declared version. This catches the
	// historical drift where go.mod said 1.27 but the Dockerfile/CI still
	// pinned 1.25.
	cases := []struct {
		name string
		file string
		// re captures a Go version in submatch group 1 for each reference we care about.
		re *regexp.Regexp
	}{
		{
			name: "Dockerfile builder",
			file: "Dockerfile",
			re:   regexp.MustCompile(`golang:(\d+\.\d+(?:\.\d+)?)`),
		},
		{
			name: "ci.yml hardcoded go-version",
			file: ".github/workflows/ci.yml",
			// Matches `go-version: '1.25'` style pins but NOT `go-version-file:`.
			re: regexp.MustCompile(`go-version:\s*'?(\d+\.\d+(?:\.\d+)?)'?`),
		},
		{
			name: "release.yml hardcoded go-version",
			file: ".github/workflows/release.yml",
			re:   regexp.MustCompile(`go-version:\s*'?(\d+\.\d+(?:\.\d+)?)'?`),
		},
		{
			name: "README.md prerequisite",
			file: "README.md",
			re:   regexp.MustCompile(`Go\s+(\d+\.\d+(?:\.\d+)?)\+?`),
		},
		{
			name: "README.zh-CN.md prerequisite",
			file: "README.zh-CN.md",
			re:   regexp.MustCompile(`Go\s+(\d+\.\d+(?:\.\d+)?)\+?`),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content := readFile(t, root, c.file)
			matches := c.re.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				v, ok := parseGoVersion(m[1])
				if !ok {
					continue
				}
				if v.lessThan(base) {
					t.Errorf("%s references Go %s which is lower than go.mod %s (in %q)",
						c.file, v, base, strings.TrimSpace(m[0]))
				}
			}
		})
	}
}

// TestContractCatchesDrift is a self-check ensuring the comparison logic would
// actually flag a lower version, so the guard can't silently degrade.
func TestContractCatchesDrift(t *testing.T) {
	root := repoRoot(t)
	base := goModVersion(t, root)
	older := goVersion{major: base.major, minor: base.minor - 1}
	if !older.lessThan(base) {
		t.Fatalf("expected %s to be detected as older than %s", older, base)
	}
	same := goVersion{major: base.major, minor: base.minor, patch: 999}
	if same.lessThan(base) {
		t.Fatalf("patch-only difference %s should not be flagged against %s", same, base)
	}
}
