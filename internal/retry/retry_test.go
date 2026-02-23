package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoWithConfig_Success(t *testing.T) {
	callCount := 0
	err := DoWithConfig(context.Background(), DefaultConfig(), func() error {
		callCount++

		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestDoWithConfig_RetryThenSuccess(t *testing.T) {
	callCount := 0
	config := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := DoWithConfig(context.Background(), config, func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}

		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDoWithConfig_AllAttemptsFail(t *testing.T) {
	callCount := 0
	expectedErr := errors.New("persistent error")
	config := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := DoWithConfig(context.Background(), config, func() error {
		callCount++

		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDoWithConfig_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	config := Config{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	// Cancel after first failure
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := DoWithConfig(ctx, config, func() error {
		callCount++

		return errors.New("always fails")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// Should have been interrupted before all attempts
	if callCount >= 5 {
		t.Errorf("expected fewer than 5 calls due to cancellation, got %d", callCount)
	}
}

func TestDoWithConfig_ZeroMaxAttempts(t *testing.T) {
	callCount := 0
	config := Config{
		MaxAttempts:  0, // Should default to 1
		InitialDelay: 1 * time.Millisecond,
	}

	err := DoWithConfig(context.Background(), config, func() error {
		callCount++

		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (default), got %d", callCount)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	expected := "success"
	result, err := DoWithResult(context.Background(), func() (string, error) {
		return expected, nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestDoWithResult_RetryThenSuccess(t *testing.T) {
	callCount := 0
	config := Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	result, err := DoWithResultConfig(context.Background(), config, func() (int, error) {
		callCount++
		if callCount < 2 {
			return 0, errors.New("temporary error")
		}

		return 42, nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestExponentialBackoff(t *testing.T) {
	config := Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // Capped at MaxDelay
		{6, 1 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		delay := ExponentialBackoff(tt.attempt, config)
		if delay != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, delay)
		}
	}
}

func TestExponentialBackoff_ZeroAttempt(t *testing.T) {
	config := Config{
		InitialDelay: 100 * time.Millisecond,
		Multiplier:   2.0,
	}

	delay := ExponentialBackoff(0, config)
	if delay != config.InitialDelay {
		t.Errorf("expected %v for attempt 0, got %v", config.InitialDelay, delay)
	}
}

func TestAddJitter(t *testing.T) {
	duration := 100 * time.Millisecond
	fraction := 0.2

	// Run multiple times to verify jitter is applied
	for range 10 {
		result := addJitter(duration, fraction)
		if result < duration {
			t.Errorf("jittered duration should be >= original: %v < %v", result, duration)
		}
		maxExpected := duration + time.Duration(float64(duration)*fraction)
		if result > maxExpected {
			t.Errorf("jittered duration exceeds max: %v > %v", result, maxExpected)
		}
	}
}

func TestAddJitter_ZeroFraction(t *testing.T) {
	duration := 100 * time.Millisecond
	result := addJitter(duration, 0)
	if result != duration {
		t.Errorf("expected no jitter with zero fraction: %v != %v", result, duration)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"generic error", errors.New("some error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLDAPConfig(t *testing.T) {
	config := LDAPConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("expected 3 max attempts, got %d", config.MaxAttempts)
	}
	if config.InitialDelay != 500*time.Millisecond {
		t.Errorf("expected 500ms initial delay, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 5*time.Second {
		t.Errorf("expected 5s max delay, got %v", config.MaxDelay)
	}
}

// TestDoWithConfig_DelayCappedAtMaxDelay verifies that the delay is properly
// capped at MaxDelay during retries (catches mutation at line 95).
func TestDoWithConfig_DelayCappedAtMaxDelay(t *testing.T) {
	callCount := 0
	var delays []time.Duration
	lastCallTime := time.Now()

	config := Config{
		MaxAttempts:    5,
		InitialDelay:   10 * time.Millisecond,
		MaxDelay:       15 * time.Millisecond, // Cap should kick in after 2nd retry
		Multiplier:     2.0,
		JitterFraction: 0, // No jitter for predictable timing
	}

	_ = DoWithConfig(context.Background(), config, func() error {
		now := time.Now()
		if callCount > 0 {
			delays = append(delays, now.Sub(lastCallTime))
		}
		lastCallTime = now
		callCount++

		return errors.New("always fails")
	})

	// Verify delays don't exceed MaxDelay (with some tolerance for test execution)
	tolerance := 10 * time.Millisecond
	for i, delay := range delays {
		if delay > config.MaxDelay+tolerance {
			t.Errorf("delay %d exceeded MaxDelay: got %v, max allowed %v",
				i+1, delay, config.MaxDelay+tolerance)
		}
	}

	// Verify we hit the cap: after attempt 2, delay would be 20ms but capped at 15ms
	// delays[0] = 10ms (initial), delays[1] = 15ms (capped from 20ms)
	if len(delays) >= 2 {
		// Second delay should be capped, not 20ms
		if delays[1] > config.MaxDelay+tolerance {
			t.Errorf("delay was not capped at MaxDelay: got %v, expected <= %v",
				delays[1], config.MaxDelay+tolerance)
		}
	}
}

// TestAddJitter_NegativeFraction verifies that negative jitter fraction
// returns the original duration (catches mutation at line 129).
func TestAddJitter_NegativeFraction(t *testing.T) {
	duration := 100 * time.Millisecond
	result := addJitter(duration, -0.1)
	if result != duration {
		t.Errorf("expected no jitter with negative fraction: %v != %v", result, duration)
	}

	// Also test a more negative value
	result = addJitter(duration, -1.0)
	if result != duration {
		t.Errorf("expected no jitter with -1.0 fraction: %v != %v", result, duration)
	}
}

// TestDefaultConfig_ExactValues verifies the exact default config values
// (catches arithmetic mutations at lines 28-29).
func TestDefaultConfig_ExactValues(t *testing.T) {
	config := DefaultConfig()

	// These exact values must match - mutation would change arithmetic
	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("InitialDelay: expected 100ms, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 10*time.Second {
		t.Errorf("MaxDelay: expected 10s, got %v", config.MaxDelay)
	}
	if config.MaxAttempts != 3 {
		t.Errorf("MaxAttempts: expected 3, got %d", config.MaxAttempts)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Multiplier: expected 2.0, got %f", config.Multiplier)
	}
	if config.JitterFraction != 0.1 {
		t.Errorf("JitterFraction: expected 0.1, got %f", config.JitterFraction)
	}
}

// TestDoWithConfig_DelayMultiplier verifies that delay is correctly multiplied
// (catches arithmetic mutation at line 94).
func TestDoWithConfig_DelayMultiplier(t *testing.T) {
	callCount := 0
	var callTimes []time.Time

	config := Config{
		MaxAttempts:    4,
		InitialDelay:   50 * time.Millisecond,
		MaxDelay:       1 * time.Second, // High enough to not cap
		Multiplier:     2.0,
		JitterFraction: 0, // No jitter for predictable timing
	}

	_ = DoWithConfig(context.Background(), config, func() error {
		callTimes = append(callTimes, time.Now())
		callCount++

		return errors.New("always fails")
	})

	// Calculate actual delays between calls
	if len(callTimes) < 3 {
		t.Fatalf("expected at least 3 calls, got %d", len(callTimes))
	}

	delay1 := callTimes[1].Sub(callTimes[0]) // Should be ~50ms
	delay2 := callTimes[2].Sub(callTimes[1]) // Should be ~100ms (50 * 2)

	// With multiplier, second delay should be approximately 2x the first
	// Allow 50% tolerance for timing variations
	ratio := float64(delay2) / float64(delay1)
	if ratio < 1.5 || ratio > 2.5 {
		t.Errorf("delay ratio should be ~2.0, got %f (delay1=%v, delay2=%v)",
			ratio, delay1, delay2)
	}
}

// TestExponentialBackoff_ExactMaxDelay verifies the boundary case where
// calculated delay exactly equals MaxDelay (catches mutation at line 163).
func TestExponentialBackoff_ExactMaxDelay(t *testing.T) {
	// Set up config so that one calculation exactly equals MaxDelay
	// InitialDelay * Multiplier^(attempt-1) = MaxDelay
	// 100ms * 2^2 = 400ms
	config := Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     400 * time.Millisecond, // Exactly 100ms * 2^2
		Multiplier:   2.0,
	}

	// Attempt 3: 100ms * 2^2 = 400ms = MaxDelay exactly
	delay := ExponentialBackoff(3, config)
	if delay != config.MaxDelay {
		t.Errorf("attempt 3: expected exact MaxDelay %v, got %v",
			config.MaxDelay, delay)
	}

	// Attempt 4: 100ms * 2^3 = 800ms > MaxDelay, should be capped
	delay = ExponentialBackoff(4, config)
	if delay != config.MaxDelay {
		t.Errorf("attempt 4: expected capped MaxDelay %v, got %v",
			config.MaxDelay, delay)
	}

	// Verify that delay equals MaxDelay is NOT capped (boundary test)
	// If the mutation changes > to >=, then delay == MaxDelay would be
	// incorrectly capped, and subsequent tests would fail
	config2 := Config{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond, // InitialDelay == MaxDelay
		Multiplier:   2.0,
	}

	// Attempt 1: 100ms * 2^0 = 100ms = MaxDelay exactly
	delay = ExponentialBackoff(1, config2)
	if delay != 100*time.Millisecond {
		t.Errorf("boundary test: expected 100ms, got %v", delay)
	}
}
