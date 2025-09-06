package options

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestEnvStringOrDefault(t *testing.T) {
	t.Run("returns environment value when set", func(t *testing.T) {
		os.Setenv("TEST_VAR", "env_value")
		defer os.Unsetenv("TEST_VAR")
		
		result := envStringOrDefault("TEST_VAR", "default_value")
		if result != "env_value" {
			t.Errorf("Expected 'env_value', got '%s'", result)
		}
	})
	
	t.Run("returns default when environment variable not set", func(t *testing.T) {
		os.Unsetenv("TEST_VAR")
		
		result := envStringOrDefault("TEST_VAR", "default_value")
		if result != "default_value" {
			t.Errorf("Expected 'default_value', got '%s'", result)
		}
	})
	
	t.Run("returns default when environment variable is empty", func(t *testing.T) {
		os.Setenv("TEST_VAR", "")
		defer os.Unsetenv("TEST_VAR")
		
		result := envStringOrDefault("TEST_VAR", "default_value")
		if result != "default_value" {
			t.Errorf("Expected 'default_value', got '%s'", result)
		}
	})
}

func TestEnvDurationOrDefault(t *testing.T) {
	t.Run("returns environment duration when valid", func(t *testing.T) {
		os.Setenv("TEST_DURATION", "5m")
		defer os.Unsetenv("TEST_DURATION")
		
		result := envDurationOrDefault("TEST_DURATION", 1*time.Minute)
		expected := 5 * time.Minute
		if result != expected {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})
	
	t.Run("returns default when environment variable not set", func(t *testing.T) {
		os.Unsetenv("TEST_DURATION")
		
		result := envDurationOrDefault("TEST_DURATION", 2*time.Hour)
		expected := 2 * time.Hour
		if result != expected {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})
}

func TestEnvLogLevelOrDefault(t *testing.T) {
	t.Run("returns environment log level when valid", func(t *testing.T) {
		os.Setenv("TEST_LOG_LEVEL", "debug")
		defer os.Unsetenv("TEST_LOG_LEVEL")
		
		result := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.InfoLevel)
		if result != "debug" {
			t.Errorf("Expected 'debug', got '%s'", result)
		}
	})
	
	t.Run("returns default when environment variable not set", func(t *testing.T) {
		os.Unsetenv("TEST_LOG_LEVEL")
		
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
			os.Setenv("TEST_BOOL", tc.envValue)
			
			result := envBoolOrDefault("TEST_BOOL", false)
			if result != tc.expected {
				t.Errorf("For envValue '%s', expected %v, got %v", tc.envValue, tc.expected, result)
			}
		}
		
		os.Unsetenv("TEST_BOOL")
	})
	
	t.Run("returns default when environment variable not set", func(t *testing.T) {
		os.Unsetenv("TEST_BOOL")
		
		result := envBoolOrDefault("TEST_BOOL", true)
		if result != true {
			t.Errorf("Expected true, got %v", result)
		}
	})
}

// Note: Integration tests for Parse() are complex to test due to fatal logging calls
// The helper functions are thoroughly tested above and provide good coverage of the parsing logic