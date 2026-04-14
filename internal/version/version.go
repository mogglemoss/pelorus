// Package version exposes build-time version metadata for display in the UI.
//
// These vars are intended to be overridden via -ldflags at build time:
//
//	go build -ldflags "\
//	  -X github.com/mogglemoss/pelorus/internal/version.Version=1.2.3 \
//	  -X github.com/mogglemoss/pelorus/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/mogglemoss/pelorus/internal/version.BuildDate=$(date -u +%Y-%m-%d)"
//
// cmd/root.go mirrors Version into cobra's rootCmd.Version so `pelorus
// --version` reflects the same string.
package version

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	// Version is the semantic version string.
	Version = "1.0.0-dev"
	// Commit is the short git commit SHA; optional.
	Commit = ""
	// BuildDate is the ISO-8601 build date; optional.
	BuildDate = ""
)

// Short returns just the version, e.g. "1.0.0".
func Short() string {
	return Version
}

// Full returns a human-readable one-line summary suitable for toasts and
// footers, e.g. "pelorus 1.0.0 · abc1234 · go1.22 · 2025-04-14".
func Full() string {
	parts := []string{"pelorus " + Version}
	if Commit != "" {
		parts = append(parts, Commit)
	}
	parts = append(parts, runtime.Version())
	if BuildDate != "" {
		parts = append(parts, BuildDate)
	}
	return strings.Join(parts, " · ")
}

// Title returns a short "Pelorus v1.0.0" header, suitable for the help
// overlay title bar.
func Title() string {
	return fmt.Sprintf("Pelorus v%s", Version)
}
