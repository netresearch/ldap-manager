// Package ldap_cache provides metrics and observability for LDAP cache operations.
// Tracks cache performance, hit rates, refresh cycles, and health status.
package ldap_cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks performance and operational statistics for the LDAP cache.
// All counters are thread-safe using atomic operations for high-performance updates.
type Metrics struct {
	// Cache performance metrics
	CacheHits   int64 // Total number of successful cache lookups
	CacheMisses int64 // Total number of failed cache lookups  
	
	// Refresh cycle metrics
	RefreshCount    int64     // Total number of cache refresh operations
	LastRefresh     time.Time // Timestamp of the last successful refresh
	RefreshErrors   int64     // Total number of refresh failures
	RefreshDuration int64     // Duration of last refresh in nanoseconds
	
	// Entity-specific metrics
	UserCount     int64 // Current number of cached users
	GroupCount    int64 // Current number of cached groups  
	ComputerCount int64 // Current number of cached computers
	
	// Operational metrics
	StartTime       time.Time // Cache manager startup time
	UptimeSeconds   int64     // Total uptime in seconds
	HealthStatus    int32     // Health status: 0=healthy, 1=degraded, 2=unhealthy
	ErrorRate       float64   // Recent error rate percentage (0-100)
	
	// Internal state protection
	mu sync.RWMutex // Protects non-atomic fields during updates
}

// CacheHealth represents the overall health status of the cache system.
type CacheHealth int32

const (
	HealthHealthy  CacheHealth = 0 // All systems operational
	HealthDegraded CacheHealth = 1 // Some issues but still functional
	HealthUnhealthy CacheHealth = 2 // Critical issues affecting functionality
)

// NewMetrics creates a new metrics instance with initialized counters.
// Sets the start time to current time and all counters to zero.
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime:    time.Now(),
		HealthStatus: int32(HealthHealthy),
	}
}

// RecordCacheHit increments the cache hit counter atomically.
// Used when a cache lookup successfully finds the requested item.
func (m *Metrics) RecordCacheHit() {
	atomic.AddInt64(&m.CacheHits, 1)
}

// RecordCacheMiss increments the cache miss counter atomically.
// Used when a cache lookup fails to find the requested item.
func (m *Metrics) RecordCacheMiss() {
	atomic.AddInt64(&m.CacheMisses, 1)
}

// RecordRefreshStart records the beginning of a cache refresh operation.
// Updates the refresh count and sets the start time for duration tracking.
func (m *Metrics) RecordRefreshStart() time.Time {
	atomic.AddInt64(&m.RefreshCount, 1)
	return time.Now()
}

// RecordRefreshComplete records the successful completion of a cache refresh.
// Updates the last refresh time, duration, and current entity counts.
func (m *Metrics) RecordRefreshComplete(startTime time.Time, userCount, groupCount, computerCount int) {
	duration := time.Since(startTime)
	
	m.mu.Lock()
	m.LastRefresh = time.Now()
	m.mu.Unlock()
	
	atomic.StoreInt64(&m.RefreshDuration, duration.Nanoseconds())
	atomic.StoreInt64(&m.UserCount, int64(userCount))
	atomic.StoreInt64(&m.GroupCount, int64(groupCount))
	atomic.StoreInt64(&m.ComputerCount, int64(computerCount))
	
	m.updateHealthStatus()
}

// RecordRefreshError increments the refresh error counter.
// Used when a cache refresh operation fails due to LDAP errors.
func (m *Metrics) RecordRefreshError() {
	atomic.AddInt64(&m.RefreshErrors, 1)
	m.updateHealthStatus()
}

// updateHealthStatus calculates and updates the overall health status.
// Uses error rates and refresh success to determine system health.
func (m *Metrics) updateHealthStatus() {
	totalRefresh := atomic.LoadInt64(&m.RefreshCount)
	if totalRefresh == 0 {
		return
	}
	
	errorCount := atomic.LoadInt64(&m.RefreshErrors)
	errorRate := float64(errorCount) / float64(totalRefresh) * 100
	
	m.mu.Lock()
	m.ErrorRate = errorRate
	m.mu.Unlock()
	
	var newStatus CacheHealth
	switch {
	case errorRate == 0:
		newStatus = HealthHealthy
	case errorRate < 10:
		newStatus = HealthDegraded  
	default:
		newStatus = HealthUnhealthy
	}
	
	atomic.StoreInt32(&m.HealthStatus, int32(newStatus))
}

// GetCacheHitRate calculates the cache hit percentage.
// Returns 0.0 if no cache operations have occurred yet.
func (m *Metrics) GetCacheHitRate() float64 {
	hits := atomic.LoadInt64(&m.CacheHits)
	misses := atomic.LoadInt64(&m.CacheMisses)
	total := hits + misses
	
	if total == 0 {
		return 0.0
	}
	
	return float64(hits) / float64(total) * 100
}

// GetUptime returns the total uptime duration since cache manager startup.
func (m *Metrics) GetUptime() time.Duration {
	m.mu.RLock()
	startTime := m.StartTime
	m.mu.RUnlock()
	
	return time.Since(startTime)
}

// GetHealthStatus returns the current health status of the cache system.
func (m *Metrics) GetHealthStatus() CacheHealth {
	return CacheHealth(atomic.LoadInt32(&m.HealthStatus))
}

// GetLastRefreshDuration returns the duration of the most recent refresh operation.
func (m *Metrics) GetLastRefreshDuration() time.Duration {
	nanos := atomic.LoadInt64(&m.RefreshDuration)
	return time.Duration(nanos)
}

// GetEntityCounts returns the current count of cached entities.
func (m *Metrics) GetEntityCounts() (users, groups, computers int64) {
	return atomic.LoadInt64(&m.UserCount),
		   atomic.LoadInt64(&m.GroupCount),
		   atomic.LoadInt64(&m.ComputerCount)
}

// GetSummaryStats returns a comprehensive view of cache metrics.
// Useful for monitoring dashboards and health check endpoints.
type SummaryStats struct {
	CacheHitRate       float64       `json:"cache_hit_rate"`
	TotalOperations    int64         `json:"total_operations"`
	RefreshCount       int64         `json:"refresh_count"`
	RefreshErrors      int64         `json:"refresh_errors"`
	LastRefreshAge     time.Duration `json:"last_refresh_age"`
	RefreshDuration    time.Duration `json:"refresh_duration"`
	Uptime             time.Duration `json:"uptime"`
	HealthStatus       string        `json:"health_status"`
	ErrorRate          float64       `json:"error_rate"`
	EntityCounts       EntityCounts  `json:"entity_counts"`
}

// EntityCounts represents the current count of each entity type.
type EntityCounts struct {
	Users     int64 `json:"users"`
	Groups    int64 `json:"groups"`
	Computers int64 `json:"computers"`
}

// GetSummaryStats returns comprehensive cache statistics for monitoring.
func (m *Metrics) GetSummaryStats() SummaryStats {
	users, groups, computers := m.GetEntityCounts()
	
	m.mu.RLock()
	lastRefresh := m.LastRefresh
	errorRate := m.ErrorRate
	m.mu.RUnlock()
	
	var lastRefreshAge time.Duration
	if !lastRefresh.IsZero() {
		lastRefreshAge = time.Since(lastRefresh)
	}
	
	healthStatus := m.GetHealthStatus()
	var healthStr string
	switch healthStatus {
	case HealthHealthy:
		healthStr = "healthy"
	case HealthDegraded:
		healthStr = "degraded"
	case HealthUnhealthy:
		healthStr = "unhealthy"
	}
	
	return SummaryStats{
		CacheHitRate:    m.GetCacheHitRate(),
		TotalOperations: atomic.LoadInt64(&m.CacheHits) + atomic.LoadInt64(&m.CacheMisses),
		RefreshCount:    atomic.LoadInt64(&m.RefreshCount),
		RefreshErrors:   atomic.LoadInt64(&m.RefreshErrors),
		LastRefreshAge:  lastRefreshAge,
		RefreshDuration: m.GetLastRefreshDuration(),
		Uptime:          m.GetUptime(),
		HealthStatus:    healthStr,
		ErrorRate:       errorRate,
		EntityCounts: EntityCounts{
			Users:     users,
			Groups:    groups,
			Computers: computers,
		},
	}
}