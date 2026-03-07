// Package name uses underscore for LDAP domain clarity (ldap_cache vs ldapcache).
package ldap_cache //nolint:revive // underscore in package name is intentional

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	require.NotNil(t, m)

	assert.Equal(t, int64(0), m.CacheHits)
	assert.Equal(t, int64(0), m.CacheMisses)
	assert.Equal(t, int64(0), m.RefreshCount)
	assert.Equal(t, HealthHealthy, m.GetHealthStatus())
	assert.False(t, m.StartTime.IsZero())
}

func TestMetrics_CacheHitsMisses(t *testing.T) {
	m := NewMetrics()

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()

	assert.InDelta(t, 66.67, m.GetCacheHitRate(), 0.1)
}

func TestMetrics_CacheHitRate_NoOperations(t *testing.T) {
	m := NewMetrics()
	assert.Equal(t, 0.0, m.GetCacheHitRate())
}

func TestMetrics_RefreshCycle(t *testing.T) {
	m := NewMetrics()

	start := m.RecordRefreshStart()
	time.Sleep(10 * time.Millisecond)
	m.RecordRefreshComplete(start, 100, 50, 25)

	users, groups, computers := m.GetEntityCounts()
	assert.Equal(t, int64(100), users)
	assert.Equal(t, int64(50), groups)
	assert.Equal(t, int64(25), computers)
	assert.Greater(t, m.GetLastRefreshDuration(), time.Duration(0))
	assert.Equal(t, HealthHealthy, m.GetHealthStatus())
}

func TestMetrics_RefreshError(t *testing.T) {
	m := NewMetrics()

	// Record a refresh then an error
	m.RecordRefreshStart()
	m.RecordRefreshError()

	// 100% error rate should be unhealthy
	assert.Equal(t, HealthUnhealthy, m.GetHealthStatus())
}

func TestMetrics_HealthStatus_Degraded(t *testing.T) {
	m := NewMetrics()

	// 10 refreshes, 1 error = 10% error rate → degraded stays at threshold
	for range 10 {
		start := m.RecordRefreshStart()
		m.RecordRefreshComplete(start, 10, 5, 2)
	}
	m.RecordRefreshStart()
	m.RecordRefreshError()

	// Error rate = 1/11 ≈ 9.09% > 0% → degraded
	assert.Equal(t, HealthDegraded, m.GetHealthStatus())
}

func TestMetrics_HealthStatus_Unhealthy(t *testing.T) {
	m := NewMetrics()

	// Many errors push rate above 10%
	for range 5 {
		m.RecordRefreshStart()
		m.RecordRefreshError()
	}
	start := m.RecordRefreshStart()
	m.RecordRefreshComplete(start, 10, 5, 2)

	// Error rate = 5/6 ≈ 83% → unhealthy
	assert.Equal(t, HealthUnhealthy, m.GetHealthStatus())
}

func TestMetrics_GetUptime(t *testing.T) {
	m := NewMetrics()
	time.Sleep(10 * time.Millisecond)

	uptime := m.GetUptime()
	assert.Greater(t, uptime, time.Duration(0))
}

func TestMetrics_GetSummaryStats(t *testing.T) {
	m := NewMetrics()

	// Record some activity
	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()

	start := m.RecordRefreshStart()
	m.RecordRefreshComplete(start, 50, 25, 10)

	stats := m.GetSummaryStats()
	assert.InDelta(t, 66.67, stats.CacheHitRate, 0.1)
	assert.Equal(t, int64(3), stats.TotalOperations)
	assert.Equal(t, int64(1), stats.RefreshCount)
	assert.Equal(t, int64(0), stats.RefreshErrors)
	assert.Equal(t, "healthy", stats.HealthStatus)
	assert.Equal(t, int64(50), stats.EntityCounts.Users)
	assert.Equal(t, int64(25), stats.EntityCounts.Groups)
	assert.Equal(t, int64(10), stats.EntityCounts.Computers)
	assert.Greater(t, stats.Uptime, time.Duration(0))
	assert.Greater(t, stats.RefreshDuration, time.Duration(0))
}

func TestMetrics_GetSummaryStats_NoRefresh(t *testing.T) {
	m := NewMetrics()

	stats := m.GetSummaryStats()
	assert.Equal(t, float64(0), stats.CacheHitRate)
	assert.Equal(t, time.Duration(0), stats.LastRefreshAge)
	assert.Equal(t, "healthy", stats.HealthStatus)
}

func TestMetrics_GetSummaryStats_WithErrors(t *testing.T) {
	m := NewMetrics()

	// Create unhealthy state
	for range 3 {
		m.RecordRefreshStart()
		m.RecordRefreshError()
	}

	stats := m.GetSummaryStats()
	assert.Equal(t, "unhealthy", stats.HealthStatus)
	assert.Equal(t, int64(3), stats.RefreshErrors)
	assert.Greater(t, stats.ErrorRate, float64(0))
}

func TestMetrics_GetSummaryStats_Degraded(t *testing.T) {
	m := NewMetrics()

	// Many successes, few errors → degraded
	for range 95 {
		start := m.RecordRefreshStart()
		m.RecordRefreshComplete(start, 10, 5, 2)
	}
	for range 5 {
		m.RecordRefreshStart()
		m.RecordRefreshError()
	}

	stats := m.GetSummaryStats()
	assert.Equal(t, "degraded", stats.HealthStatus)
}

func TestMetrics_UpdateHealthStatus_ZeroRefreshes(t *testing.T) {
	m := NewMetrics()
	// updateHealthStatus with no refreshes should be a no-op
	m.updateHealthStatus()
	assert.Equal(t, HealthHealthy, m.GetHealthStatus())
}

func TestMetrics_ConcurrentReadWrite(t *testing.T) {
	m := NewMetrics()

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(4)
		go func() {
			defer wg.Done()
			m.RecordCacheHit()
		}()
		go func() {
			defer wg.Done()
			m.RecordCacheMiss()
		}()
		go func() {
			defer wg.Done()
			start := m.RecordRefreshStart()
			m.RecordRefreshComplete(start, 10, 5, 2)
		}()
		go func() {
			defer wg.Done()
			_ = m.GetSummaryStats()
		}()
	}

	wg.Wait()

	// No races = success. Final state should be consistent.
	stats := m.GetSummaryStats()
	assert.Greater(t, stats.TotalOperations, int64(0))
}

func TestCacheHealth_Values(t *testing.T) {
	assert.Equal(t, CacheHealth(0), HealthHealthy)
	assert.Equal(t, CacheHealth(1), HealthDegraded)
	assert.Equal(t, CacheHealth(2), HealthUnhealthy)
}
