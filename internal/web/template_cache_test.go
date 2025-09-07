package web

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTemplateCacheBasicOperations(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      100 * time.Millisecond,
		MaxSize:         10,
		CleanupInterval: 50 * time.Millisecond,
	})
	defer cache.Stop()

	// Test cache miss
	content, found := cache.Get("test-key")
	assert.False(t, found)
	assert.Nil(t, content)

	// Test cache set and hit
	testContent := []byte("test content")
	cache.Set("test-key", testContent, 0)

	content, found = cache.Get("test-key")
	assert.True(t, found)
	assert.Equal(t, testContent, content)

	// Test TTL expiration
	time.Sleep(150 * time.Millisecond)
	content, found = cache.Get("test-key")
	assert.False(t, found)
	assert.Nil(t, content)
}

func TestTemplateCacheStats(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      1 * time.Second,
		MaxSize:         5,
		CleanupInterval: 100 * time.Millisecond,
	})
	defer cache.Stop()

	// Add some entries
	cache.Set("key1", []byte("content1"), 0)
	cache.Set("key2", []byte("content2"), 0)
	cache.Set("key3", []byte("content3"), 0)

	stats := cache.Stats()
	assert.Equal(t, 3, stats.Entries)
	assert.Equal(t, 5, stats.MaxSize)
	assert.Greater(t, stats.TotalSize, int64(0))
}

func TestTemplateCacheEviction(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      1 * time.Second,
		MaxSize:         2, // Small size to trigger eviction
		CleanupInterval: 100 * time.Millisecond,
	})
	defer cache.Stop()

	// Fill cache to capacity
	cache.Set("key1", []byte("content1"), 0)
	cache.Set("key2", []byte("content2"), 0)

	stats := cache.Stats()
	assert.Equal(t, 2, stats.Entries)

	// Access key1 to make it more recently used
	_, found := cache.Get("key1")
	assert.True(t, found)

	// Add another entry, should evict key2 (oldest)
	cache.Set("key3", []byte("content3"), 0)

	stats = cache.Stats()
	assert.Equal(t, 2, stats.Entries)

	// key1 should still exist
	_, found = cache.Get("key1")
	assert.True(t, found)

	// key2 should be evicted
	_, found = cache.Get("key2")
	assert.False(t, found)

	// key3 should exist
	_, found = cache.Get("key3")
	assert.True(t, found)
}

func TestTemplateCacheClear(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Add entries
	cache.Set("key1", []byte("content1"), 0)
	cache.Set("key2", []byte("content2"), 0)

	stats := cache.Stats()
	assert.Equal(t, 2, stats.Entries)

	// Clear cache
	cache.Clear()

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Entries)

	// Verify entries are gone
	_, found := cache.Get("key1")
	assert.False(t, found)
	_, found = cache.Get("key2")
	assert.False(t, found)
}

func TestTemplateCacheInvalidation(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Add some entries
	cache.Set("key1", []byte("content1"), 0)
	cache.Set("key2", []byte("content2"), 0)
	cache.Set("test-pattern", []byte("content3"), 0)

	stats := cache.Stats()
	assert.Equal(t, 3, stats.Entries)

	// Test pattern invalidation
	count := cache.Invalidate("key1")
	assert.Equal(t, 1, count)

	// Verify key1 is gone
	_, found := cache.Get("key1")
	assert.False(t, found)

	// Other keys should remain
	_, found = cache.Get("key2")
	assert.True(t, found)

	// Test wildcard invalidation
	count = cache.Invalidate("*")
	assert.Equal(t, 2, count) // Should remove remaining 2 entries

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Entries)
}

func TestTemplateCacheCleanup(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      50 * time.Millisecond, // Short TTL
		MaxSize:         10,
		CleanupInterval: 25 * time.Millisecond, // Frequent cleanup
	})
	defer cache.Stop()

	// Add entries
	cache.Set("key1", []byte("content1"), 0)
	cache.Set("key2", []byte("content2"), 0)

	stats := cache.Stats()
	assert.Equal(t, 2, stats.Entries)

	// Wait for entries to expire and cleanup to run
	time.Sleep(100 * time.Millisecond)

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Entries, "Cleanup should have removed expired entries")
}

func TestDefaultTemplateCacheConfig(t *testing.T) {
	config := DefaultTemplateCacheConfig()

	assert.Equal(t, 30*time.Second, config.DefaultTTL)
	assert.Equal(t, 1000, config.MaxSize)
	assert.Equal(t, 60*time.Second, config.CleanupInterval)
}

func TestTemplateCacheCustomTTL(t *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Set with custom TTL
	customTTL := 200 * time.Millisecond
	cache.Set("custom-ttl-key", []byte("custom content"), customTTL)

	// Should be available immediately
	content, found := cache.Get("custom-ttl-key")
	assert.True(t, found)
	assert.Equal(t, []byte("custom content"), content)

	// Wait for custom TTL to expire
	time.Sleep(250 * time.Millisecond)

	// Should be expired
	content, found = cache.Get("custom-ttl-key")
	assert.False(t, found)
	assert.Nil(t, content)
}

// Benchmark cache performance
func BenchmarkTemplateCacheSet(b *testing.B) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	content := []byte(strings.Repeat("test content ", 100))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strings.Repeat("k", i%10+1) // Vary key length to avoid collision
		cache.Set(key, content, 0)
	}
}

func BenchmarkTemplateCacheGet(b *testing.B) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	// Pre-populate cache
	content := []byte(strings.Repeat("test content ", 100))
	for i := 0; i < 100; i++ {
		key := strings.Repeat("k", i%10+1)
		cache.Set(key, content, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strings.Repeat("k", i%10+1)
		cache.Get(key)
	}
}

func BenchmarkTemplateCacheSetGet(b *testing.B) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	content := []byte(strings.Repeat("test content ", 100))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strings.Repeat("k", i%10+1)
		if i%2 == 0 {
			cache.Set(key, content, 0)
		} else {
			cache.Get(key)
		}
	}
}
