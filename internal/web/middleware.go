package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// RequireAuth middleware ensures user is authenticated before accessing protected routes
// It checks for valid session and redirects unauthenticated users to login page
func (a *App) RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := a.sessionStore.Get(c)
		if err != nil {
			log.Error().Err(err).Msg("failed to get session in auth middleware")

			return c.Redirect("/login")
		}

		// Check if session is fresh (no authenticated user)
		if sess.Fresh() {
			log.Debug().Str("path", c.Path()).Msg("unauthenticated access attempt, redirecting to login")

			return c.Redirect("/login")
		}

		// Get user DN from session
		userDN, ok := sess.Get("dn").(string)
		if !ok || userDN == "" {
			log.Warn().Msg("session exists but no user DN found, redirecting to login")

			return c.Redirect("/login")
		}

		// Store user DN and username in context for handlers to use
		c.Locals("userDN", userDN)
		if username, ok := sess.Get("username").(string); ok {
			c.Locals("username", username)
		}

		log.Debug().
			Str("userDN", userDN).
			Str("path", c.Path()).
			Msg("authenticated user accessing protected route")

		return c.Next()
	}
}

// GetUserDN is a helper function to retrieve authenticated user DN from context
// Returns empty string if user is not authenticated
func GetUserDN(c *fiber.Ctx) string {
	if userDN, ok := c.Locals("userDN").(string); ok {
		return userDN
	}

	return ""
}

// RequireUserDN is a helper function that returns user DN or handles error
// Should only be called from handlers protected by RequireAuth middleware
func RequireUserDN(c *fiber.Ctx) (string, error) {
	userDN := GetUserDN(c)
	if userDN == "" {
		log.Error().Str("path", c.Path()).Msg("RequireUserDN called but no user DN in context")

		return "", fiber.NewError(fiber.StatusUnauthorized, "Authentication required")
	}

	return userDN, nil
}
