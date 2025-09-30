package version

import (
	"testing"
)

func TestFormatVersion_DevBuild(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := CommitHash
	originalBuild := BuildTimestamp
	defer func() {
		Version = originalVersion
		CommitHash = originalCommit
		BuildTimestamp = originalBuild
	}()

	// Test dev version
	Version = "dev"
	CommitHash = "n/a"
	BuildTimestamp = "n/a"

	result := FormatVersion()
	expected := "Development version"

	if result != expected {
		t.Errorf("FormatVersion() for dev build = %q, want %q", result, expected)
	}
}

func TestFormatVersion_ProductionBuild(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := CommitHash
	originalBuild := BuildTimestamp
	defer func() {
		Version = originalVersion
		CommitHash = originalCommit
		BuildTimestamp = originalBuild
	}()

	// Test production version
	Version = "v1.2.3"
	CommitHash = "abc123def456"
	BuildTimestamp = "2025-09-30T10:00:00Z"

	result := FormatVersion()
	expected := "v1.2.3 (abc123def456, built at 2025-09-30T10:00:00Z)"

	if result != expected {
		t.Errorf("FormatVersion() for production build = %q, want %q", result, expected)
	}
}

func TestFormatVersion_EmptyVersion(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := CommitHash
	originalBuild := BuildTimestamp
	defer func() {
		Version = originalVersion
		CommitHash = originalCommit
		BuildTimestamp = originalBuild
	}()

	// Test with empty version (should format as production)
	Version = ""
	CommitHash = "empty-test"
	BuildTimestamp = "2025-01-01"

	result := FormatVersion()
	expected := " (empty-test, built at 2025-01-01)"

	if result != expected {
		t.Errorf("FormatVersion() for empty version = %q, want %q", result, expected)
	}
}

func TestFormatVersion_SpecialCharacters(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := CommitHash
	originalBuild := BuildTimestamp
	defer func() {
		Version = originalVersion
		CommitHash = originalCommit
		BuildTimestamp = originalBuild
	}()

	// Test with special characters
	Version = "v2.0.0-beta.1+build.123"
	CommitHash = "abc-123-def"
	BuildTimestamp = "2025-12-31T23:59:59Z"

	result := FormatVersion()
	expected := "v2.0.0-beta.1+build.123 (abc-123-def, built at 2025-12-31T23:59:59Z)"

	if result != expected {
		t.Errorf("FormatVersion() with special chars = %q, want %q", result, expected)
	}
}

func TestFormatVersion_CaseInsensitiveDev(t *testing.T) {
	// Save original values
	originalVersion := Version
	originalCommit := CommitHash
	originalBuild := BuildTimestamp
	defer func() {
		Version = originalVersion
		CommitHash = originalCommit
		BuildTimestamp = originalBuild
	}()

	// Test that only lowercase "dev" is recognized
	testCases := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "lowercase dev",
			version:  "dev",
			expected: "Development version",
		},
		{
			name:     "uppercase DEV",
			version:  "DEV",
			expected: "DEV (test-commit, built at 2025-01-01)",
		},
		{
			name:     "mixed case Dev",
			version:  "Dev",
			expected: "Dev (test-commit, built at 2025-01-01)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.version
			CommitHash = "test-commit"
			BuildTimestamp = "2025-01-01"

			result := FormatVersion()
			if result != tc.expected {
				t.Errorf("FormatVersion() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestVersionVariables_DefaultValues(t *testing.T) {
	// This test verifies that default values are set
	// Note: In actual builds, these will be overridden by ldflags
	if Version == "" {
		t.Error("Version should have a default value")
	}
	if CommitHash == "" {
		t.Error("CommitHash should have a default value")
	}
	if BuildTimestamp == "" {
		t.Error("BuildTimestamp should have a default value")
	}
}
