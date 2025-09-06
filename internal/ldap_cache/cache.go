// Package ldap_cache provides thread-safe generic caching for LDAP entities.
package ldap_cache

import (
	"sync"
)

// cacheable defines the interface for objects that can be cached.
// All LDAP entities must provide their Distinguished Name for indexing.
type cacheable interface {
	DN() string
}

// Cache provides thread-safe storage for LDAP entities with concurrent read/write access.
// It uses RWMutex to allow multiple concurrent readers while ensuring exclusive write access.
// Includes optional metrics tracking for performance monitoring.
type Cache[T cacheable] struct {
	m       sync.RWMutex // Reader-writer mutex for concurrent access
	items   []T          // Slice storing all cached items
	metrics *Metrics     // Optional metrics collector for performance tracking
}

// NewCached creates a new empty cache for the specified LDAP entity type.
// The cache is initialized with zero capacity and will grow as needed.
func NewCached[T cacheable]() Cache[T] {
	return Cache[T]{
		items: make([]T, 0),
	}
}

// NewCachedWithMetrics creates a new empty cache with metrics tracking enabled.
// Provides performance monitoring for cache hit rates and operation counts.
func NewCachedWithMetrics[T cacheable](metrics *Metrics) Cache[T] {
	return Cache[T]{
		items:   make([]T, 0),
		metrics: metrics,
	}
}

// setAll replaces the entire cache contents with the provided items.
// This operation is atomic and thread-safe, used during cache refresh cycles.
func (c *Cache[T]) setAll(v []T) {
	c.m.Lock()
	defer c.m.Unlock()

	c.items = v
}

// update applies a function to all cached items, allowing in-place modifications.
// The function receives a pointer to each item for direct modification.
// This is used for maintaining cache consistency during LDAP operations.
func (c *Cache[T]) update(fn func(*T)) {
	c.m.Lock()
	defer c.m.Unlock()

	for idx, item := range c.items {
		fn(&item)
		c.items[idx] = item
	}
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
// This is a convenience method that uses Find() with a DN comparison predicate.
func (c *Cache[T]) FindByDN(dn string) (v *T, found bool) {
	return c.Find(func(v T) bool {
		return v.DN() == dn
	})
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
