package options

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Test helpers for environment variable testing
func setEnvVar(t *testing.T, key, value string) func() {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	return func() {
		if err := os.Unsetenv(key); err != nil {
			t.Logf("Failed to unset environment variable: %v", err)
		}
	}
}

func unsetEnvVar(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Logf("Failed to unset environment variable: %v", err)
	}
}

func TestEnvStringOrDefault(t *testing.T) {
	t.Run("returns environment value when set", func(t *testing.T) {
		defer setEnvVar(t, "TEST_VAR", "env_value")()

		result := envStringOrDefault("TEST_VAR", "default_value")
		if result != "env_value" {
			t.Errorf("Expected 'env_value', got '%s'", result)
		}
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_VAR")

		result := envStringOrDefault("TEST_VAR", "default_value")
		if result != "default_value" {
			t.Errorf("Expected 'default_value', got '%s'", result)
		}
	})

	t.Run("returns default when environment variable is empty", func(t *testing.T) {
		defer setEnvVar(t, "TEST_VAR", "")()

		result := envStringOrDefault("TEST_VAR", "default_value")
		if result != "default_value" {
			t.Errorf("Expected 'default_value', got '%s'", result)
		}
	})
}

func TestEnvDurationOrDefault(t *testing.T) {
	t.Run("returns environment duration when valid", func(t *testing.T) {
		defer setEnvVar(t, "TEST_DURATION", "5m")()

		result := envDurationOrDefault("TEST_DURATION", 1*time.Minute)
		expected := 5 * time.Minute
		if result != expected {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_DURATION")

		result := envDurationOrDefault("TEST_DURATION", 2*time.Hour)
		expected := 2 * time.Hour
		if result != expected {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})
}

func TestEnvLogLevelOrDefault(t *testing.T) {
	t.Run("returns environment log level when valid", func(t *testing.T) {
		defer setEnvVar(t, "TEST_LOG_LEVEL", "debug")()

		result := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.InfoLevel)
		if result != "debug" {
			t.Errorf("Expected 'debug', got '%s'", result)
		}
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_LOG_LEVEL")

		result := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.WarnLevel)
		if result != "warn" {
			t.Errorf("Expected 'warn', got '%s'", result)
		}
	})
}

func TestEnvBoolOrDefault(t *testing.T) {
	t.Run("returns environment bool when valid", func(t *testing.T) {
		testCases := []struct {
			envValue string
			expected bool
		}{
			{"true", true},
			{"false", false},
			{"1", true},
			{"0", false},
			{"t", true},
			{"f", false},
			{"T", true},
			{"F", false},
		}

		for _, tc := range testCases {
			func() {
				cleanup := setEnvVar(t, "TEST_BOOL", tc.envValue)
				defer cleanup()

				result := envBoolOrDefault("TEST_BOOL", false)
				if result != tc.expected {
					t.Errorf("For envValue '%s', expected %v, got %v", tc.envValue, tc.expected, result)
				}
			}()
		}

		unsetEnvVar(t, "TEST_BOOL")
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_BOOL")

		result := envBoolOrDefault("TEST_BOOL", true)
		if result != true {
			t.Errorf("Expected true, got %v", result)
		}
	})
}

// Note: Integration tests for Parse() are complex to test due to fatal logging calls
// The helper functions are thoroughly tested above and provide good coverage of the parsing logic
