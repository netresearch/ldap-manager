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
	overallHealthy := cacheHealthStats.HealthStatus == "healthy" &&
		poolHealthStatus["healthy"].(bool)

	var statusCode int
	if overallHealthy {
		statusCode = fiber.StatusOK
	} else if cacheHealthStats.HealthStatus == "degraded" ||
		(cacheHealthStats.HealthStatus == "healthy" && !poolHealthStatus["healthy"].(bool)) {
		statusCode = fiber.StatusOK // Still functional but degraded
	} else {
		statusCode = fiber.StatusServiceUnavailable
	}

	c.Status(statusCode)

	response := fiber.Map{
		"cache":           cacheHealthStats,
		"connection_pool": poolHealthStatus,
		"overall_healthy": overallHealthy,
	}

	return c.JSON(response)
}

// readinessHandler provides a simple readiness check.
// Returns 200 OK if the cache system and connection pool are operational and ready to serve requests.
// Includes cache warming status and connection pool health to indicate if system is ready.
func (a *App) readinessHandler(c *fiber.Ctx) error {
	isCacheHealthy := a.ldapCache.IsHealthy()
	isWarmedUp := a.ldapCache.IsWarmedUp()
	poolHealthStatus := a.ldapPool.GetHealthStatus()
	isPoolHealthy := poolHealthStatus["healthy"].(bool)

	if isCacheHealthy && isWarmedUp && isPoolHealthy {
		return c.JSON(fiber.Map{
			"status":          "ready",
			"cache":           "healthy",
			"warmed_up":       true,
			"connection_pool": "healthy",
		})
	}

	c.Status(fiber.StatusServiceUnavailable)
	status := "not ready"
	reason := ""

	switch {
	case !isCacheHealthy && !isWarmedUp && !isPoolHealthy:
		reason = "cache unhealthy, not warmed up, and connection pool unhealthy"
	case !isCacheHealthy && !isWarmedUp:
		reason = "cache unhealthy and not warmed up"
	case !isCacheHealthy && !isPoolHealthy:
		reason = "cache and connection pool unhealthy"
	case !isWarmedUp && !isPoolHealthy:
		reason = "cache warming in progress and connection pool unhealthy"
		status = "warming up"
	case !isCacheHealthy:
		reason = "cache degraded or unhealthy"
	case !isWarmedUp:
		reason = "cache warming in progress"
		status = "warming up"
	case !isPoolHealthy:
		reason = "connection pool unhealthy"
	}

	return c.JSON(fiber.Map{
		"status":          status,
		"cache":           reason,
		"warmed_up":       isWarmedUp,
		"connection_pool": "unhealthy",
	})
}

// livenessHandler provides a simple liveness check.
// Returns 200 OK if the application is running and responsive.
func (a *App) livenessHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "alive",
		"uptime": a.ldapCache.GetMetrics().GetUptime().String(),
	})
}
