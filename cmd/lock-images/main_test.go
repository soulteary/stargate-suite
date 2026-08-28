package main

import "testing"

func TestParseInspectDigest(t *testing.T) {
	output := "Name: example.invalid/app:1.0\nMediaType: application/vnd.oci.image.index.v1+json\nDigest: sha256:0123456789abcdef\n"
	digest, err := parseInspectDigest(output)
	if err != nil {
		t.Fatalf("parseInspectDigest: %v", err)
	}
	if digest != "sha256:0123456789abcdef" {
		t.Fatalf("digest = %q", digest)
	}
}

func TestParseInspectDigestRejectsMissingDigest(t *testing.T) {
	if _, err := parseInspectDigest("Name: example.invalid/app:1.0\n"); err == nil {
		t.Fatal("expected missing digest error")
	}
}
