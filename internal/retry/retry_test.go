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
	for i := 0; i < 10; i++ {
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
