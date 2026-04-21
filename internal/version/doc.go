// Package version provides build-time information and version management for the LDAP Manager application.
//
// # Overview
//
// This package holds the three user-facing build metadata values —
// semantic version, git commit hash, and build timestamp — that the
// application reports via FormatVersion() and the `version` / `--version`
// subcommands.
//
// # Build-Time Injection (template-driven)
//
// Release builds use the shared go-app release pipeline
// (netresearch/.github/templates/go-app/.github/workflows/release.yml).
// Release metadata lands in package main via two reusable mechanisms:
//
//   - The release.yml ldflags input:
//
//     -X main.version=<tag>
//     -X main.build=<commit-sha>
//
//   - The build-go-attest.yml `auto-build-timestamp` input (enabled by
//     the release template), which after checkout runs
//     `git show -s --format=%cI HEAD` and appends
//     `-X main.buildTime=<ISO-8601>` to the effective ldflags. Works on
//     tag pushes and workflow_dispatch backfills alike because it reads
//     git directly instead of `github.event.head_commit.timestamp`.
//
// cmd/ldap-manager/main.go declares matching package-level string
// variables named version, build, and buildTime and calls
// forwardBuildMetadata() at init() time to copy their values into
// Version, CommitHash, and BuildTimestamp here. The indirection keeps
// the fleet ldflag convention (`main.*`) uniform across every go-app
// consumer while still letting this package expose structured
// package-level identifiers.
//
// Local development builds (plain `go build ./cmd/ldap-manager`) leave
// the shim vars empty; the defaults "dev"/"n/a"/"n/a" below are
// preserved unchanged.
//
// # Package Variables
//
// Three package-level variables store build metadata:
//
//   - Version: Semantic version string (e.g., "v1.0.8") or "dev" for development builds
//   - CommitHash: git commit SHA (e.g., "a4d1aae") or "n/a" if not available
//   - BuildTimestamp: ISO 8601 build timestamp (e.g., "2026-04-20T17:58:00Z") or "n/a"
//
// Default values ("dev", "n/a", "n/a") are used for development builds when no ldflags are provided.
//
// # Usage
//
// Display version information in the application:
//
//	import (
//	    "github.com/netresearch/ldap-manager/internal/version"
//	    "github.com/rs/zerolog/log"
//	)
//
//	func main() {
//	    log.Info().Str("version", version.FormatVersion()).Msg("Starting LDAP Manager")
//	    // Output (production): Starting LDAP Manager version=v1.0.8 (a4d1aae, built at 2026-04-20T17:58:00Z)
//	    // Output (development): Starting LDAP Manager version=Development version
//	}
//
// Version endpoint for monitoring:
//
//	func versionHandler(c *fiber.Ctx) error {
//	    return c.JSON(fiber.Map{
//	        "version":    version.Version,
//	        "commit":     version.CommitHash,
//	        "build_time": version.BuildTimestamp,
//	    })
//	}
//
// # FormatVersion Function
//
// FormatVersion() returns a human-readable version string:
//
//	// Development build (no ldflags)
//	version.Version = "dev"
//	version.FormatVersion() // Returns: "Development version"
//
//	// Release build (ldflags applied via main.* -> forwardBuildMetadata)
//	version.Version = "v1.0.8"
//	version.CommitHash = "a4d1aae"
//	version.BuildTimestamp = "2026-04-20T17:58:00Z"
//	version.FormatVersion() // Returns: "v1.0.8 (a4d1aae, built at 2026-04-20T17:58:00Z)"
//
// # Version String Format
//
// Version strings follow semantic versioning (SemVer) with optional git metadata:
//
//   - Release build: "v1.0.8" (clean git tag)
//   - Dirty working tree: "v1.0.8-dirty" (uncommitted changes)
//   - No git tags: "a4d1aae" (commit hash only)
//   - Development: "dev" (no version injection)
//
// # Use Cases
//
// Common scenarios where version information is used:
//
//  1. Application startup logs for debugging and auditing
//  2. Health check endpoints for monitoring systems
//  3. /version or /health endpoints for version discovery
//  4. Error reports to include build information for troubleshooting
//  5. Release notes and changelog generation
//  6. CI/CD pipeline integration for deployment tracking
//
// # Best Practices
//
//  1. Always use semantic versioning for Version (e.g., v1.0.8, not 1.0.8)
//  2. Include git commit hash for precise build identification
//  3. Use ISO 8601 format for timestamps (YYYY-MM-DDTHH:MM:SSZ)
//  4. Never hard-code version strings in source code
//  5. Include version in application logs at startup
//  6. Expose version via health check endpoint for monitoring
//
// For release pipeline details, see:
// https://github.com/netresearch/.github/blob/main/templates/go-app/.github/workflows/release.yml
package version
