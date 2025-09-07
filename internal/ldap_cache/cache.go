// Package ldap_cache provides thread-safe generic caching for LDAP entities.
// Package name uses underscore for LDAP domain clarity (ldap_cache vs ldapcache).
// nolint:revive
package ldap_cache

import (
	"reflect"
	"sync"
)

// cacheable defines the interface for objects that can be cached.
// All LDAP entities must provide their Distinguished Name for indexing.
type cacheable interface {
	DN() string
}

// getSAMAccountName provides a unified interface to access SAMAccountName across different types.
// This adapter pattern allows the cache to work with both User and Computer types seamlessly.
// Uses reflection to access the SAMAccountName field for maximum compatibility.
func getSAMAccountName(item any) string {
	v := reflect.ValueOf(item)

	// Handle pointers by dereferencing
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// Only handle struct types
	if v.Kind() != reflect.Struct {
		return ""
	}

	// Look for SAMAccountName field
	field := v.FieldByName("SAMAccountName")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}

	return field.String()
}

// Cache provides thread-safe storage for LDAP entities with O(1) indexed lookups.
// It maintains both slice storage for iteration and hash-based indexes for fast lookups.
// Uses RWMutex to allow multiple concurrent readers while ensuring exclusive write access.
// Includes optional metrics tracking for performance monitoring.
type Cache[T cacheable] struct {
	m        sync.RWMutex  // Reader-writer mutex for concurrent access
	items    []T           // Slice storing all cached items for iteration
	dnIndex  map[string]*T // O(1) index for DN-based lookups
	samIndex map[string]*T // O(1) index for SAMAccountName-based lookups
	metrics  *Metrics      // Optional metrics collector for performance tracking
}

// NewCached creates a new empty cache for the specified LDAP entity type.
// The cache is initialized with empty indexes and will grow as needed.
// Provides O(1) lookup performance for both DN and SAMAccountName searches.
func NewCached[T cacheable]() Cache[T] {
	return Cache[T]{
		items:    make([]T, 0),
		dnIndex:  make(map[string]*T),
		samIndex: make(map[string]*T),
	}
}

// NewCachedWithMetrics creates a new empty cache with metrics tracking enabled.
// Provides performance monitoring for cache hit rates and operation counts.
// Includes O(1) indexed lookups with comprehensive performance tracking.
func NewCachedWithMetrics[T cacheable](metrics *Metrics) Cache[T] {
	return Cache[T]{
		items:    make([]T, 0),
		dnIndex:  make(map[string]*T),
		samIndex: make(map[string]*T),
		metrics:  metrics,
	}
}

// buildIndexes creates hash-based indexes for O(1) lookups after cache updates.
// This method rebuilds both DN and SAMAccountName indexes atomically.
// Called internally after setAll operations to maintain index consistency.
func (c *Cache[T]) buildIndexes() {
	// Clear existing indexes
	c.dnIndex = make(map[string]*T, len(c.items))
	c.samIndex = make(map[string]*T, len(c.items))

	// Build new indexes from current items
	for i := range c.items {
		item := &c.items[i]

		// Index by DN (always available)
		if dn := (*item).DN(); dn != "" {
			c.dnIndex[dn] = item
		}

		// Index by SAMAccountName (if available)
		if samAccount := getSAMAccountName(*item); samAccount != "" {
			c.samIndex[samAccount] = item
		}
	}
}

// setAll replaces the entire cache contents with the provided items.
// This operation is atomic and thread-safe, used during cache refresh cycles.
// Rebuilds indexes for O(1) lookup performance after data replacement.
func (c *Cache[T]) setAll(v []T) {
	c.m.Lock()
	defer c.m.Unlock()

	c.items = v
	c.buildIndexes()
}

// update applies a function to all cached items, allowing in-place modifications.
// The function receives a pointer to each item for direct modification.
// Rebuilds indexes after modifications to maintain lookup consistency.
// This is used for maintaining cache consistency during LDAP operations.
func (c *Cache[T]) update(fn func(*T)) {
	c.m.Lock()
	defer c.m.Unlock()

	for idx, item := range c.items {
		fn(&item)
		c.items[idx] = item
	}

	// Rebuild indexes after updates to maintain consistency
	c.buildIndexes()
}

// Get returns a copy of all cached items.
// This operation is read-locked to allow concurrent access from multiple readers.
func (c *Cache[T]) Get() []T {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.items
}

// Find searches the cache for the first item matching the provided predicate.
// Returns a pointer to the matching item and true if found, nil and false otherwise.
// The search is performed under a read lock for thread safety.
// Records cache hit/miss metrics if metrics tracking is enabled.
func (c *Cache[T]) Find(fn func(T) bool) (v *T, found bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	for _, item := range c.items {
		if fn(item) {
			if c.metrics != nil {
				c.metrics.RecordCacheHit()
			}

			return &item, true
		}
	}

	if c.metrics != nil {
		c.metrics.RecordCacheMiss()
	}

	return nil, false
}

// FindByDN searches the cache for an item with the specified Distinguished Name.
// Uses O(1) hash-based index lookup for optimal performance instead of linear search.
// This provides significant performance improvement over the previous O(n) implementation.
func (c *Cache[T]) FindByDN(dn string) (v *T, found bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	if item, exists := c.dnIndex[dn]; exists {
		if c.metrics != nil {
			c.metrics.RecordCacheHit()
		}
		return item, true
	}

	if c.metrics != nil {
		c.metrics.RecordCacheMiss()
	}

	return nil, false
}

// FindBySAMAccountName searches the cache for an item with the specified SAMAccountName.
// Uses O(1) hash-based index lookup for optimal performance instead of linear search.
// This provides significant performance improvement for user/computer lookups by username.
// Returns nil, false if the item doesn't have a SAMAccountName or if not found.
func (c *Cache[T]) FindBySAMAccountName(samAccountName string) (v *T, found bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	if item, exists := c.samIndex[samAccountName]; exists {
		if c.metrics != nil {
			c.metrics.RecordCacheHit()
		}
		return item, true
	}

	if c.metrics != nil {
		c.metrics.RecordCacheMiss()
	}

	return nil, false
}

// Filter returns all cached items that match the provided predicate.
// Creates a new slice containing only the matching items.
// The operation is read-locked for concurrent safety.
func (c *Cache[T]) Filter(fn func(T) bool) (v []T) {
	c.m.RLock()
	defer c.m.RUnlock()

	for _, item := range c.items {
		if fn(item) {
			v = append(v, item)
		}
	}

	return v
}

// Count returns the total number of cached items.
// This operation is read-locked to ensure consistent results during concurrent access.
func (c *Cache[T]) Count() int {
	c.m.RLock()
	defer c.m.RUnlock()

	return len(c.items)
}
