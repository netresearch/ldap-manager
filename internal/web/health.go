package web

import (
	"github.com/gofiber/fiber/v3"
	ldap "github.com/netresearch/simple-ldap-go"
)

// poolIsHealthy reports whether the service account's connection pool can
// serve requests.
//
// PoolStats is nil exactly when the client has no pool, which the health
// endpoints treat as unhealthy: every service account client is built with
// one (serviceAccountLDAPConfig). With a pool, TotalConnections > 0 is the
// signal, except when MinConnections is 0: the idle cleanup then drains an
// unused pool to zero connections, and a healthy client would read as down
// (simple-ldap-go v1.18.0 changelog, #247). Such a pool is healthy while it
// exists; a request that needs a connection opens one.
func poolIsHealthy(stats ldap.PerformanceStats) bool {
	if stats.PoolStats == nil {
		return false
	}

	return stats.TotalConnections > 0 || stats.PoolStats.MinConnections == 0
}

// healthHandler provides a comprehensive health check endpoint.
// Returns cache metrics, connection pool health, system health status, and operational statistics.
// When no service account is configured, reports a simplified status.
func (a *App) healthHandler(c fiber.Ctx) error {
	if a.ldapCache == nil || a.ldapReadonly == nil {
		return c.JSON(fiber.Map{
			"overall_healthy": true,
			"mode":            "per-user credentials",
			"cache":           "disabled (no service account)",
			"connection_pool": "disabled (no service account)",
		})
	}

	cacheHealthStats := a.ldapCache.GetHealthCheck()
	poolStats := a.ldapReadonly.GetPoolStats()

	poolHealthy := poolIsHealthy(poolStats)

	overallHealthy := cacheHealthStats.HealthStatus == statusHealthy && poolHealthy

	// Determine status code based on health state
	statusCode := a.getHealthStatusCode(overallHealthy, cacheHealthStats.HealthStatus, poolHealthy)

	c.Status(statusCode)

	response := fiber.Map{
		"cache":           cacheHealthStats,
		"connection_pool": poolStats,
		"overall_healthy": overallHealthy,
	}

	return c.JSON(response)
}

// getHealthStatusCode determines the appropriate HTTP status code based on health state
func (a *App) getHealthStatusCode(overallHealthy bool, cacheStatus string, poolHealthy bool) int {
	if overallHealthy {
		return fiber.StatusOK
	}
	if cacheStatus == "degraded" || (cacheStatus == statusHealthy && !poolHealthy) {
		return fiber.StatusOK // Still functional but degraded
	}

	return fiber.StatusServiceUnavailable
}

// readinessHandler provides a simple readiness check.
// Returns 200 OK if the system is operational and ready to serve requests.
// When no service account is configured, always reports ready (auth happens per-request).
func (a *App) readinessHandler(c fiber.Ctx) error {
	if a.ldapCache == nil || a.ldapReadonly == nil {
		return c.JSON(fiber.Map{
			"status": "ready",
			"mode":   "per-user credentials",
		})
	}

	isCacheHealthy := a.ldapCache.IsHealthy()
	isWarmedUp := a.ldapCache.IsWarmedUp()
	poolStats := a.ldapReadonly.GetPoolStats()
	isPoolHealthy := poolIsHealthy(poolStats)

	// Check if fully ready
	if isCacheHealthy && isWarmedUp && isPoolHealthy {
		return c.JSON(fiber.Map{
			"status":          "ready",
			"cache":           statusHealthy,
			"warmed_up":       true,
			"connection_pool": statusHealthy,
		})
	}

	// Get status and reason for not ready state
	status, reason := a.getReadinessStatus(isCacheHealthy, isWarmedUp, isPoolHealthy)
	c.Status(fiber.StatusServiceUnavailable)

	poolStatus := statusUnhealthy
	if isPoolHealthy {
		poolStatus = statusHealthy
	}

	return c.JSON(fiber.Map{
		"status":          status,
		"cache":           reason,
		"warmed_up":       isWarmedUp,
		"connection_pool": poolStatus,
	})
}

const (
	statusNotReady  = "not ready"
	statusWarmingUp = "warming up"
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"
)

// getReadinessStatus determines status and reason based on readiness conditions
func (a *App) getReadinessStatus(cacheHealthy, warmedUp, poolHealthy bool) (status, reason string) {
	// Check all unhealthy conditions
	if !cacheHealthy && !warmedUp && !poolHealthy {
		return statusNotReady, "cache unhealthy, not warmed up, and connection pool unhealthy"
	}
	if !cacheHealthy && !warmedUp {
		return statusNotReady, "cache unhealthy and not warmed up"
	}
	if !cacheHealthy && !poolHealthy {
		return statusNotReady, "cache and connection pool unhealthy"
	}
	if !warmedUp && !poolHealthy {
		return statusWarmingUp, "cache warming in progress and connection pool unhealthy"
	}
	if !cacheHealthy {
		return statusNotReady, "cache degraded or unhealthy"
	}
	if !warmedUp {
		return statusWarmingUp, "cache warming in progress"
	}
	if !poolHealthy {
		return statusNotReady, "connection pool unhealthy"
	}

	return "", ""
}

// livenessHandler provides a simple liveness check.
// Returns 200 OK if the application is running and responsive.
func (a *App) livenessHandler(c fiber.Ctx) error {
	response := fiber.Map{
		"status": "alive",
	}

	if a.ldapCache != nil {
		response["uptime"] = a.ldapCache.GetMetrics().GetUptime().String()
	}

	return c.JSON(response)
}
