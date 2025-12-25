package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// TemplateCache provides thread-safe template result caching with TTL and automatic cleanup
type TemplateCache struct {
	entries         map[string]*cacheEntry
	mu              sync.RWMutex
	defaultTTL      time.Duration
	maxSize         int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

type cacheEntry struct {
	content    []byte
	createdAt  time.Time
	accessedAt time.Time
	ttl        time.Duration
}

// TemplateCacheConfig holds configuration for template caching
type TemplateCacheConfig struct {
	DefaultTTL      time.Duration
	MaxSize         int
	CleanupInterval time.Duration
}

// DefaultTemplateCacheConfig returns sensible defaults for template caching
func DefaultTemplateCacheConfig() TemplateCacheConfig {
	return TemplateCacheConfig{
		DefaultTTL:      30 * time.Second,
		MaxSize:         1000, // Maximum number of cached entries
		CleanupInterval: 60 * time.Second,
	}
}

// NewTemplateCache creates a new template cache with the given configuration
func NewTemplateCache(config TemplateCacheConfig) *TemplateCache {
	tc := &TemplateCache{
		entries:         make(map[string]*cacheEntry),
		defaultTTL:      config.DefaultTTL,
		maxSize:         config.MaxSize,
		cleanupInterval: config.CleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go tc.startCleanup()

	return tc
}

// generateCacheKey creates a cache key based on request path, query parameters, and user context
func (tc *TemplateCache) generateCacheKey(c *fiber.Ctx, additionalData ...string) string {
	h := sha256.New()

	// Include request path
	h.Write([]byte(c.Path()))

	// Include query parameters (sorted for consistency)
	for key, value := range c.Request().URI().QueryArgs().All() {
		h.Write([]byte(string(key) + "=" + string(value)))
	}

	// Include user session context if available
	if userDN, err := RequireUserDN(c); err == nil {
		h.Write([]byte("user:"))
		h.Write([]byte(userDN))
	}

	// Include any additional data for cache differentiation
	for _, data := range additionalData {
		h.Write([]byte(data))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves cached template content if available and not expired
func (tc *TemplateCache) Get(key string) ([]byte, bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	entry, exists := tc.entries[key]
	if !exists {
		return nil, false
	}

	// Check if entry is expired
	if time.Since(entry.createdAt) > entry.ttl {
		// Entry expired, but don't remove it here to avoid complex logic
		return nil, false
	}

	// Update access time for LRU tracking
	entry.accessedAt = time.Now()

	return entry.content, true
}

// Set stores template content in cache with the specified TTL
func (tc *TemplateCache) Set(key string, content []byte, ttl time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// If cache is full, remove oldest entries
	if len(tc.entries) >= tc.maxSize {
		tc.evictOldestUnsafe()
	}

	if ttl == 0 {
		ttl = tc.defaultTTL
	}

	now := time.Now()
	tc.entries[key] = &cacheEntry{
		content:    content,
		createdAt:  now,
		accessedAt: now,
		ttl:        ttl,
	}
}

// Invalidate removes cached entries matching the given pattern
func (tc *TemplateCache) Invalidate(pattern string) int {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	count := 0
	for key := range tc.entries {
		// Simple pattern matching - could be enhanced with regex if needed
		if pattern == "*" || key == pattern {
			delete(tc.entries, key)
			count++
		}
	}

	return count
}

// InvalidateByPath removes all cached entries for a specific path
func (tc *TemplateCache) InvalidateByPath(path string) int {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	count := 0
	for key := range tc.entries {
		// Check if the key contains the path (since keys are hashed,
		// we'll need to maintain a reverse mapping or use a different approach)
		// For now, we'll invalidate all entries (this could be optimized)
		if path != "" {
			delete(tc.entries, key)
			count++
		}
	}

	return count
}

// Clear removes all cached entries
func (tc *TemplateCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.entries = make(map[string]*cacheEntry)
}

// Stats returns cache statistics
func (tc *TemplateCache) Stats() CacheStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	var totalSize int64
	expired := 0
	now := time.Now()

	for _, entry := range tc.entries {
		totalSize += int64(len(entry.content))
		if now.Sub(entry.createdAt) > entry.ttl {
			expired++
		}
	}

	return CacheStats{
		Entries:        len(tc.entries),
		ExpiredEntries: expired,
		TotalSize:      totalSize,
		MaxSize:        tc.maxSize,
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Entries        int
	ExpiredEntries int
	TotalSize      int64
	MaxSize        int
}

// evictOldestUnsafe removes the oldest cache entry (must be called with write lock)
func (tc *TemplateCache) evictOldestUnsafe() {
	if len(tc.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time

	for key, entry := range tc.entries {
		if oldestKey == "" || entry.accessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.accessedAt
		}
	}

	if oldestKey != "" {
		delete(tc.entries, oldestKey)
	}
}

// cleanup removes expired entries
func (tc *TemplateCache) cleanup() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()
	for key, entry := range tc.entries {
		if now.Sub(entry.createdAt) > entry.ttl {
			delete(tc.entries, key)
		}
	}
}

// startCleanup runs periodic cleanup in a background goroutine
func (tc *TemplateCache) startCleanup() {
	ticker := time.NewTicker(tc.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.cleanup()
		case <-tc.stopCleanup:
			return
		}
	}
}

// Stop gracefully shuts down the cache cleanup goroutine
func (tc *TemplateCache) Stop() {
	close(tc.stopCleanup)
}

// RenderWithCache renders a template component with caching support
func (tc *TemplateCache) RenderWithCache(c *fiber.Ctx, component templ.Component, additionalCacheData ...string) error {
	// Generate cache key
	cacheKey := tc.generateCacheKey(c, additionalCacheData...)

	// Try to get from cache first
	if cachedContent, found := tc.Get(cacheKey); found {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

		return c.Send(cachedContent)
	}

	// Not in cache, render the template
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		return err
	}

	content := buf.Bytes()

	// Store in cache
	tc.Set(cacheKey, content, 0) // Use default TTL

	// Send to client
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return c.Send(content)
}

// CacheMiddleware creates a Fiber middleware for template caching
func (tc *TemplateCache) CacheMiddleware(paths ...string) fiber.Handler {
	pathMap := make(map[string]bool)
	for _, path := range paths {
		pathMap[path] = true
	}

	return func(c *fiber.Ctx) error {
		// Only cache GET requests for specified paths
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		// Check if this path should be cached
		if len(pathMap) > 0 && !pathMap[c.Path()] {
			return c.Next()
		}

		// Generate cache key
		cacheKey := tc.generateCacheKey(c)

		// Try to serve from cache
		if cachedContent, found := tc.Get(cacheKey); found {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

			return c.Send(cachedContent)
		}

		// Not in cache, continue to handler
		return c.Next()
	}
}

// LogStats logs cache statistics
func (tc *TemplateCache) LogStats() {
	stats := tc.Stats()
	log.Debug().
		Int("entries", stats.Entries).
		Int("expired_entries", stats.ExpiredEntries).
		Int64("total_size_bytes", stats.TotalSize).
		Int("max_size", stats.MaxSize).
		Msg("Template cache statistics")
}
