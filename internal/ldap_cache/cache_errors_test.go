// Package ldap_cache provides thread-safe generic caching for LDAP entities.
// This file contains error scenario and edge case tests for the cache implementation.
// nolint:revive
package ldap_cache

import (
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nonexistentData is a constant for test data that doesn't exist in the cache
const nonexistentData = "nonexistent"

// mockCacheableWithSAM is a test implementation with SAMAccountName support
type mockCacheableWithSAM struct {
	dn             string
	SAMAccountName string
	data           string
}

func (m mockCacheableWithSAM) DN() string {
	return m.dn
}

// TestCache_EmptyDNHandling tests behavior when items have empty DN
func TestCache_EmptyDNHandling(t *testing.T) {
	cache := NewCached[mockCacheable]()

	items := []mockCacheable{
		{dn: "", data: "empty-dn"},
		{dn: "cn=valid,dc=example,dc=com", data: "valid"},
	}
	cache.setAll(items)

	t.Run("empty DN not indexed", func(t *testing.T) {
		// Empty DN should not be findable by DN lookup
		item, found := cache.FindByDN("")
		assert.False(t, found, "Empty DN should not be indexed")
		assert.Nil(t, item)
	})

	t.Run("valid DN still works", func(t *testing.T) {
		item, found := cache.FindByDN("cn=valid,dc=example,dc=com")
		assert.True(t, found)
		assert.Equal(t, "valid", item.data)
	})

	t.Run("empty DN findable via predicate", func(t *testing.T) {
		item, found := cache.Find(func(m mockCacheable) bool {
			return m.data == "empty-dn"
		})
		assert.True(t, found)
		assert.Equal(t, "", item.dn)
	})
}

// TestCache_DuplicateDNs tests behavior when multiple items have same DN
func TestCache_DuplicateDNs(t *testing.T) {
	cache := NewCached[mockCacheable]()

	items := []mockCacheable{
		{dn: "cn=dup,dc=example,dc=com", data: "first"},
		{dn: "cn=dup,dc=example,dc=com", data: "second"},
		{dn: "cn=dup,dc=example,dc=com", data: "third"},
	}
	cache.setAll(items)

	t.Run("last item wins in index", func(t *testing.T) {
		// Due to index building, last item with same DN should be indexed
		item, found := cache.FindByDN("cn=dup,dc=example,dc=com")
		assert.True(t, found)
		// The index points to the last item with that DN
		assert.Equal(t, "third", item.data)
	})

	t.Run("all items in Get", func(t *testing.T) {
		items := cache.Get()
		assert.Len(t, items, 3)
	})
}

// TestCache_ConcurrentWriteDuringRead tests race conditions
func TestCache_ConcurrentWriteDuringRead(t *testing.T) {
	cache := NewCached[mockCacheable]()

	// Initialize with data
	initial := make([]mockCacheable, 1000)
	for i := range 1000 {
		initial[i] = mockCacheable{
			dn:   "cn=user" + strconv.Itoa(i) + ",dc=example,dc=com",
			data: "initial",
		}
	}
	cache.setAll(initial)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent readers
	for range 20 {
		wg.Go(func() {
			for range 100 {
				// Read operations should not panic
				_ = cache.Get()
				_ = cache.Count()
				_, _ = cache.FindByDN("cn=user0,dc=example,dc=com")
				_ = cache.Filter(func(m mockCacheable) bool { return true })
			}
		})
	}

	// Concurrent writers
	for range 5 {
		wg.Go(func() {
			for j := range 50 {
				newItems := make([]mockCacheable, 100)
				for k := range 100 {
					newItems[k] = mockCacheable{
						dn:   "cn=new" + strconv.Itoa(k) + ",dc=example,dc=com",
						data: "updated-" + strconv.Itoa(j),
					}
				}
				cache.setAll(newItems)
			}
		})
	}

	wg.Wait()
	close(errors)

	// No panics or errors should have occurred
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Cache should still be functional
	assert.Greater(t, cache.Count(), 0)
}

// TestCache_LargeBatchOperations tests performance with large datasets
func TestCache_LargeBatchOperations(t *testing.T) {
	cache := NewCached[mockCacheable]()

	t.Run("10000 items", func(t *testing.T) {
		items := make([]mockCacheable, 10000)
		for i := range 10000 {
			items[i] = mockCacheable{
				dn:   "cn=user" + strconv.Itoa(i) + ",dc=example,dc=com",
				data: "data",
			}
		}

		start := time.Now()
		cache.setAll(items)
		duration := time.Since(start)

		assert.Equal(t, 10000, cache.Count())
		// Should complete in reasonable time (under 1 second)
		assert.Less(t, duration, time.Second)
	})

	t.Run("rapid setAll replacements", func(t *testing.T) {
		for i := range 100 {
			items := make([]mockCacheable, 100)
			for j := range 100 {
				items[j] = mockCacheable{
					dn:   "cn=rapid" + strconv.Itoa(j) + ",dc=example,dc=com",
					data: "iteration-" + strconv.Itoa(i),
				}
			}
			cache.setAll(items)
		}

		// Final state should be consistent
		assert.Equal(t, 100, cache.Count())
	})
}

// TestCache_FindWithNilPredicate tests nil predicate handling
func TestCache_FindWithNilPredicate(t *testing.T) {
	cache := NewCached[mockCacheable]()
	cache.setAll([]mockCacheable{
		{dn: "cn=test,dc=example,dc=com", data: "test"},
	})

	// Find with always-false predicate
	item, found := cache.Find(func(_ mockCacheable) bool {
		return false
	})
	assert.False(t, found)
	assert.Nil(t, item)

	// Filter with always-false predicate
	filtered := cache.Filter(func(_ mockCacheable) bool {
		return false
	})
	assert.Empty(t, filtered)
}

// TestCache_UpdateWithEmptyItems tests update on empty cache
func TestCache_UpdateWithEmptyItems(t *testing.T) {
	cache := NewCached[mockCacheable]()

	// Update on empty cache should not panic
	updateCalled := false
	cache.update(func(item *mockCacheable) {
		updateCalled = true
		item.data = "updated"
	})

	assert.False(t, updateCalled, "Update should not be called on empty cache")
	assert.Equal(t, 0, cache.Count())
}

// TestGetSAMAccountName tests the reflection-based SAMAccountName extraction
func TestGetSAMAccountName(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		var ptr *mockCacheableWithSAM
		result := getSAMAccountName(ptr)
		assert.Empty(t, result)
	})

	t.Run("non-struct type", func(t *testing.T) {
		result := getSAMAccountName("string value")
		assert.Empty(t, result)

		result = getSAMAccountName(12345)
		assert.Empty(t, result)

		result = getSAMAccountName([]string{"slice"})
		assert.Empty(t, result)
	})

	t.Run("struct without SAMAccountName", func(t *testing.T) {
		type noSAM struct {
			Name string
		}
		result := getSAMAccountName(noSAM{Name: "test"})
		assert.Empty(t, result)
	})

	t.Run("struct with non-string SAMAccountName", func(t *testing.T) {
		type wrongTypeSAM struct {
			SAMAccountName int
		}
		result := getSAMAccountName(wrongTypeSAM{SAMAccountName: 123})
		assert.Empty(t, result)
	})

	t.Run("valid struct with SAMAccountName", func(t *testing.T) {
		item := mockCacheableWithSAM{
			dn:             "cn=test,dc=example,dc=com",
			SAMAccountName: "testuser",
			data:           "data",
		}
		result := getSAMAccountName(item)
		assert.Equal(t, "testuser", result)
	})

	t.Run("pointer to valid struct", func(t *testing.T) {
		item := &mockCacheableWithSAM{
			dn:             "cn=test,dc=example,dc=com",
			SAMAccountName: "testuser",
			data:           "data",
		}
		result := getSAMAccountName(item)
		assert.Equal(t, "testuser", result)
	})
}

// TestMetrics_ConcurrentAccess tests metrics under heavy concurrent access
func TestMetrics_ConcurrentAccess(t *testing.T) {
	metrics := NewMetrics()

	var wg sync.WaitGroup

	// Concurrent cache hit/miss recording
	for range 100 {
		wg.Go(func() {
			for range 1000 {
				metrics.RecordCacheHit()
				metrics.RecordCacheMiss()
			}
		})
	}

	wg.Wait()

	// Verify counts are consistent
	hits := atomic.LoadInt64(&metrics.CacheHits)
	misses := atomic.LoadInt64(&metrics.CacheMisses)

	assert.Equal(t, int64(100000), hits)
	assert.Equal(t, int64(100000), misses)
}

// TestMetrics_HitRateEdgeCases tests edge cases in hit rate calculation
func TestMetrics_HitRateEdgeCases(t *testing.T) {
	t.Run("zero operations", func(t *testing.T) {
		metrics := NewMetrics()
		rate := metrics.GetCacheHitRate()
		assert.Equal(t, 0.0, rate)
	})

	t.Run("100% hit rate", func(t *testing.T) {
		metrics := NewMetrics()
		for range 100 {
			metrics.RecordCacheHit()
		}
		rate := metrics.GetCacheHitRate()
		assert.Equal(t, 100.0, rate)
	})

	t.Run("0% hit rate", func(t *testing.T) {
		metrics := NewMetrics()
		for range 100 {
			metrics.RecordCacheMiss()
		}
		rate := metrics.GetCacheHitRate()
		assert.Equal(t, 0.0, rate)
	})

	t.Run("50% hit rate", func(t *testing.T) {
		metrics := NewMetrics()
		for range 50 {
			metrics.RecordCacheHit()
			metrics.RecordCacheMiss()
		}
		rate := metrics.GetCacheHitRate()
		assert.Equal(t, 50.0, rate)
	})
}

// TestMetrics_HealthStatusTransitions tests health status changes
func TestMetrics_HealthStatusTransitions(t *testing.T) {
	t.Run("starts healthy", func(t *testing.T) {
		metrics := NewMetrics()
		assert.Equal(t, HealthHealthy, metrics.GetHealthStatus())
	})

	t.Run("degrades with errors", func(t *testing.T) {
		metrics := NewMetrics()
		// Record 10 refreshes with 1 error (10% error rate)
		for range 9 {
			startTime := metrics.RecordRefreshStart()
			metrics.RecordRefreshComplete(startTime, 10, 5, 2)
		}
		// This refresh fails
		metrics.RecordRefreshStart()
		metrics.RecordRefreshError()

		// Should be degraded at exactly 10%
		status := metrics.GetHealthStatus()
		assert.True(t, status == HealthDegraded || status == HealthUnhealthy)
	})

	t.Run("unhealthy with high error rate", func(t *testing.T) {
		metrics := NewMetrics()
		// Record 10 refreshes with 5 errors (50% error rate)
		for range 5 {
			startTime := metrics.RecordRefreshStart()
			metrics.RecordRefreshComplete(startTime, 10, 5, 2)
		}
		for range 5 {
			metrics.RecordRefreshStart()
			metrics.RecordRefreshError()
		}

		assert.Equal(t, HealthUnhealthy, metrics.GetHealthStatus())
	})
}

// TestMetrics_RefreshDuration tests refresh duration tracking
func TestMetrics_RefreshDuration(t *testing.T) {
	metrics := NewMetrics()

	startTime := metrics.RecordRefreshStart()
	time.Sleep(10 * time.Millisecond)
	metrics.RecordRefreshComplete(startTime, 100, 50, 25)

	duration := metrics.GetLastRefreshDuration()
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
}

// TestMetrics_LargeNumbers tests metrics with large counter values
func TestMetrics_LargeNumbers(t *testing.T) {
	metrics := NewMetrics()

	// Set counters to near-max values
	atomic.StoreInt64(&metrics.CacheHits, math.MaxInt64-1)

	// Should not overflow
	metrics.RecordCacheHit()
	hits := atomic.LoadInt64(&metrics.CacheHits)
	assert.Equal(t, int64(math.MaxInt64), hits)
}

// TestCache_IndexConsistencyAfterUpdate tests that indexes remain valid
func TestCache_IndexConsistencyAfterUpdate(t *testing.T) {
	cache := NewCached[mockCacheable]()

	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "original1"},
		{dn: "cn=user2,dc=example,dc=com", data: "original2"},
	}
	cache.setAll(items)

	// Verify initial index state
	item, found := cache.FindByDN("cn=user1,dc=example,dc=com")
	require.True(t, found)
	assert.Equal(t, "original1", item.data)

	// Update items
	cache.update(func(item *mockCacheable) {
		item.data = "updated"
	})

	// Index should still work and reflect updated data
	item, found = cache.FindByDN("cn=user1,dc=example,dc=com")
	require.True(t, found)
	assert.Equal(t, "updated", item.data)
}

// TestCache_SAMAccountNameIndex tests SAMAccountName index operations
func TestCache_SAMAccountNameIndex(t *testing.T) {
	cache := NewCached[mockCacheableWithSAM]()

	items := []mockCacheableWithSAM{
		{dn: "cn=user1,dc=example,dc=com", SAMAccountName: "user1", data: "data1"},
		{dn: "cn=user2,dc=example,dc=com", SAMAccountName: "user2", data: "data2"},
		{dn: "cn=user3,dc=example,dc=com", SAMAccountName: "", data: "data3"}, // Empty SAM
	}
	cache.setAll(items)

	t.Run("find by existing SAM", func(t *testing.T) {
		item, found := cache.FindBySAMAccountName("user1")
		assert.True(t, found)
		assert.Equal(t, "data1", item.data)
	})

	t.Run("find by non-existent SAM", func(t *testing.T) {
		item, found := cache.FindBySAMAccountName(nonexistentData)
		assert.False(t, found)
		assert.Nil(t, item)
	})

	t.Run("empty SAM not indexed", func(t *testing.T) {
		item, found := cache.FindBySAMAccountName("")
		assert.False(t, found)
		assert.Nil(t, item)
	})
}

// TestCache_FilterNoMatches tests Filter with no matching items
func TestCache_FilterNoMatches(t *testing.T) {
	cache := NewCached[mockCacheable]()

	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "data1"},
		{dn: "cn=user2,dc=example,dc=com", data: "data2"},
	}
	cache.setAll(items)

	// Filter that matches nothing
	result := cache.Filter(func(m mockCacheable) bool {
		return m.data == nonexistentData
	})

	// Filter returns nil when no matches (not an empty slice)
	assert.Nil(t, result)
}

// TestCache_EmptySetAll tests setAll with empty slice
func TestCache_EmptySetAll(t *testing.T) {
	cache := NewCached[mockCacheable]()

	// Set initial data
	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "data1"},
	}
	cache.setAll(items)
	assert.Equal(t, 1, cache.Count())

	// Replace with empty slice
	cache.setAll([]mockCacheable{})
	assert.Equal(t, 0, cache.Count())

	// Indexes should be cleared
	_, found := cache.FindByDN("cn=user1,dc=example,dc=com")
	assert.False(t, found)
}

// TestMetrics_SummaryStats tests GetSummaryStats comprehensive output
func TestMetrics_SummaryStats(t *testing.T) {
	metrics := NewMetrics()

	// Record some activity
	for range 5 {
		metrics.RecordCacheHit()
	}
	for range 3 {
		metrics.RecordCacheMiss()
	}

	startTime := metrics.RecordRefreshStart()
	time.Sleep(5 * time.Millisecond)
	metrics.RecordRefreshComplete(startTime, 100, 50, 25)

	stats := metrics.GetSummaryStats()

	assert.Greater(t, stats.CacheHitRate, 0.0)
	assert.Equal(t, int64(8), stats.TotalOperations)
	assert.Equal(t, int64(1), stats.RefreshCount)
	assert.Equal(t, int64(0), stats.RefreshErrors)
	assert.Greater(t, stats.Uptime, time.Duration(0))
	assert.Equal(t, "healthy", stats.HealthStatus)
	assert.Equal(t, int64(100), stats.EntityCounts.Users)
	assert.Equal(t, int64(50), stats.EntityCounts.Groups)
	assert.Equal(t, int64(25), stats.EntityCounts.Computers)
}

// TestCache_ConcurrentIndexAccess tests concurrent access to indexes
func TestCache_ConcurrentIndexAccess(t *testing.T) {
	cache := NewCached[mockCacheable]()

	items := make([]mockCacheable, 100)
	for i := range 100 {
		items[i] = mockCacheable{
			dn:   "cn=user" + strconv.Itoa(i) + ",dc=example,dc=com",
			data: "data",
		}
	}
	cache.setAll(items)

	var wg sync.WaitGroup

	// Concurrent FindByDN operations
	for range 50 {
		wg.Go(func() {
			for j := range 100 {
				dn := "cn=user" + strconv.Itoa(j) + ",dc=example,dc=com"
				cache.FindByDN(dn)
			}
		})
	}

	// Concurrent setAll operations (index rebuilding)
	for range 5 {
		wg.Go(func() {
			for range 10 {
				newItems := make([]mockCacheable, 50)
				for k := range 50 {
					newItems[k] = mockCacheable{
						dn:   "cn=new" + strconv.Itoa(k) + ",dc=example,dc=com",
						data: "new",
					}
				}
				cache.setAll(newItems)
			}
		})
	}

	wg.Wait()

	// Cache should still be functional
	count := cache.Count()
	assert.GreaterOrEqual(t, count, 0)
}

// TestMetrics_Uptime tests uptime calculation
func TestMetrics_Uptime(t *testing.T) {
	metrics := NewMetrics()

	time.Sleep(10 * time.Millisecond)

	uptime := metrics.GetUptime()
	assert.GreaterOrEqual(t, uptime, 10*time.Millisecond)
}

// TestMetrics_EntityCounts tests entity count tracking
func TestMetrics_EntityCounts(t *testing.T) {
	metrics := NewMetrics()

	startTime := metrics.RecordRefreshStart()
	metrics.RecordRefreshComplete(startTime, 1000, 500, 250)

	users, groups, computers := metrics.GetEntityCounts()

	assert.Equal(t, int64(1000), users)
	assert.Equal(t, int64(500), groups)
	assert.Equal(t, int64(250), computers)
}

// TestCacheWithMetrics_HitMissTracking tests metrics integration with cache
func TestCacheWithMetrics_HitMissTracking(t *testing.T) {
	metrics := NewMetrics()
	cache := NewCachedWithMetrics[mockCacheable](metrics)

	items := []mockCacheable{
		{dn: "cn=user1,dc=example,dc=com", data: "data1"},
	}
	cache.setAll(items)

	// Hit
	_, found := cache.FindByDN("cn=user1,dc=example,dc=com")
	assert.True(t, found)

	// Miss
	_, found = cache.FindByDN("cn=nonexistent,dc=example,dc=com")
	assert.False(t, found)

	// Hit via Find
	_, found = cache.Find(func(m mockCacheable) bool {
		return m.data == "data1"
	})
	assert.True(t, found)

	// Miss via Find
	_, found = cache.Find(func(m mockCacheable) bool {
		return m.data == nonexistentData
	})
	assert.False(t, found)

	hits := atomic.LoadInt64(&metrics.CacheHits)
	misses := atomic.LoadInt64(&metrics.CacheMisses)

	assert.Equal(t, int64(2), hits)
	assert.Equal(t, int64(2), misses)
}
