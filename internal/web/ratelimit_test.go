package web

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRateLimiter_RecordAttempt(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  3,
		WindowPeriod: 1 * time.Minute,
		BlockPeriod:  1 * time.Minute,
		CleanupEvery: 1 * time.Hour, // Don't cleanup during test
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.1"

	// First attempt should not block
	if blocked := rl.RecordAttempt(ip); blocked {
		t.Error("First attempt should not block")
	}

	// Second attempt should not block
	if blocked := rl.RecordAttempt(ip); blocked {
		t.Error("Second attempt should not block")
	}

	// Third attempt should block (maxAttempts = 3)
	if blocked := rl.RecordAttempt(ip); !blocked {
		t.Error("Third attempt should block")
	}

	// Should still be blocked
	if !rl.IsBlocked(ip) {
		t.Error("IP should be blocked after max attempts")
	}
}

func TestRateLimiter_IsBlocked(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  2,
		WindowPeriod: 1 * time.Minute,
		BlockPeriod:  100 * time.Millisecond,
		CleanupEvery: 1 * time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.2"

	// Not blocked initially
	if rl.IsBlocked(ip) {
		t.Error("IP should not be blocked initially")
	}

	// Record max attempts
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip) // Should trigger block

	// Should be blocked now
	if !rl.IsBlocked(ip) {
		t.Error("IP should be blocked after max attempts")
	}

	// Wait for block to expire
	time.Sleep(150 * time.Millisecond)

	// Should no longer be blocked
	if rl.IsBlocked(ip) {
		t.Error("IP should not be blocked after block period expires")
	}
}

func TestRateLimiter_ResetAttempts(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  3,
		WindowPeriod: 1 * time.Minute,
		BlockPeriod:  1 * time.Minute,
		CleanupEvery: 1 * time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.3"

	// Record some attempts
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip)

	// Reset attempts
	rl.ResetAttempts(ip)

	// Should have max attempts again
	remaining := rl.GetRemainingAttempts(ip)
	if remaining != config.MaxAttempts {
		t.Errorf("Expected %d remaining attempts after reset, got %d", config.MaxAttempts, remaining)
	}
}

func TestRateLimiter_GetRemainingAttempts(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  5,
		WindowPeriod: 1 * time.Minute,
		BlockPeriod:  1 * time.Minute,
		CleanupEvery: 1 * time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.4"

	// Initially should have max attempts
	if remaining := rl.GetRemainingAttempts(ip); remaining != 5 {
		t.Errorf("Expected 5 remaining attempts, got %d", remaining)
	}

	// After 2 attempts
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip)

	if remaining := rl.GetRemainingAttempts(ip); remaining != 3 {
		t.Errorf("Expected 3 remaining attempts, got %d", remaining)
	}

	// After being blocked
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip) // This triggers block

	if remaining := rl.GetRemainingAttempts(ip); remaining != 0 {
		t.Errorf("Expected 0 remaining attempts when blocked, got %d", remaining)
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  3,
		WindowPeriod: 100 * time.Millisecond,
		BlockPeriod:  1 * time.Minute,
		CleanupEvery: 1 * time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.5"

	// Record 2 attempts
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip)

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Next attempt should reset the counter
	if blocked := rl.RecordAttempt(ip); blocked {
		t.Error("Attempt after window expiry should not block")
	}

	// Should have 2 remaining (just used 1)
	if remaining := rl.GetRemainingAttempts(ip); remaining != 2 {
		t.Errorf("Expected 2 remaining attempts after window reset, got %d", remaining)
	}
}

func TestRateLimiter_MultipleIPs(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  2,
		WindowPeriod: 1 * time.Minute,
		BlockPeriod:  1 * time.Minute,
		CleanupEvery: 1 * time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip1 := "192.168.1.10"
	ip2 := "192.168.1.11"

	// Block ip1
	rl.RecordAttempt(ip1)
	rl.RecordAttempt(ip1)

	// ip1 should be blocked
	if !rl.IsBlocked(ip1) {
		t.Error("ip1 should be blocked")
	}

	// ip2 should not be blocked
	if rl.IsBlocked(ip2) {
		t.Error("ip2 should not be blocked")
	}

	// ip2 should have full attempts
	if remaining := rl.GetRemainingAttempts(ip2); remaining != 2 {
		t.Errorf("ip2 should have 2 remaining attempts, got %d", remaining)
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	config := DefaultRateLimiterConfig()

	if config.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts 5, got %d", config.MaxAttempts)
	}
	if config.WindowPeriod != 15*time.Minute {
		t.Errorf("Expected WindowPeriod 15m, got %v", config.WindowPeriod)
	}
	if config.BlockPeriod != 15*time.Minute {
		t.Errorf("Expected BlockPeriod 15m, got %v", config.BlockPeriod)
	}
	if config.CleanupEvery != 5*time.Minute {
		t.Errorf("Expected CleanupEvery 5m, got %v", config.CleanupEvery)
	}
}

func TestRateLimiter_ZeroConfig(t *testing.T) {
	// Test that zero values get defaults
	config := RateLimiterConfig{}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Should use defaults
	if rl.maxAttempts <= 0 {
		t.Error("maxAttempts should have a positive default")
	}
	if rl.windowPeriod <= 0 {
		t.Error("windowPeriod should have a positive default")
	}
	if rl.blockPeriod <= 0 {
		t.Error("blockPeriod should have a positive default")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  2,
		WindowPeriod: 50 * time.Millisecond,
		BlockPeriod:  50 * time.Millisecond,
		CleanupEvery: 100 * time.Millisecond,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.20"

	// Record an attempt
	rl.RecordAttempt(ip)

	// Wait for cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Entry should be cleaned up, so remaining should be max
	if remaining := rl.GetRemainingAttempts(ip); remaining != config.MaxAttempts {
		t.Errorf("Expected %d remaining after cleanup, got %d", config.MaxAttempts, remaining)
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	t.Run("allows requests when not blocked", func(t *testing.T) {
		config := RateLimiterConfig{
			MaxAttempts:  3,
			WindowPeriod: 1 * time.Minute,
			BlockPeriod:  1 * time.Minute,
			CleanupEvery: 1 * time.Hour,
		}
		rl := NewRateLimiter(config)
		defer rl.Stop()

		app := fiber.New()
		app.Use(rl.Middleware())
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "OK" {
			t.Errorf("Expected body 'OK', got '%s'", string(body))
		}
	})

	t.Run("blocks requests when IP is rate limited", func(t *testing.T) {
		config := RateLimiterConfig{
			MaxAttempts:  2,
			WindowPeriod: 1 * time.Minute,
			BlockPeriod:  1 * time.Minute,
			CleanupEvery: 1 * time.Hour,
		}
		rl := NewRateLimiter(config)
		defer rl.Stop()

		// Block the IP by recording max attempts
		rl.RecordAttempt("0.0.0.0") // Fiber test uses 0.0.0.0 as default IP
		rl.RecordAttempt("0.0.0.0")

		app := fiber.New()
		app.Use(rl.Middleware())
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("Failed to test app: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != fiber.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "Too many failed login attempts. Please try again later." {
			t.Errorf("Expected rate limit message, got '%s'", string(body))
		}
	})
}
