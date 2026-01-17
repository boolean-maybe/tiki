package config

// Build information variables injected via ldflags at compile time.
// These are set by the build process (Makefile or GoReleaser) using:
// -ldflags "-X tiki/config.Version=... -X tiki/config.GitCommit=... -X tiki/config.BuildDate=..."
var (
	// Version is the semantic version or commit hash.
	Version = "dev"

	// GitCommit is the full git commit hash.
	GitCommit = "unknown"

	// BuildDate is the ISO 8601 build timestamp.
	BuildDate = "unknown"
)
