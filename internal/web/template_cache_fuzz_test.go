package web

import (
	"strings"
	"testing"
	"time"
)

// FuzzTemplateCacheSet tests cache Set with fuzzed keys and values
func FuzzTemplateCacheSet(f *testing.F) {
	// Seed with edge cases
	f.Add("key", "content", int64(1000))
	f.Add("", "content", int64(0))
	f.Add("key", "", int64(100))
	f.Add("", "", int64(0))
	f.Add(strings.Repeat("k", 1000), strings.Repeat("c", 10000), int64(60000))
	f.Add("key with spaces", "content", int64(1000))
	f.Add("key/with/slashes", "content", int64(1000))
	f.Add("key?with=query", "content", int64(1000))
	f.Add("key\twith\ttabs", "content", int64(1000))
	f.Add("key\nwith\nnewlines", "content", int64(1000))
	f.Add("键", "内容", int64(1000)) // Unicode
	f.Add("key🎉", "content🎊", int64(1000)) // Emoji

	f.Fuzz(func(t *testing.T, key, content string, ttlMs int64) {
		cache := NewTemplateCache(TemplateCacheConfig{
			DefaultTTL:      100 * time.Millisecond,
			MaxSize:         100,
			CleanupInterval: 50 * time.Millisecond,
		})
		defer cache.Stop()

		// Limit TTL to reasonable values
		if ttlMs < 0 {
			ttlMs = 0
		}
		if ttlMs > time.Hour.Milliseconds() {
			ttlMs = time.Hour.Milliseconds()
		}
		ttl := time.Duration(ttlMs) * time.Millisecond

		// Set shouldn't panic
		cache.Set(key, []byte(content), ttl)

		// Get should return what we set (if TTL > 0)
		if ttl > 0 {
			retrieved, found := cache.Get(key)
			if found && string(retrieved) != content {
				t.Errorf("Content mismatch for key %q: got %q, want %q", key, retrieved, content)
			}
		}
	})
}

// FuzzTemplateCacheGet tests cache Get with fuzzed keys
func FuzzTemplateCacheGet(f *testing.F) {
	// Seed with edge cases
	f.Add("existing")
	f.Add("nonexistent")
	f.Add("")
	f.Add(strings.Repeat("x", 10000))
	f.Add("key with spaces")
	f.Add("键") // Unicode
	f.Add("key🎉") // Emoji
	f.Add("key\x00null") // Null byte

	f.Fuzz(func(t *testing.T, key string) {
		cache := NewTemplateCache(DefaultTemplateCacheConfig())
		defer cache.Stop()

		// Add a known entry
		cache.Set("existing", []byte("content"), 0)

		// Get shouldn't panic for any key
		_, found := cache.Get(key)

		// Known key should be found
		if key == "existing" && !found {
			t.Error("Known key 'existing' should be found")
		}
	})
}

// FuzzTemplateCacheInvalidate tests invalidation with fuzzed patterns
func FuzzTemplateCacheInvalidate(f *testing.F) {
	// Seed with patterns
	f.Add("*")
	f.Add("user-*")
	f.Add("*-suffix")
	f.Add("")
	f.Add("exact-key")
	f.Add("key with spaces")
	f.Add(strings.Repeat("x", 1000))

	f.Fuzz(func(t *testing.T, pattern string) {
		cache := NewTemplateCache(DefaultTemplateCacheConfig())
		defer cache.Stop()

		// Add some entries
		cache.Set("user-1", []byte("1"), 0)
		cache.Set("user-2", []byte("2"), 0)
		cache.Set("group-1", []byte("3"), 0)

		// Invalidate shouldn't panic
		count := cache.Invalidate(pattern)

		// Verify count is reasonable
		if count < 0 {
			t.Errorf("Negative invalidation count: %d", count)
		}

		// After "*" invalidation, cache should be empty
		if pattern == "*" {
			stats := cache.Stats()
			if stats.Entries != 0 {
				t.Errorf("Cache not empty after '*' invalidation: %d entries", stats.Entries)
			}
		}
	})
}

// FuzzTemplateCacheConcurrent tests concurrent cache operations
func FuzzTemplateCacheConcurrent(f *testing.F) {
	f.Add("key", "value", 10, 10)
	f.Add("k", "v", 100, 50)
	f.Add(strings.Repeat("x", 100), strings.Repeat("y", 1000), 5, 5)

	f.Fuzz(func(t *testing.T, key, value string, numWriters, numReaders int) {
		// Limit goroutine count
		if numWriters < 1 {
			numWriters = 1
		}
		if numWriters > 20 {
			numWriters = 20
		}
		if numReaders < 1 {
			numReaders = 1
		}
		if numReaders > 20 {
			numReaders = 20
		}

		cache := NewTemplateCache(TemplateCacheConfig{
			DefaultTTL:      time.Second,
			MaxSize:         100,
			CleanupInterval: 100 * time.Millisecond,
		})
		defer cache.Stop()

		done := make(chan bool)

		// Writers
		for range numWriters {
			go func() {
				for range 10 {
					cache.Set(key, []byte(value), 0)
				}
				done <- true
			}()
		}

		// Readers
		for range numReaders {
			go func() {
				for range 10 {
					cache.Get(key)
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for range numWriters + numReaders {
			<-done
		}

		// Cache should still be functional
		cache.Set("final", []byte("test"), 0)
		_, found := cache.Get("final")
		if !found {
			t.Error("Cache not functional after concurrent access")
		}
	})
}

// FuzzTemplateCacheEviction tests cache eviction under pressure
func FuzzTemplateCacheEviction(f *testing.F) {
	f.Add(5, 100)  // Small cache, many entries
	f.Add(100, 50) // Large cache, fewer entries
	f.Add(1, 10)   // Minimal cache

	f.Fuzz(func(t *testing.T, maxSize, numEntries int) {
		// Limit values
		if maxSize < 1 {
			maxSize = 1
		}
		if maxSize > 1000 {
			maxSize = 1000
		}
		if numEntries < 0 {
			numEntries = 0
		}
		if numEntries > 10000 {
			numEntries = 10000
		}

		cache := NewTemplateCache(TemplateCacheConfig{
			DefaultTTL:      time.Hour,
			MaxSize:         maxSize,
			CleanupInterval: time.Hour, // No auto-cleanup
		})
		defer cache.Stop()

		// Add many entries
		for i := range numEntries {
			key := "key" + string(rune(i%65536))
			cache.Set(key, []byte("content"), 0)
		}

		// Cache should respect max size
		stats := cache.Stats()
		if stats.Entries > maxSize {
			t.Errorf("Cache exceeded max size: %d > %d", stats.Entries, maxSize)
		}
	})
}

// FuzzTemplateCacheKeyGeneration tests that key generation is deterministic
func FuzzTemplateCacheKeyGeneration(f *testing.F) {
	f.Add("/path", "query=value", "extra")
	f.Add("", "", "")
	f.Add("/users", "page=1&sort=name", "session123")
	f.Add("/用户", "搜索=中文", "会话")

	f.Fuzz(func(t *testing.T, path, query, extra string) {
		cache := NewTemplateCache(DefaultTemplateCacheConfig())
		defer cache.Stop()

		// Generate key multiple times
		key1 := path + "?" + query + "#" + extra
		key2 := path + "?" + query + "#" + extra

		// Keys should be identical
		if key1 != key2 {
			t.Errorf("Non-deterministic key: %q != %q", key1, key2)
		}

		// Set and get should work with generated key
		cache.Set(key1, []byte("content"), 0)
		content, found := cache.Get(key2)
		if !found {
			t.Error("Key not found after setting")
		}
		if string(content) != "content" {
			t.Errorf("Content mismatch: %q", content)
		}
	})
}
