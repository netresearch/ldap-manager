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
// Includes cache warming status to indicate if initial population is complete.
func (a *App) readinessHandler(c *fiber.Ctx) error {
	isHealthy := a.ldapCache.IsHealthy()
	isWarmedUp := a.ldapCache.IsWarmedUp()

	if isHealthy && isWarmedUp {
		return c.JSON(fiber.Map{
			"status":    "ready",
			"cache":     "healthy",
			"warmed_up": true,
		})
	}

	c.Status(fiber.StatusServiceUnavailable)
	status := "not ready"
	reason := ""

	switch {
	case !isHealthy && !isWarmedUp:
		reason = "cache unhealthy and not warmed up"
	case !isHealthy:
		reason = "cache degraded or unhealthy"
	case !isWarmedUp:
		reason = "cache warming in progress"
		status = "warming up"
	}

	return c.JSON(fiber.Map{
		"status":    status,
		"cache":     reason,
		"warmed_up": isWarmedUp,
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
