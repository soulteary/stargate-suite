// Package main: `doctor` subcommand. Read-only diagnostics for a generated
// docker-compose.yml — compose syntax, image/manifest match, host-published
// ports and local port usage, declared networks, and (with --probe) liveness/
// readiness reachability. Supports --json for CI/Cursor. Exits non-zero only
// when a hard failure is found (unparseable compose, no services); advisory
// findings (warnings) do not fail the command unless --strict is given.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/soulteary/stargate-suite/internal/contract"
	"github.com/soulteary/stargate-suite/internal/doctor"
)

func cmdDoctor() error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	composePath := fs.String("compose", "", "path to a generated docker-compose.yml to diagnose (required)")
	jsonOut := fs.Bool("json", false, "emit the diagnostic report as JSON")
	probe := fs.Bool("probe", false, "actively probe liveness/readiness endpoints on published host ports (requires running services)")
	strict := fs.Bool("strict", false, "treat warnings as failures (non-zero exit on any warning)")
	probeTimeout := fs.Duration("probe-timeout", 3*time.Second, "per-endpoint HTTP probe timeout when --probe is set")
	if err := fs.Parse(cmdArgs); err != nil {
		return err
	}
	if *composePath == "" {
		return fmt.Errorf("doctor: --compose <docker-compose.yml> is required")
	}
	data, err := os.ReadFile(*composePath)
	if err != nil {
		return fmt.Errorf("doctor: read compose: %w", err)
	}

	rep, err := doctor.Run(doctor.Options{
		ComposePath:  *composePath,
		ComposeBytes: data,
		ManifestFS:   assetFS(),
		ManifestPath: contract.ManifestPath,
		Probe:        *probe,
		ProbeTimeout: *probeTimeout,
	})
	if err != nil {
		return fmt.Errorf("doctor: %w", err)
	}

	if *jsonOut {
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
	} else {
		printDoctorReport(rep)
	}

	// Exit code contract: hard failures always fail; warnings fail only under
	// --strict. This keeps `doctor` usable as a soft advisory by default and a
	// hard gate in CI when desired.
	if rep.HasFailures() {
		return fmt.Errorf("doctor: %d failing check(s)", rep.Fail)
	}
	if *strict && rep.Warn > 0 {
		return fmt.Errorf("doctor: %d warning(s) under --strict", rep.Warn)
	}
	return nil
}

// printDoctorReport renders a human-readable report to stdout.
func printDoctorReport(rep *doctor.Report) {
	fmt.Printf("doctor: %s\n", rep.Compose)
	for _, c := range rep.Checks {
		var mark string
		switch c.Status {
		case doctor.StatusOK:
			mark = "✓"
		case doctor.StatusWarn:
			mark = "!"
		case doctor.StatusFail:
			mark = "✗"
		}
		fmt.Printf("  %s [%s] %s\n", mark, c.Code, c.Message)
	}
	fmt.Printf("summary: %d ok, %d warn, %d fail\n", rep.OK, rep.Warn, rep.Fail)
}
