package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Container smoke tests. These require Docker and are opt-in: set
// RUN_DOCKER_SMOKE=1 to run them. They are skipped by default so that
// `go test ./...` (unit mode, no Docker) stays fast and hermetic.
//
// They verify B-02's acceptance criteria end-to-end:
//   - the image runs `serve` with NO repository mount (embedded assets), and
//   - it works under a read-only root filesystem (writes go to tmpfs only).

const defaultSmokeImage = "stargate-suite:smoke-test"

func smokeImage() string {
	if image := strings.TrimSpace(os.Getenv("DOCKER_SMOKE_IMAGE")); image != "" {
		return image
	}
	return defaultSmokeImage
}

func requireDockerSmoke(t *testing.T) {
	t.Helper()
	if os.Getenv("RUN_DOCKER_SMOKE") != "1" {
		t.Skip("skipping Docker smoke test; set RUN_DOCKER_SMOKE=1 to enable (requires Docker)")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH; skipping container smoke test")
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == "" || parent == dir {
			t.Fatal("could not locate repo root (go.mod)")
		}
		dir = parent
	}
}

func dockerBuild(t *testing.T) {
	t.Helper()
	if os.Getenv("DOCKER_SMOKE_IMAGE") != "" {
		return
	}
	root := repoRootDir(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", smokeImage(), ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker build failed: %v\n%s", err, out)
	}
}

func waitHTTPOK(t *testing.T, url, token string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("create smoke request: %v", err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("service at %s did not become ready within %s", url, timeout)
}

func containerToken(t *testing.T, name string, timeout time.Duration) string {
	t.Helper()
	pattern := regexp.MustCompile(`Authorization: Bearer ([[:xdigit:]]{32})`)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command("docker", "logs", name).CombinedOutput()
		if err == nil {
			if match := pattern.FindStringSubmatch(string(out)); len(match) == 2 {
				return match[1]
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("container %s did not print an access token within %s", name, timeout)
	return ""
}

// TestContainerServeNoRepoMount builds the image and runs `serve` without any
// repository mount, confirming the Web UI is reachable (embedded assets).
func TestContainerServeNoRepoMount(t *testing.T) {
	requireDockerSmoke(t)
	dockerBuild(t)

	name := "stargate-suite-smoke-serve"
	_ = exec.Command("docker", "rm", "-f", name).Run()
	run := exec.Command("docker", "run", "-d", "--name", name,
		"-p", "127.0.0.1:18085:8085", smokeImage())
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	token := containerToken(t, name, 10*time.Second)
	waitHTTPOK(t, "http://127.0.0.1:18085/", token, 30*time.Second)
	waitHTTPOK(t, "http://127.0.0.1:18085/healthz", "", 5*time.Second)
}

// TestContainerServeReadOnlyRootFS runs the image with a read-only root
// filesystem and a tmpfs for writable paths, confirming the app does not need
// to write into its own image (assets are embedded and read-only).
func TestContainerServeReadOnlyRootFS(t *testing.T) {
	requireDockerSmoke(t)
	dockerBuild(t)

	name := "stargate-suite-smoke-ro"
	_ = exec.Command("docker", "rm", "-f", name).Run()
	run := exec.Command("docker", "run", "-d", "--name", name,
		"--read-only", "--tmpfs", "/tmp",
		"-p", "127.0.0.1:18086:8085", smokeImage())
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("docker run (read-only) failed: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	token := containerToken(t, name, 10*time.Second)
	waitHTTPOK(t, "http://127.0.0.1:18086/", token, 30*time.Second)
}
