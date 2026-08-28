package main

import (
	"strings"
	"testing"
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
