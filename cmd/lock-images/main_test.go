package main

import (
	"os"
	"strings"
	"testing"

	"github.com/soulteary/stargate-suite/internal/contract"
)

func TestParseInspectDigest(t *testing.T) {
	want := "sha256:" + strings.Repeat("a", 64)
	output := "Name: example.invalid/app:1.0\nMediaType: application/vnd.oci.image.index.v1+json\nDigest: " + want + "\n"
	digest, err := parseInspectDigest(output)
	if err != nil {
		t.Fatalf("parseInspectDigest: %v", err)
	}
	if digest != want {
		t.Fatalf("digest = %q", digest)
	}
}

func TestParseInspectDigestRejectsMissingDigest(t *testing.T) {
	if _, err := parseInspectDigest("Name: example.invalid/app:1.0\n"); err == nil {
		t.Fatal("expected missing digest error")
	}
}

func TestValidateTestedLock(t *testing.T) {
	manifestData, err := os.ReadFile("../../config/components.yaml")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := contract.ParseManifest(manifestData)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../config/components.lock.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTestedLock(manifest, data); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"missing digest", "invalid digest", "image", "version", "schema", "missing image"} {
		t.Run(field, func(t *testing.T) {
			lock, err := contract.ParseLock(data)
			if err != nil {
				t.Fatal(err)
			}
			image := lock.Images["redis"]
			switch field {
			case "missing digest":
				image.Digest = ""
			case "invalid digest":
				image.Digest = "sha256:placeholder"
			case "image":
				image.Image = "example.invalid/redis"
			case "version":
				image.Version = "wrong"
			case "schema":
				lock.SchemaVersion++
			case "missing image":
				delete(lock.Images, "herald")
			}
			lock.Images["redis"] = image
			invalid, err := contract.MarshalLock(lock)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateTestedLock(manifest, invalid); err == nil {
				t.Fatal("accepted invalid tested lock")
			}
		})
	}
}
