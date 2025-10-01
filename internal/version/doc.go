// Package version provides build-time information and version management for the LDAP Manager application.
//
// # Overview
//
// This package manages application version metadata that is injected at build time using Go's -ldflags.
// It provides three key pieces of information: semantic version, git commit hash, and build timestamp.
//
// # Build-Time Injection
//
// Version information is injected during the build process using -ldflags to set package-level variables:
//
//	go build -ldflags="\
//	  -X 'github.com/netresearch/ldap-manager/internal/version.Version=v1.0.8' \
//	  -X 'github.com/netresearch/ldap-manager/internal/version.CommitHash=$(git rev-parse --short HEAD)' \
//	  -X 'github.com/netresearch/ldap-manager/internal/version.BuildTimestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
//	" ./cmd/ldap-manager
//
// The Makefile automates this process:
//
//	make build        # Production build with version injection
//	make build-dev    # Development build (Version="dev")
//
// # Package Variables
//
// Three package-level variables store build metadata:
//
//   - Version: Semantic version string (e.g., "v1.0.8") or "dev" for development builds
//   - CommitHash: Short git commit SHA (e.g., "a4d1aae") or "n/a" if not available
//   - BuildTimestamp: ISO 8601 build timestamp (e.g., "2025-09-30T21:41:41Z") or "n/a"
//
// Default values ("dev", "n/a", "n/a") are used for development builds when -ldflags are not provided.
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
//	    // Output (production): Starting LDAP Manager version=v1.0.8 (a4d1aae, built at 2025-09-30T21:41:41Z)
//	    // Output (development): Starting LDAP Manager version=Development version
//	}
//
// Add version endpoint for monitoring:
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
// The FormatVersion() function provides human-readable version strings:
//
//	// Development build (no -ldflags)
//	version.Version = "dev"
//	version.FormatVersion() // Returns: "Development version"
//
//	// Production build (with -ldflags)
//	version.Version = "v1.0.8"
//	version.CommitHash = "a4d1aae"
//	version.BuildTimestamp = "2025-09-30T21:41:41Z"
//	version.FormatVersion() // Returns: "v1.0.8 (a4d1aae, built at 2025-09-30T21:41:41Z)"
//
// # Makefile Integration
//
// The project Makefile handles version injection automatically:
//
//	# Extract version from git tags
//	VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
//	COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "n/a")
//	BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
//
//	# Build with version injection
//	build:
//	    @echo "Building version $(VERSION)..."
//	    @go build -ldflags="\
//	      -X 'github.com/netresearch/ldap-manager/internal/version.Version=$(VERSION)' \
//	      -X 'github.com/netresearch/ldap-manager/internal/version.CommitHash=$(COMMIT_HASH)' \
//	      -X 'github.com/netresearch/ldap-manager/internal/version.BuildTimestamp=$(BUILD_TIME)' \
//	    " -o bin/ldap-manager ./cmd/ldap-manager
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
// # Docker Builds
//
// For Docker images, version is injected at build time:
//
//	# In Dockerfile
//	ARG VERSION=dev
//	ARG COMMIT_HASH=n/a
//	ARG BUILD_TIME=n/a
//
//	RUN go build -ldflags="\
//	  -X 'github.com/netresearch/ldap-manager/internal/version.Version=${VERSION}' \
//	  -X 'github.com/netresearch/ldap-manager/internal/version.CommitHash=${COMMIT_HASH}' \
//	  -X 'github.com/netresearch/ldap-manager/internal/version.BuildTimestamp=${BUILD_TIME}' \
//	" ./cmd/ldap-manager
//
//	# Build with version
//	docker build \
//	  --build-arg VERSION=v1.0.8 \
//	  --build-arg COMMIT_HASH=$(git rev-parse --short HEAD) \
//	  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
//	  -t ldap-manager:v1.0.8 .
//
// # Best Practices
//
//  1. Always use semantic versioning for Version field (e.g., v1.0.8, not 1.0.8)
//  2. Include git commit hash for precise build identification
//  3. Use ISO 8601 format for timestamps (YYYY-MM-DDTHH:MM:SSZ)
//  4. Automate version injection in CI/CD pipelines
//  5. Never hard-code version strings in source code
//  6. Include version in application logs at startup
//  7. Expose version via health check endpoint for monitoring
//
// For more details on build process, see: docs/development/contributing.md
package version
