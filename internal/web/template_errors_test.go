package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateCacheConcurrentAccess tests thread safety under heavy concurrent access
func TestTemplateCacheConcurrentAccess(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      1 * time.Second,
		MaxSize:         100,
		CleanupInterval: 100 * time.Millisecond,
	})
	defer cache.Stop()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent writers
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 50 {
				key := "key" + strconv.Itoa(id) + "_" + strconv.Itoa(j)
				content := []byte("content-" + key)
				cache.Set(key, content, 0)
			}
		}(i)
	}

	// Concurrent readers
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 50 {
				key := "key" + strconv.Itoa(id) + "_" + strconv.Itoa(j)
				_, _ = cache.Get(key)
			}
		}(i)
	}

	// Concurrent invalidators
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				cache.Invalidate("*")
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}

	// Cache should still be functional
	cache.Set("final-key", []byte("final-content"), 0)
	content, found := cache.Get("final-key")
	assert.True(t, found)
	assert.Equal(t, []byte("final-content"), content)
}

// TestTemplateCacheMemoryPressure tests behavior under memory pressure
func TestTemplateCacheMemoryPressure(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      1 * time.Second,
		MaxSize:         5, // Very small cache
		CleanupInterval: 100 * time.Millisecond,
	})
	defer cache.Stop()

	// Add more entries than max size
	for i := range 20 {
		key := "key" + strconv.Itoa(i)
		content := []byte(strings.Repeat("x", 1000)) // 1KB content
		cache.Set(key, content, 0)
	}

	stats := cache.Stats()
	assert.LessOrEqual(t, stats.Entries, 5, "Cache should not exceed max size")
}

// TestTemplateCacheZeroDefaultTTL tests behavior with zero default TTL configuration.
// When both DefaultTTL and the explicit TTL parameter are 0, entries expire immediately.
func TestTemplateCacheZeroDefaultTTL(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      0, // Zero default TTL means entries expire immediately
		MaxSize:         10,
		CleanupInterval: 10 * time.Millisecond,
	})
	defer cache.Stop()

	// Set with 0 TTL uses the default (which is 0), so entry expires immediately
	cache.Set("zero-ttl", []byte("content"), 0)

	// Entry might expire immediately with 0 TTL
	time.Sleep(15 * time.Millisecond)

	// Get should not return expired entry
	_, found := cache.Get("zero-ttl")
	// With 0 TTL, the entry expires immediately
	assert.False(t, found, "Entry with 0 TTL should expire immediately")
}

// TestTemplateCacheVeryLargeTTL tests behavior with very large TTL
func TestTemplateCacheVeryLargeTTL(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      365 * 24 * time.Hour, // 1 year
		MaxSize:         10,
		CleanupInterval: 100 * time.Millisecond,
	})
	defer cache.Stop()

	cache.Set("long-lived", []byte("content"), 0)

	// Should still be available immediately
	content, found := cache.Get("long-lived")
	assert.True(t, found)
	assert.Equal(t, []byte("content"), content)
}

// TestTemplateCacheEmptyContent tests caching empty content
func TestTemplateCacheEmptyContent(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Set empty content
	cache.Set("empty", []byte{}, 0)

	content, found := cache.Get("empty")
	assert.True(t, found)
	assert.Empty(t, content)
}

// TestTemplateCacheNilContent tests handling of nil content
func TestTemplateCacheNilContent(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Set nil content
	cache.Set("nil", nil, 0)

	content, found := cache.Get("nil")
	assert.True(t, found)
	assert.Nil(t, content)
}

// TestTemplateCacheEmptyKey tests handling of empty key
func TestTemplateCacheEmptyKey(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Set with empty key
	cache.Set("", []byte("content"), 0)

	content, found := cache.Get("")
	assert.True(t, found)
	assert.Equal(t, []byte("content"), content)
}

// TestTemplateCacheStatsAccuracy tests that stats are accurate
func TestTemplateCacheStatsAccuracy(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      100 * time.Millisecond,
		MaxSize:         100,
		CleanupInterval: 50 * time.Millisecond,
	})
	defer cache.Stop()

	// Add entries
	for i := range 10 {
		key := "key" + strconv.Itoa(i)
		cache.Set(key, []byte("content"), 0)
	}

	stats := cache.Stats()
	assert.Equal(t, 10, stats.Entries)

	// Wait for expiration and cleanup
	time.Sleep(150 * time.Millisecond)

	// Stats might still show entries but they're expired
	// After cleanup, entries should be 0
	time.Sleep(60 * time.Millisecond) // Wait for cleanup
	stats = cache.Stats()
	assert.Equal(t, 0, stats.Entries)
}

// TestTemplateCacheInvalidatePatterns tests various invalidation patterns
func TestTemplateCacheInvalidatePatterns(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Add entries
	cache.Set("user-1", []byte("content1"), 0)
	cache.Set("user-2", []byte("content2"), 0)
	cache.Set("group-1", []byte("content3"), 0)
	cache.Set("group-2", []byte("content4"), 0)

	stats := cache.Stats()
	assert.Equal(t, 4, stats.Entries)

	// Invalidate specific entry
	count := cache.Invalidate("user-1")
	assert.Equal(t, 1, count)

	stats = cache.Stats()
	assert.Equal(t, 3, stats.Entries)

	// Invalidate non-existent entry
	count = cache.Invalidate("nonexistent")
	assert.Equal(t, 0, count)

	// Invalidate all
	count = cache.Invalidate("*")
	assert.Equal(t, 3, count)

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Entries)
}

// TestTemplateCacheInvalidateByPath tests path-based invalidation
func TestTemplateCacheInvalidateByPath(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	cache.Set("key1", []byte("content1"), 0)
	cache.Set("key2", []byte("content2"), 0)

	stats := cache.Stats()
	assert.Equal(t, 2, stats.Entries)

	// Invalidate by path
	count := cache.InvalidateByPath("/users")
	assert.Equal(t, 2, count) // Currently invalidates all for non-empty path

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Entries)
}

// TestTemplateCacheStopSingleCallSafe tests that Stop can be called once safely
func TestTemplateCacheStopSingleCallSafe(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())

	// Stop should not panic on the first (and only) call
	require.NotPanics(t, func() {
		cache.Stop()
	}, "Stop should not panic on first call")
}

// TestTemplateCacheGenerateKey tests cache key generation
func TestTemplateCacheGenerateKey(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	app := fiber.New()

	t.Run("same request generates same key", func(t *testing.T) {
		var key1, key2 string

		app.Get("/test", func(c *fiber.Ctx) error {
			key1 = cache.generateCacheKey(c)
			key2 = cache.generateCacheKey(c)

			return c.SendString("ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, key1, key2, "Same request should generate same cache key")
	})

	t.Run("different paths generate different keys", func(t *testing.T) {
		var key1, key2 string

		app.Get("/path1", func(c *fiber.Ctx) error {
			key1 = cache.generateCacheKey(c)

			return c.SendString("ok")
		})
		app.Get("/path2", func(c *fiber.Ctx) error {
			key2 = cache.generateCacheKey(c)

			return c.SendString("ok")
		})

		req1 := httptest.NewRequest(http.MethodGet, "/path1", http.NoBody)
		resp1, err := app.Test(req1)
		require.NoError(t, err)
		_ = resp1.Body.Close()

		req2 := httptest.NewRequest(http.MethodGet, "/path2", http.NoBody)
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		_ = resp2.Body.Close()

		assert.NotEqual(t, key1, key2, "Different paths should generate different cache keys")
	})

	t.Run("additional data changes key", func(t *testing.T) {
		var key1, key2 string

		app.Get("/additional", func(c *fiber.Ctx) error {
			key1 = cache.generateCacheKey(c, "data1")
			key2 = cache.generateCacheKey(c, "data2")

			return c.SendString("ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/additional", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.NotEqual(t, key1, key2, "Different additional data should generate different keys")
	})
}

// TestTemplateCacheMiddleware tests the cache middleware
func TestTemplateCacheMiddleware(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	app := fiber.New()

	// Add cache middleware for specific paths
	app.Use(cache.CacheMiddleware("/cached"))

	callCount := 0
	app.Get("/cached", func(c *fiber.Ctx) error {
		callCount++

		return c.SendString("content")
	})

	app.Get("/not-cached", func(c *fiber.Ctx) error {
		callCount++

		return c.SendString("content")
	})

	t.Run("non-GET requests are not cached", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cached", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()

		// POST should not be cached
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("non-matching paths are not cached", func(t *testing.T) {
		callCount = 0

		req := httptest.NewRequest(http.MethodGet, "/not-cached", http.NoBody)
		resp, _ := app.Test(req)
		_ = resp.Body.Close()

		assert.Equal(t, 1, callCount)

		// Second request should still call handler
		req = httptest.NewRequest(http.MethodGet, "/not-cached", http.NoBody)
		resp, _ = app.Test(req)
		_ = resp.Body.Close()

		assert.Equal(t, 2, callCount)
	})
}

// TestTemplateCacheEvictionOrder tests that oldest entries are evicted first
func TestTemplateCacheEvictionOrder(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      1 * time.Second,
		MaxSize:         3,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	// Add entries with time gaps to establish access order
	cache.Set("first", []byte("1"), 0)
	time.Sleep(10 * time.Millisecond)
	cache.Set("second", []byte("2"), 0)
	time.Sleep(10 * time.Millisecond)
	cache.Set("third", []byte("3"), 0)

	// Access "first" to make it more recently used
	cache.Get("first")
	time.Sleep(10 * time.Millisecond)

	// Add fourth entry, should evict "second" (oldest accessed)
	cache.Set("fourth", []byte("4"), 0)

	// Check what's in cache
	_, foundFirst := cache.Get("first")
	_, foundSecond := cache.Get("second")
	_, foundThird := cache.Get("third")
	_, foundFourth := cache.Get("fourth")

	assert.True(t, foundFirst, "first should still be in cache (recently accessed)")
	assert.False(t, foundSecond, "second should be evicted (oldest accessed)")
	assert.True(t, foundThird, "third should still be in cache")
	assert.True(t, foundFourth, "fourth should be in cache (just added)")
}

// TestTemplateCacheLargeContent tests caching large content
func TestTemplateCacheLargeContent(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Create large content (1MB)
	largeContent := []byte(strings.Repeat("x", 1024*1024))

	cache.Set("large", largeContent, 0)

	content, found := cache.Get("large")
	assert.True(t, found)
	assert.Len(t, content, 1024*1024)

	stats := cache.Stats()
	assert.GreaterOrEqual(t, stats.TotalSize, int64(1024*1024))
}

// TestTemplateCacheSpecialCharacterKeys tests handling of special character keys
func TestTemplateCacheSpecialCharacterKeys(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	specialKeys := []string{
		"key with spaces",
		"key/with/slashes",
		"key?with=query",
		"key#with#hash",
		"key\twith\ttabs",
		"key\nwith\nnewlines",
		"key日本語",
		"key🎉emoji",
	}

	for _, key := range specialKeys {
		t.Run(key, func(t *testing.T) {
			cache.Set(key, []byte("content"), 0)
			content, found := cache.Get(key)
			assert.True(t, found, "Key '%s' should be found", key)
			assert.Equal(t, []byte("content"), content)
		})
	}
}
