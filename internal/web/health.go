package web

import (
	"github.com/gofiber/fiber/v2"
)

// healthHandler provides a comprehensive health check endpoint.
// Returns cache metrics, system health status, and operational statistics.
func (a *App) healthHandler(c *fiber.Ctx) error {
	healthStats := a.ldapCache.GetHealthCheck()
	
	var statusCode int
	switch healthStats.HealthStatus {
	case "healthy":
		statusCode = fiber.StatusOK
	case "degraded":
		statusCode = fiber.StatusOK // Still functional
	case "unhealthy":
		statusCode = fiber.StatusServiceUnavailable
	default:
		statusCode = fiber.StatusInternalServerError
	}
	
	c.Status(statusCode)
	return c.JSON(healthStats)
}

// readinessHandler provides a simple readiness check.
// Returns 200 OK if the cache system is operational and ready to serve requests.
func (a *App) readinessHandler(c *fiber.Ctx) error {
	if a.ldapCache.IsHealthy() {
		return c.JSON(fiber.Map{
			"status": "ready",
			"cache": "healthy",
		})
	}
	
	c.Status(fiber.StatusServiceUnavailable)
	return c.JSON(fiber.Map{
		"status": "not ready",
		"cache": "degraded or unhealthy",
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