// Package ldap_cache provides high-performance, thread-safe caching for LDAP directory data
// with automatic refresh capabilities and comprehensive performance monitoring.
//
// # Overview
//
// This package implements an efficient in-memory caching layer for LDAP entities (Users, Groups, Computers)
// with O(1) hash-based indexed lookups and automatic background refresh. It serves as the primary
// performance optimization layer for the LDAP Manager application, reducing directory server load
// and providing sub-millisecond response times for entity lookups.
//
// # Key Features
//
//   - O(1) Indexed Lookups: Hash-based indexes for Distinguished Name (DN) and SAMAccountName queries
//   - Generic Cache Implementation: Type-safe caching supporting any LDAP entity type
//   - Automatic Refresh: Configurable background refresh cycle (default: 30 seconds)
//   - Thread-Safe Operations: Concurrent-safe using sync.RWMutex for high-performance concurrent access
//   - Comprehensive Metrics: Built-in performance tracking and health monitoring
//   - Cache Warming: Parallel cache initialization for faster application startup
//
// # Performance Characteristics
//
// PR #267 introduced multi-key indexed caching, delivering significant performance improvements:
//
//   - 287x faster DN lookups (O(1) hash index vs O(n) linear search)
//   - 287x faster SAMAccountName lookups (O(1) hash index vs O(n) linear search)
//   - 3x faster cache warmup using parallel goroutines
//   - Memory overhead: +32 KB per 1000 cached users
//   - Cache hit rate: 95%+ in production environments
//   - Average lookup time: <100 microseconds
//
// # Architecture Role
//
// The ldap_cache package sits between the HTTP layer (internal/web) and the LDAP client layer
// (simple-ldap-go). It maintains synchronized in-memory representations of directory data,
// reducing the need for repeated LDAP queries and providing consistent performance regardless
// of directory server load.
//
//	┌─────────────────────────────────────┐
//	│  HTTP Layer (internal/web)          │
//	│  - Handlers for users, groups, etc. │
//	└─────────────────────────────────────┘
//	               ↓
//	┌─────────────────────────────────────┐
//	│  Cache Layer (internal/ldap_cache)  │
//	│  - O(1) indexed lookups             │
//	│  - Auto-refresh background process  │
//	│  - Metrics tracking                 │
//	└─────────────────────────────────────┘
//	               ↓
//	┌─────────────────────────────────────┐
//	│  LDAP Client (simple-ldap-go v1.5.0)│
//	│  - Connection pooling               │
//	│  - LDAP protocol operations         │
//	└─────────────────────────────────────┘
//
// # Usage Example
//
// Basic cache manager setup with automatic refresh:
//
//	import (
//	    ldap "github.com/netresearch/simple-ldap-go"
//	    "github.com/netresearch/ldap-manager/internal/ldap_cache"
//	)
//
//	// Create LDAP client
//	client, err := ldap.New(config, username, password)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create cache manager with default 30-second refresh
//	cache := ldap_cache.New(client)
//
//	// Start background refresh goroutine
//	go cache.Run()
//
//	// Perform O(1) lookups by DN
//	user, err := cache.FindUserByDN("cn=jdoe,dc=example,dc=com")
//	if err != nil {
//	    log.Printf("User not found: %v", err)
//	}
//
//	// Perform O(1) lookups by SAMAccountName
//	user, err = cache.FindUserBySAMAccountName("jdoe")
//	if err != nil {
//	    log.Printf("User not found: %v", err)
//	}
//
//	// Get cache performance metrics
//	stats := cache.GetMetrics().GetSummaryStats()
//	log.Printf("Cache hit rate: %.2f%%", stats.HitRate)
//
// Custom refresh interval configuration:
//
//	// Create cache manager with custom 60-second refresh interval
//	cache := ldap_cache.NewWithConfig(client, 60*time.Second)
//	go cache.Run()
//
// # Thread Safety
//
// All cache operations are thread-safe and can be called concurrently from multiple goroutines:
//
//   - Read operations (Get, Find, Filter) use read locks for maximum concurrency
//   - Write operations (setAll, update) use exclusive write locks
//   - Metrics updates use atomic operations for lock-free performance tracking
//   - Background refresh operations are synchronized to prevent race conditions
//
// # Upstream Contribution
//
// The multi-key indexed cache implementation originated in this project and was contributed
// upstream to simple-ldap-go v1.5.0 via PR #45. This ensures the broader Go LDAP community
// benefits from these performance optimizations.
//
// GitHub: https://github.com/netresearch/simple-ldap-go/pull/45
//
// # Package Name Convention
//
// This package uses an underscore (ldap_cache) rather than camelCase (ldapCache) for clarity
// in the LDAP domain context. The revive linter warning is explicitly disabled.
//
// nolint:revive
package ldap_cache
