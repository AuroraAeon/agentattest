package app

import "io"

// Build information. Overridden at release time via
// go build -ldflags "-X agentattest.dev/agentattest/internal/app.version=..."
// (see .goreleaser.yaml and .github/workflows/release.yml). The defaults keep
// `go run` / local builds honestly labeled as dev builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// VersionInfo is the machine-readable build descriptor emitted by
// `agentattest version` and `agentattest --version`.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Version returns the build information of the running binary.
func Version() VersionInfo {
	return VersionInfo{Version: version, Commit: commit, Date: date}
}

func runVersion(stdout io.Writer) error {
	return writeJSON(stdout, Version())
}
