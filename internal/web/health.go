package web

import (
	"github.com/gofiber/fiber/v2"
)

// healthHandler provides a comprehensive health check endpoint.
// Returns cache metrics, connection pool health, system health status, and operational statistics.
func (a *App) healthHandler(c *fiber.Ctx) error {
	cacheHealthStats := a.ldapCache.GetHealthCheck()
	poolHealthStatus := a.ldapPool.GetHealthStatus()

	// Determine overall health status
	poolHealthy, ok := poolHealthStatus["healthy"].(bool)
	if !ok {
		poolHealthy = false
	}
	overallHealthy := cacheHealthStats.HealthStatus == "healthy" && poolHealthy

	// Determine status code based on health state
	statusCode := a.getHealthStatusCode(overallHealthy, cacheHealthStats.HealthStatus, poolHealthy)

	c.Status(statusCode)

	response := fiber.Map{
		"cache":           cacheHealthStats,
		"connection_pool": poolHealthStatus,
		"overall_healthy": overallHealthy,
	}

	return c.JSON(response)
}

// getHealthStatusCode determines the appropriate HTTP status code based on health state
func (a *App) getHealthStatusCode(overallHealthy bool, cacheStatus string, poolHealthy bool) int {
	if overallHealthy {
		return fiber.StatusOK
	}
	if cacheStatus == "degraded" || (cacheStatus == "healthy" && !poolHealthy) {
		return fiber.StatusOK // Still functional but degraded
	}

	return fiber.StatusServiceUnavailable
}

// readinessHandler provides a simple readiness check.
// Returns 200 OK if the cache system and connection pool are operational and ready to serve requests.
// Includes cache warming status and connection pool health to indicate if system is ready.
func (a *App) readinessHandler(c *fiber.Ctx) error {
	isCacheHealthy := a.ldapCache.IsHealthy()
	isWarmedUp := a.ldapCache.IsWarmedUp()
	poolHealthStatus := a.ldapPool.GetHealthStatus()
	isPoolHealthy, ok := poolHealthStatus["healthy"].(bool)
	if !ok {
		isPoolHealthy = false
	}

	// Check if fully ready
	if isCacheHealthy && isWarmedUp && isPoolHealthy {
		return c.JSON(fiber.Map{
			"status":          "ready",
			"cache":           "healthy",
			"warmed_up":       true,
			"connection_pool": "healthy",
		})
	}

	// Get status and reason for not ready state
	status, reason := a.getReadinessStatus(isCacheHealthy, isWarmedUp, isPoolHealthy)
	c.Status(fiber.StatusServiceUnavailable)

	return c.JSON(fiber.Map{
		"status":          status,
		"cache":           reason,
		"warmed_up":       isWarmedUp,
		"connection_pool": "unhealthy",
	})
}

const (
	statusNotReady  = "not ready"
	statusWarmingUp = "warming up"
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
func (a *App) livenessHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "alive",
		"uptime": a.ldapCache.GetMetrics().GetUptime().String(),
	})
}
