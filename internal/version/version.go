// Package version provides build-time information and version management.
package version

import "fmt"

// Version, CommitHash, and BuildTimestamp hold build-time metadata
// injected via -ldflags at link time. They default to "dev" / "n/a"
// for local builds.
var (
	Version        = "dev"
	CommitHash     = "n/a"
	BuildTimestamp = "n/a"
)

// FormatVersion returns a human-readable version string including build metadata.
// Returns "Development version" for dev builds, or formatted version with commit and timestamp.
func FormatVersion() string {
	if Version == "dev" {
		return "Development version"
	}

	return fmt.Sprintf("%s (%s, built at %s)", Version, CommitHash, BuildTimestamp)
}
