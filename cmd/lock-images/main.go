// Command lock-images resolves every image in config/components.yaml to an
// immutable registry digest and writes a release-ready components.lock.yaml.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/soulteary/stargate-suite/internal/contract"
)

func main() {
	manifestPath := flag.String("manifest", contract.ManifestPath, "component manifest path")
	outputPath := flag.String("output", contract.LockPath, "lock output path")
	fromLock := flag.String("from-lock", "", "validate and copy an existing tested lock without resolving tags")
	flag.Parse()

	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		fatalf("read manifest: %v", err)
	}
	manifest, err := contract.ParseManifest(data)
	if err != nil {
		fatalf("parse manifest: %v", err)
	}
	if *fromLock != "" {
		data, err := os.ReadFile(*fromLock)
		if err != nil {
			fatalf("read tested lock: %v", err)
		}
		if err := validateTestedLock(manifest, data); err != nil {
			fatalf("validate tested lock: %v", err)
		}
		if err := os.WriteFile(*outputPath, data, 0o644); err != nil {
			fatalf("copy tested lock: %v", err)
		}
		fmt.Printf("Copied validated tested lock from %s to %s\n", *fromLock, *outputPath)
		return
	}
	lock, err := contract.NewResolvedLock(manifest, dockerDigest)
	if err != nil {
		fatalf("build lock: %v", err)
	}
	if err := contract.ValidateLock(manifest, lock, true); err != nil {
		fatalf("validate generated lock: %v", err)
	}
	out, err := contract.MarshalLock(lock)
	if err != nil {
		fatalf("marshal lock: %v", err)
	}
	if err := os.WriteFile(*outputPath, out, 0o644); err != nil {
		fatalf("write lock: %v", err)
	}
	fmt.Printf("Wrote %s with %d immutable image digests\n", *outputPath, len(lock.Images))
}

func validateTestedLock(manifest *contract.Manifest, data []byte) error {
	lock, err := contract.ParseLock(data)
	if err != nil {
		return err
	}
	return contract.ValidateLock(manifest, lock, true)
}

func dockerDigest(ref string) (string, error) {
	cmd := exec.Command("docker", "buildx", "imagetools", "inspect", ref)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker buildx imagetools inspect: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	digest, err := parseInspectDigest(string(out))
	if err != nil {
		return "", err
	}
	return digest, nil
}

func parseInspectDigest(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "Digest:" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("docker buildx imagetools inspect output has no descriptor digest")
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
