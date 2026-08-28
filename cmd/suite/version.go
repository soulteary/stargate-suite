// Package main: `version` subcommand and build-time version metadata.
//
// Version, Commit and BuildDate are injected at build time via -ldflags -X
// (see Dockerfile and .github/workflows/release.yml). They fall back to
// development defaults when built without injection (e.g. `go run`/`go build`).
package main

import (
	"fmt"
	"runtime"
)

// Build metadata, overridable via:
//
//	go build -ldflags "-X main.Version=... -X main.Commit=... -X main.BuildDate=..."
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Verified component combination this Suite release is validated against.
// These constants are the fallback contract source; the authoritative values
// live in config/components.yaml under `verifiedCombo` and are read at runtime
// by cmdVersion so there is a single source of truth (M-01). The constants
// remain so `version` still reports a sane combo if the manifest cannot be
// loaded (e.g. a binary built without embedded assets).
const (
	verifiedStargate = "v1.0.0"
	verifiedWarden   = "v1.1.0"
	verifiedHerald   = "v1.1.0"
)

// verifiedCombo returns the verified target versions, sourced from the manifest
// (config/components.yaml `verifiedCombo`) when available, else the constants.
func verifiedCombo() (stargate, warden, herald string) {
	stargate, warden, herald = verifiedStargate, verifiedWarden, verifiedHerald
	m, err := loadManifest()
	if err != nil || m == nil {
		return
	}
	if v := m.VerifiedCombo["stargate"]; v != "" {
		stargate = v
	}
	if v := m.VerifiedCombo["warden"]; v != "" {
		warden = v
	}
	if v := m.VerifiedCombo["herald"]; v != "" {
		herald = v
	}
	return
}

func cmdVersion() error {
	stargate, warden, herald := verifiedCombo()
	fmt.Printf("stargate-suite %s\n", Version)
	fmt.Printf("  Commit:     %s\n", Commit)
	fmt.Printf("  BuildDate:  %s\n", BuildDate)
	fmt.Printf("  Go:         %s\n", runtime.Version())
	fmt.Println("  Verified components:")
	fmt.Printf("    Stargate: %s\n", stargate)
	fmt.Printf("    Warden:   %s\n", warden)
	fmt.Printf("    Herald:   %s\n", herald)
	return nil
}
