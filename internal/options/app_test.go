package options

import (
	"errors"
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

		result, err := envDurationOrDefault("TEST_DURATION", 1*time.Minute)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := 5 * time.Minute
		if result != expected {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_DURATION")

		result, err := envDurationOrDefault("TEST_DURATION", 2*time.Hour)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := 2 * time.Hour
		if result != expected {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("returns error for invalid duration", func(t *testing.T) {
		defer setEnvVar(t, "TEST_DURATION", "invalid")()

		_, err := envDurationOrDefault("TEST_DURATION", 1*time.Minute)
		if err == nil {
			t.Error("Expected error for invalid duration, got nil")
		}

		validationErr, ok := errors.AsType[ValidationError](err)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if validationErr.Field != "TEST_DURATION" {
			t.Errorf("Expected field 'TEST_DURATION', got '%s'", validationErr.Field)
		}
	})
}

func TestEnvLogLevelOrDefault(t *testing.T) {
	t.Run("returns environment log level when valid", func(t *testing.T) {
		defer setEnvVar(t, "TEST_LOG_LEVEL", "debug")()

		result, err := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.InfoLevel)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != "debug" {
			t.Errorf("Expected 'debug', got '%s'", result)
		}
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_LOG_LEVEL")

		result, err := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.WarnLevel)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != "warn" {
			t.Errorf("Expected 'warn', got '%s'", result)
		}
	})

	t.Run("returns error for invalid log level", func(t *testing.T) {
		defer setEnvVar(t, "TEST_LOG_LEVEL", "invalid_level")()

		_, err := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.InfoLevel)
		if err == nil {
			t.Error("Expected error for invalid log level, got nil")
		}

		validationErr, ok := errors.AsType[ValidationError](err)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if validationErr.Field != "TEST_LOG_LEVEL" {
			t.Errorf("Expected field 'TEST_LOG_LEVEL', got '%s'", validationErr.Field)
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

				result, err := envBoolOrDefault("TEST_BOOL", false)
				if err != nil {
					t.Fatalf("Unexpected error for %s: %v", tc.envValue, err)
				}
				if result != tc.expected {
					t.Errorf("For envValue '%s', expected %v, got %v", tc.envValue, tc.expected, result)
				}
			}()
		}

		unsetEnvVar(t, "TEST_BOOL")
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_BOOL")

		result, err := envBoolOrDefault("TEST_BOOL", true)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("Expected true, got %v", result)
		}
	})

	t.Run("returns error for invalid bool", func(t *testing.T) {
		defer setEnvVar(t, "TEST_BOOL", "not_a_bool")()

		_, err := envBoolOrDefault("TEST_BOOL", false)
		if err == nil {
			t.Error("Expected error for invalid bool, got nil")
		}

		validationErr, ok := errors.AsType[ValidationError](err)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if validationErr.Field != "TEST_BOOL" {
			t.Errorf("Expected field 'TEST_BOOL', got '%s'", validationErr.Field)
		}
	})
}

func TestEnvIntOrDefault(t *testing.T) {
	t.Run("returns environment int when valid", func(t *testing.T) {
		defer setEnvVar(t, "TEST_INT", "42")()

		result, err := envIntOrDefault("TEST_INT", 10)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("Expected 42, got %d", result)
		}
	})

	t.Run("returns default when environment variable not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_INT")

		result, err := envIntOrDefault("TEST_INT", 100)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != 100 {
			t.Errorf("Expected 100, got %d", result)
		}
	})

	t.Run("returns default value of zero when env var not set", func(t *testing.T) {
		unsetEnvVar(t, "TEST_INT")

		result, err := envIntOrDefault("TEST_INT", 0)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != 0 {
			t.Errorf("Expected 0, got %d", result)
		}
	})

	t.Run("handles negative int values", func(t *testing.T) {
		defer setEnvVar(t, "TEST_INT", "-123")()

		result, err := envIntOrDefault("TEST_INT", 10)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result != -123 {
			t.Errorf("Expected -123, got %d", result)
		}
	})

	t.Run("returns error for invalid int", func(t *testing.T) {
		defer setEnvVar(t, "TEST_INT", "not_an_int")()

		_, err := envIntOrDefault("TEST_INT", 10)
		if err == nil {
			t.Error("Expected error for invalid int, got nil")
		}

		validationErr, ok := errors.AsType[ValidationError](err)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if validationErr.Field != "TEST_INT" {
			t.Errorf("Expected field 'TEST_INT', got '%s'", validationErr.Field)
		}
	})
}

func TestValidateRequired(t *testing.T) {
	t.Run("returns nil for non-empty value", func(t *testing.T) {
		value := "some_value"
		err := validateRequired("test-field", &value)
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})

	t.Run("returns error for empty value", func(t *testing.T) {
		value := ""
		err := validateRequired("test-field", &value)
		if err == nil {
			t.Error("Expected error for empty value, got nil")
		}

		validationErr, ok := errors.AsType[ValidationError](err)
		if !ok {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if validationErr.Field != "test-field" {
			t.Errorf("Expected field 'test-field', got '%s'", validationErr.Field)
		}
	})
}

func TestValidationError(t *testing.T) {
	err := ValidationError{Field: "test-field", Message: "test message"}
	expected := "configuration error for test-field: test message"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestOptsStructure(t *testing.T) {
	opts := &Opts{
		LogLevel:                zerolog.DebugLevel,
		ReadonlyUser:            "cn=readonly,dc=example,dc=com",
		ReadonlyPassword:        "secret",
		PersistSessions:         true,
		SessionPath:             "/data/sessions.db",
		SessionDuration:         30 * time.Minute,
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolMaxLifetime:         1 * time.Hour,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Verify struct fields are properly set
	if opts.LogLevel != zerolog.DebugLevel {
		t.Errorf("Expected DebugLevel, got %v", opts.LogLevel)
	}
	if opts.ReadonlyUser != "cn=readonly,dc=example,dc=com" {
		t.Errorf("Expected readonly user, got %s", opts.ReadonlyUser)
	}
	if opts.PersistSessions != true {
		t.Error("Expected PersistSessions to be true")
	}
	if opts.PoolMaxConnections != 10 {
		t.Errorf("Expected 10 max connections, got %d", opts.PoolMaxConnections)
	}
}
