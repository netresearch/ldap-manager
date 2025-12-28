package web

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// RateLimiter provides IP-based rate limiting for authentication endpoints.
// It tracks failed login attempts and blocks IPs that exceed the threshold.
type RateLimiter struct {
	mu           sync.RWMutex
	attempts     map[string]*rateLimitEntry
	maxAttempts  int           // Maximum attempts before blocking
	windowPeriod time.Duration // Time window for counting attempts
	blockPeriod  time.Duration // How long to block after exceeding limit
	cleanupEvery time.Duration // Cleanup interval for expired entries
	stopCleanup  chan struct{}
	stopOnce     sync.Once // Ensures Stop() is idempotent
}

type rateLimitEntry struct {
	count     int
	firstSeen time.Time
	blockedAt time.Time
}

// RateLimiterConfig holds rate limiter configuration.
type RateLimiterConfig struct {
	MaxAttempts  int           // Max failed attempts before blocking (default: 5)
	WindowPeriod time.Duration // Window to count attempts (default: 15 minutes)
	BlockPeriod  time.Duration // Block duration after exceeding limit (default: 15 minutes)
	CleanupEvery time.Duration // Cleanup interval (default: 5 minutes)
}

// DefaultRateLimiterConfig returns sensible defaults for authentication rate limiting.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		MaxAttempts:  5,
		WindowPeriod: 15 * time.Minute,
		BlockPeriod:  15 * time.Minute,
		CleanupEvery: 5 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 5
	}
	if config.WindowPeriod <= 0 {
		config.WindowPeriod = 15 * time.Minute
	}
	if config.BlockPeriod <= 0 {
		config.BlockPeriod = 15 * time.Minute
	}
	if config.CleanupEvery <= 0 {
		config.CleanupEvery = 5 * time.Minute
	}

	rl := &RateLimiter{
		attempts:     make(map[string]*rateLimitEntry),
		maxAttempts:  config.MaxAttempts,
		windowPeriod: config.WindowPeriod,
		blockPeriod:  config.BlockPeriod,
		cleanupEvery: config.CleanupEvery,
		stopCleanup:  make(chan struct{}),
	}

	// Start background cleanup
	go rl.startCleanup()

	return rl
}

// RecordAttempt records a failed login attempt for the given IP.
// Returns true if the IP should be blocked, false otherwise.
func (rl *RateLimiter) RecordAttempt(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.attempts[ip]

	if !exists {
		rl.attempts[ip] = &rateLimitEntry{
			count:     1,
			firstSeen: now,
		}

		return false
	}

	// If blocked, check if block period has expired
	if !entry.blockedAt.IsZero() {
		if now.Sub(entry.blockedAt) > rl.blockPeriod {
			// Block expired, reset
			entry.count = 1
			entry.firstSeen = now
			entry.blockedAt = time.Time{}

			return false
		}
		// Still blocked

		return true
	}

	// Check if window has expired
	if now.Sub(entry.firstSeen) > rl.windowPeriod {
		// Window expired, reset counter
		entry.count = 1
		entry.firstSeen = now

		return false
	}

	// Increment counter
	entry.count++

	// Check if should be blocked
	if entry.count >= rl.maxAttempts {
		entry.blockedAt = now
		log.Warn().
			Str("ip", ip).
			Int("attempts", entry.count).
			Msg("IP blocked due to too many failed login attempts")

		return true
	}

	return false
}

// IsBlocked checks if an IP is currently blocked.
// Note: Expired entries are not cleaned up here to avoid lock upgrades;
// cleanup happens periodically via startCleanup goroutine.
func (rl *RateLimiter) IsBlocked(ip string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.attempts[ip]
	if !exists {
		return false
	}

	if entry.blockedAt.IsZero() {
		return false
	}

	// Check if block has expired
	if time.Since(entry.blockedAt) > rl.blockPeriod {
		return false
	}

	return true
}

// ResetAttempts clears attempts for an IP (call on successful login).
func (rl *RateLimiter) ResetAttempts(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, ip)
}

// startCleanup runs periodic cleanup of expired entries.
func (rl *RateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.cleanupEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes expired entries from the rate limiter.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, entry := range rl.attempts {
		// Remove if block has expired
		if !entry.blockedAt.IsZero() && now.Sub(entry.blockedAt) > rl.blockPeriod {
			delete(rl.attempts, ip)

			continue
		}

		// Remove if window has expired and not blocked
		if entry.blockedAt.IsZero() && now.Sub(entry.firstSeen) > rl.windowPeriod {
			delete(rl.attempts, ip)
		}
	}
}

// Stop gracefully stops the rate limiter cleanup goroutine.
// Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stopCleanup)
	})
}

// Middleware creates a Fiber middleware for rate limiting.
func (rl *RateLimiter) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()

		// Check if blocked before processing
		if rl.IsBlocked(ip) {
			log.Warn().
				Str("ip", ip).
				Str("path", c.Path()).
				Msg("Rate limited request blocked")

			return c.Status(fiber.StatusTooManyRequests).
				SendString("Too many failed login attempts. Please try again later.")
		}

		return c.Next()
	}
}

// GetRemainingAttempts returns the number of remaining attempts for an IP.
func (rl *RateLimiter) GetRemainingAttempts(ip string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.attempts[ip]
	if !exists {
		return rl.maxAttempts
	}

	if !entry.blockedAt.IsZero() {
		return 0
	}

	remaining := rl.maxAttempts - entry.count
	if remaining < 0 {
		return 0
	}

	return remaining
}
