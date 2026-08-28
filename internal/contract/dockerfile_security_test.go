package contract

import (
	"strings"
	"testing"
)

func TestDockerBuildContextExcludesSecrets(t *testing.T) {
	root := repoRoot(t)
	ignore := readFile(t, root, ".dockerignore")
	for _, required := range []string{
		".git", "**/.env", "**/.env.*", "**/*.pem", "**/*.key", "**/*.crt", "**/secrets", "**/secrets/**",
	} {
		if !hasExactLine(ignore, required) {
			t.Errorf(".dockerignore does not exclude %q", required)
		}
	}
	if !hasExactLine(ignore, "!**/.env.example") {
		t.Error(".dockerignore must retain the non-secret .env.example documentation")
	}
}

func TestRuntimeImageUsesNonRootUser(t *testing.T) {
	root := repoRoot(t)
	dockerfile := readFile(t, root, "Dockerfile")
	if !strings.Contains(dockerfile, "USER stargate-suite:stargate-suite") {
		t.Fatal("runtime Dockerfile must select the unprivileged stargate-suite user")
	}
	if !strings.Contains(dockerfile, "COPY --from=builder --chown=stargate-suite:stargate-suite") {
		t.Fatal("runtime binary must be owned by the unprivileged user")
	}
}

func hasExactLine(body, want string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}
