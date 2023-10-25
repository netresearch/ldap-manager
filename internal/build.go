package internal

import "fmt"

var (
	Version        = "dev"
	CommitHash     = "n/a"
	BuildTimestamp = "n/a"
)

func FormatVersion() string {
	if Version == "dev" {
		return "Development version"
	}

	return fmt.Sprintf("%s (%s, built at %s)", Version, CommitHash, BuildTimestamp)
}
