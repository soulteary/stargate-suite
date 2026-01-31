// Package main: docker compose up/down/logs/ps/restart/health and test-wait.
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func buildComposePath(mode string) string {
	return filepath.Join(projectRoot(), buildDirRelative, mode, "docker-compose.yml")
}

func cmdUp() error {
	return run("docker", "compose", "-f", composeFile(), "up", "-d")
}

func cmdUpBuild() error {
	return run("docker", "compose", "-f", buildComposePath("build"), "up", "-d", "--build")
}

func cmdUpImage() error {
	return run("docker", "compose", "-f", buildComposePath("image"), "up", "-d")
}

func cmdUpTraefik() error {
	return run("docker", "compose", "-f", buildComposePath("traefik"), "up", "-d")
}

func cmdNetTraefikSplit() error {
	for _, net := range []string{"the-gate-network", "traefik"} {
		cmd := exec.Command("docker", "network", "create", net)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}
	return nil
}

func cmdUpTraefikHerald() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-herald"), "up", "-d")
}

func cmdUpTraefikWarden() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-warden"), "up", "-d")
}

func cmdUpTraefikStargate() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-stargate"), "up", "-d")
}

func cmdDown() error {
	return run("docker", "compose", "-f", composeFile(), "down")
}

func cmdDownBuild() error {
	return run("docker", "compose", "-f", buildComposePath("build"), "down")
}

func cmdDownImage() error {
	return run("docker", "compose", "-f", buildComposePath("image"), "down")
}

func cmdDownTraefik() error {
	return run("docker", "compose", "-f", buildComposePath("traefik"), "down")
}

func cmdDownTraefikHerald() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-herald"), "down")
}

func cmdDownTraefikWarden() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-warden"), "down")
}

func cmdDownTraefikStargate() error {
	return run("docker", "compose", "-f", buildComposePath("traefik-stargate"), "down")
}

func cmdLogs() error {
	return run("docker", "compose", "-f", composeFile(), "logs", "-f")
}

func cmdPs() error {
	return run("docker", "compose", "-f", composeFile(), "ps")
}

func cmdTest() error {
	return run("go", "test", "-v", "./e2e/...")
}

var testWaitTimeout = 60 * time.Second

func waitForServicesReady(timeout time.Duration) bool {
	checks := []struct {
		name, url string
	}{
		{"Stargate", "http://localhost:8080/_auth"},
		{"Warden", "http://localhost:8081/health"},
		{"Herald", "http://localhost:8082/healthz"},
	}
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allOk := true
		for _, c := range checks {
			resp, err := client.Get(c.url)
			if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
				allOk = false
				if resp != nil {
					_ = resp.Body.Close()
				}
				break
			}
			_ = resp.Body.Close()
		}
		if allOk {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func cmdTestWait() error {
	if s := strings.TrimSpace(os.Getenv("TEST_WAIT_TIMEOUT")); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			testWaitTimeout = d
		}
	}
	fmt.Printf("Waiting for services to be ready (timeout %s)...\n", testWaitTimeout)
	if !waitForServicesReady(testWaitTimeout) {
		fmt.Fprintf(os.Stderr, "Services did not become ready within %s. Run make health to check.\n", testWaitTimeout)
		return fmt.Errorf("services not ready")
	}
	fmt.Println("All services ready. Running tests.")
	return run("go", "test", "-v", "./e2e/...")
}

func cmdClean() error {
	return run("docker", "compose", "-f", composeFile(), "down", "-v")
}

func cmdRestart() error {
	return run("docker", "compose", "-f", composeFile(), "restart")
}

func cmdRestartWarden() error {
	return run("docker", "compose", "-f", composeFile(), "restart", "warden")
}

func cmdRestartHerald() error {
	return run("docker", "compose", "-f", composeFile(), "restart", "herald")
}

func cmdRestartStargate() error {
	return run("docker", "compose", "-f", composeFile(), "restart", "stargate")
}

func cmdHealth() error {
	client := &http.Client{Timeout: 5 * time.Second}
	checks := []struct {
		name, url string
	}{
		{"Stargate", "http://localhost:8080/_auth"},
		{"Warden", "http://localhost:8081/health"},
		{"Herald", "http://localhost:8082/healthz"},
	}
	for _, c := range checks {
		fmt.Printf("Checking %s...\n", c.name)
		resp, err := client.Get(c.url)
		if err != nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
			fmt.Printf("✗ %s Unhealthy\n", c.name)
		} else {
			_ = resp.Body.Close()
			fmt.Printf("✓ %s Healthy\n", c.name)
		}
	}
	return nil
}
