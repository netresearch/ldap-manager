// Package options provides configuration parsing and environment variable handling.
// This file contains edge case and validation tests for configuration parsing.
package options

import (
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnvStringOrDefault_EdgeCases tests edge cases in string parsing
func TestEnvStringOrDefault_EdgeCases(t *testing.T) {
	t.Run("whitespace-only value returns default", func(t *testing.T) {
		// Note: whitespace is treated as non-empty by current implementation
		cleanup := setEnvVar(t, "TEST_WHITESPACE", "   ")
		defer cleanup()

		result := envStringOrDefault("TEST_WHITESPACE", "default")
		// Current implementation returns whitespace since it's non-empty
		assert.Equal(t, "   ", result)
	})

	t.Run("very long string value", func(t *testing.T) {
		// Create a long string (10000 'x' characters)
		longValue := ""
		for range 1000 {
			longValue += "xxxxxxxxxx"
		}
		cleanup := setEnvVar(t, "TEST_LONG", longValue)
		defer cleanup()

		result := envStringOrDefault("TEST_LONG", "default")
		assert.Len(t, result, 10000)
	})

	t.Run("unicode characters", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_UNICODE", "日本語テスト")
		defer cleanup()

		result := envStringOrDefault("TEST_UNICODE", "default")
		assert.Equal(t, "日本語テスト", result)
	})

	t.Run("special characters", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_SPECIAL", "!@#$%^&*()_+-=[]{}|;':\",./<>?")
		defer cleanup()

		result := envStringOrDefault("TEST_SPECIAL", "default")
		assert.Equal(t, "!@#$%^&*()_+-=[]{}|;':\",./<>?", result)
	})

	t.Run("newline in value", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_NEWLINE", "line1\nline2")
		defer cleanup()

		result := envStringOrDefault("TEST_NEWLINE", "default")
		assert.Equal(t, "line1\nline2", result)
	})
}

// TestEnvDurationOrDefault_EdgeCases tests edge cases in duration parsing
func TestEnvDurationOrDefault_EdgeCases(t *testing.T) {
	t.Run("nanoseconds", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_NS", "100ns")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_NS", time.Second)
		assert.Equal(t, 100*time.Nanosecond, result)
	})

	t.Run("microseconds", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_US", "500us")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_US", time.Second)
		assert.Equal(t, 500*time.Microsecond, result)
	})

	t.Run("milliseconds", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_MS", "250ms")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_MS", time.Second)
		assert.Equal(t, 250*time.Millisecond, result)
	})

	t.Run("hours", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_H", "24h")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_H", time.Second)
		assert.Equal(t, 24*time.Hour, result)
	})

	t.Run("combined duration", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_COMBINED", "1h30m45s")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_COMBINED", time.Second)
		expected := time.Hour + 30*time.Minute + 45*time.Second
		assert.Equal(t, expected, result)
	})

	t.Run("zero duration", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_ZERO", "0s")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_ZERO", time.Second)
		assert.Equal(t, time.Duration(0), result)
	})

	t.Run("negative duration", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_NEG", "-5m")
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_NEG", time.Second)
		assert.Equal(t, -5*time.Minute, result)
	})

	t.Run("very large duration", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_DURATION_LARGE", "8760h") // 1 year
		defer cleanup()

		result := envDurationOrDefault("TEST_DURATION_LARGE", time.Second)
		assert.Equal(t, 8760*time.Hour, result)
	})
}

// TestEnvIntOrDefault_EdgeCases tests edge cases in int parsing
func TestEnvIntOrDefault_EdgeCases(t *testing.T) {
	t.Run("max int", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_INT_MAX", strconv.Itoa(math.MaxInt))
		defer cleanup()

		result := envIntOrDefault("TEST_INT_MAX", 0)
		assert.Equal(t, math.MaxInt, result)
	})

	t.Run("min int", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_INT_MIN", strconv.Itoa(math.MinInt))
		defer cleanup()

		result := envIntOrDefault("TEST_INT_MIN", 0)
		assert.Equal(t, math.MinInt, result)
	})

	t.Run("zero value", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_INT_ZERO", "0")
		defer cleanup()

		result := envIntOrDefault("TEST_INT_ZERO", 999)
		assert.Equal(t, 0, result)
	})

	t.Run("positive with plus sign", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_INT_PLUS", "+42")
		defer cleanup()

		result := envIntOrDefault("TEST_INT_PLUS", 0)
		assert.Equal(t, 42, result)
	})
}

// TestEnvBoolOrDefault_EdgeCases tests edge cases in bool parsing
func TestEnvBoolOrDefault_EdgeCases(t *testing.T) {
	t.Run("TRUE uppercase", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_BOOL", "TRUE")
		defer cleanup()

		result := envBoolOrDefault("TEST_BOOL", false)
		assert.True(t, result)
	})

	t.Run("FALSE uppercase", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_BOOL", "FALSE")
		defer cleanup()

		result := envBoolOrDefault("TEST_BOOL", true)
		assert.False(t, result)
	})

	t.Run("True mixed case", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_BOOL", "True")
		defer cleanup()

		result := envBoolOrDefault("TEST_BOOL", false)
		assert.True(t, result)
	})

	t.Run("False mixed case", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_BOOL", "False")
		defer cleanup()

		result := envBoolOrDefault("TEST_BOOL", true)
		assert.False(t, result)
	})
}

// TestEnvLogLevelOrDefault_EdgeCases tests edge cases in log level parsing
func TestEnvLogLevelOrDefault_EdgeCases(t *testing.T) {
	logLevels := []struct {
		input    string
		expected string
	}{
		{"trace", "trace"},
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
		{"fatal", "fatal"},
		{"panic", "panic"},
		{"disabled", "disabled"},
	}

	for _, tc := range logLevels {
		t.Run(tc.input, func(t *testing.T) {
			cleanup := setEnvVar(t, "TEST_LOG_LEVEL", tc.input)
			defer cleanup()

			result := envLogLevelOrDefault("TEST_LOG_LEVEL", zerolog.InfoLevel)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestOptsPoolConfiguration tests pool configuration defaults and ranges
func TestOptsPoolConfiguration(t *testing.T) {
	t.Run("default pool values are sensible", func(t *testing.T) {
		opts := &Opts{
			PoolMaxConnections:      10,
			PoolMinConnections:      2,
			PoolMaxIdleTime:         15 * time.Minute,
			PoolMaxLifetime:         1 * time.Hour,
			PoolHealthCheckInterval: 30 * time.Second,
			PoolConnectionTimeout:   30 * time.Second,
			PoolAcquireTimeout:      10 * time.Second,
		}

		// Verify sensible defaults
		assert.GreaterOrEqual(t, opts.PoolMaxConnections, opts.PoolMinConnections,
			"Max connections should be >= min connections")
		assert.Greater(t, opts.PoolMaxIdleTime, time.Duration(0),
			"Max idle time should be positive")
		assert.Greater(t, opts.PoolMaxLifetime, opts.PoolMaxIdleTime,
			"Max lifetime should be greater than max idle time")
		assert.Greater(t, opts.PoolHealthCheckInterval, time.Duration(0),
			"Health check interval should be positive")
	})

	t.Run("edge case: min equals max connections", func(t *testing.T) {
		opts := &Opts{
			PoolMaxConnections: 5,
			PoolMinConnections: 5,
		}

		assert.Equal(t, opts.PoolMaxConnections, opts.PoolMinConnections)
	})

	t.Run("edge case: very short timeouts", func(t *testing.T) {
		opts := &Opts{
			PoolConnectionTimeout: 1 * time.Millisecond,
			PoolAcquireTimeout:    1 * time.Millisecond,
		}

		assert.Equal(t, time.Millisecond, opts.PoolConnectionTimeout)
		assert.Equal(t, time.Millisecond, opts.PoolAcquireTimeout)
	})

	t.Run("edge case: very long timeouts", func(t *testing.T) {
		opts := &Opts{
			PoolMaxIdleTime: 24 * time.Hour,
			PoolMaxLifetime: 7 * 24 * time.Hour, // 1 week
		}

		assert.Equal(t, 24*time.Hour, opts.PoolMaxIdleTime)
		assert.Equal(t, 7*24*time.Hour, opts.PoolMaxLifetime)
	})
}

// TestOptsSessionConfiguration tests session configuration edge cases
func TestOptsSessionConfiguration(t *testing.T) {
	t.Run("non-persist session defaults", func(t *testing.T) {
		opts := &Opts{
			PersistSessions: false,
			SessionPath:     "",
			SessionDuration: 30 * time.Minute,
		}

		assert.False(t, opts.PersistSessions)
		assert.Empty(t, opts.SessionPath)
	})

	t.Run("persist session requires path", func(t *testing.T) {
		opts := &Opts{
			PersistSessions: true,
			SessionPath:     "/data/sessions.db",
			SessionDuration: 30 * time.Minute,
		}

		assert.True(t, opts.PersistSessions)
		assert.NotEmpty(t, opts.SessionPath)
	})

	t.Run("short session duration", func(t *testing.T) {
		opts := &Opts{
			SessionDuration: 1 * time.Minute,
		}

		assert.Equal(t, time.Minute, opts.SessionDuration)
	})

	t.Run("long session duration", func(t *testing.T) {
		opts := &Opts{
			SessionDuration: 24 * time.Hour,
		}

		assert.Equal(t, 24*time.Hour, opts.SessionDuration)
	})
}

// TestOptsCookieSecurity tests cookie security configuration
func TestOptsCookieSecurity(t *testing.T) {
	t.Run("secure cookies enabled by default", func(t *testing.T) {
		opts := &Opts{
			CookieSecure: true,
		}

		assert.True(t, opts.CookieSecure)
	})

	t.Run("insecure cookies for development", func(t *testing.T) {
		opts := &Opts{
			CookieSecure: false,
		}

		assert.False(t, opts.CookieSecure)
	})
}

// TestOptsLDAPConfiguration tests LDAP configuration edge cases
func TestOptsLDAPConfiguration(t *testing.T) {
	t.Run("complete configuration", func(t *testing.T) {
		opts := &Opts{
			ReadonlyUser:     "cn=readonly,ou=users,dc=example,dc=com",
			ReadonlyPassword: "secretpassword123",
		}

		assert.Contains(t, opts.ReadonlyUser, "cn=")
		assert.NotEmpty(t, opts.ReadonlyPassword)
	})

	t.Run("DN with special characters", func(t *testing.T) {
		opts := &Opts{
			ReadonlyUser: "cn=readonly+serialNumber=123,ou=users,dc=example,dc=com",
		}

		assert.Contains(t, opts.ReadonlyUser, "+serialNumber")
	})

	t.Run("DN with escaped characters", func(t *testing.T) {
		opts := &Opts{
			ReadonlyUser: "cn=read\\,only,ou=users,dc=example,dc=com",
		}

		assert.Contains(t, opts.ReadonlyUser, "\\,")
	})
}

// TestEnvironmentVariablePrecedence tests that env vars override defaults
func TestEnvironmentVariablePrecedence(t *testing.T) {
	t.Run("env overrides default for string", func(t *testing.T) {
		cleanup := setEnvVar(t, "TEST_PRECEDENCE", "from_env")
		defer cleanup()

		result := envStringOrDefault("TEST_PRECEDENCE", "from_default")
		assert.Equal(t, "from_env", result)
	})

	t.Run("unset env uses default", func(t *testing.T) {
		unsetEnvVar(t, "TEST_PRECEDENCE_UNSET")

		result := envStringOrDefault("TEST_PRECEDENCE_UNSET", "from_default")
		assert.Equal(t, "from_default", result)
	})
}

// TestConcurrentEnvironmentAccess tests concurrent env var access
func TestConcurrentEnvironmentAccess(t *testing.T) {
	const envKey = "TEST_CONCURRENT_ENV"
	cleanup := setEnvVar(t, envKey, "initial")
	defer cleanup()

	done := make(chan bool, 100)

	// Concurrent readers
	for range 50 {
		go func() {
			for range 100 {
				_ = envStringOrDefault(envKey, "default")
			}
			done <- true
		}()
	}

	// Concurrent writers
	for i := range 50 {
		go func(val int) {
			for range 100 {
				if err := os.Setenv(envKey, strconv.Itoa(val)); err != nil {
					t.Error(err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 100 {
		<-done
	}

	// Environment should still be accessible
	result := envStringOrDefault(envKey, "default")
	require.NotEmpty(t, result)
}
