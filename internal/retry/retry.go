// Package retry provides retry logic with exponential backoff for resilient operations.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2" //nolint:gosec // Weak random is acceptable for jitter calculation
	"time"

	"github.com/rs/zerolog/log"
)

// Config holds retry configuration parameters.
type Config struct {
	MaxAttempts     int           // Maximum number of attempts (default: 3)
	InitialDelay    time.Duration // Initial delay between retries (default: 100ms)
	MaxDelay        time.Duration // Maximum delay between retries (default: 10s)
	Multiplier      float64       // Backoff multiplier (default: 2.0)
	JitterFraction  float64       // Jitter fraction 0-1 to prevent thundering herd (default: 0.1)
	RetryableErrors []error       // If set, only retry these specific errors
}

// DefaultConfig returns sensible default retry configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:    3,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       10 * time.Second,
		Multiplier:     2.0,
		JitterFraction: 0.1,
	}
}

// LDAPConfig returns retry configuration optimized for LDAP operations.
func LDAPConfig() Config {
	return Config{
		MaxAttempts:    3,
		InitialDelay:   500 * time.Millisecond,
		MaxDelay:       5 * time.Second,
		Multiplier:     2.0,
		JitterFraction: 0.15,
	}
}

// Do executes the operation with retry logic using the default configuration.
func Do(ctx context.Context, operation func() error) error {
	return DoWithConfig(ctx, DefaultConfig(), operation)
}

// DoWithConfig executes the operation with retry logic using the provided configuration.
func DoWithConfig(ctx context.Context, config Config, operation func() error) error {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 1
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Log retry attempt
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_attempts", config.MaxAttempts).
			Dur("next_delay", delay).
			Msg("Operation failed, retrying")

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(addJitter(delay, config.JitterFraction)):
		}

		// Calculate next delay with exponential backoff
		delay = min(time.Duration(float64(delay)*config.Multiplier), config.MaxDelay)
	}

	log.Error().
		Err(lastErr).
		Int("attempts", config.MaxAttempts).
		Msg("Operation failed after all retry attempts")

	return lastErr
}

// DoWithResult executes an operation that returns a value with retry logic.
func DoWithResult[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	return DoWithResultConfig(ctx, DefaultConfig(), operation)
}

// DoWithResultConfig executes an operation that returns a value with retry logic and custom config.
func DoWithResultConfig[T any](ctx context.Context, config Config, operation func() (T, error)) (T, error) {
	var result T

	err := DoWithConfig(ctx, config, func() error {
		var opErr error
		result, opErr = operation()

		return opErr
	})

	return result, err
}

// addJitter adds random jitter to prevent thundering herd problem.
func addJitter(duration time.Duration, fraction float64) time.Duration {
	if fraction <= 0 {
		return duration
	}

	jitter := float64(duration) * fraction * rand.Float64() //nolint:gosec // Weak random acceptable for jitter

	return duration + time.Duration(jitter)
}

// IsRetryable checks if an error should trigger a retry.
// Network errors, timeouts, and temporary errors are typically retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation - not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Most LDAP errors are retryable (connection issues, timeouts)
	// Specific non-retryable errors (auth failures, invalid credentials) should be
	// handled by the caller before retrying
	return true
}

// ExponentialBackoff calculates the delay for a given attempt number.
func ExponentialBackoff(attempt int, config Config) time.Duration {
	if attempt <= 0 {
		return config.InitialDelay
	}

	delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt-1))
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	return time.Duration(delay)
}
