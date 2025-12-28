package web

// HTTP handlers and middleware for authentication and session management.

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/version"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) logoutHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if err := sess.Destroy(); err != nil {
		return handle500(c, err)
	}

	return c.Redirect("/login")
}

func (a *App) loginHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	username := c.FormValue("username")
	password := c.FormValue("password")

	if username != "" && password != "" {
		user, err := a.ldapReadonly.CheckPasswordForSAMAccountName(username, password)
		if err != nil {
			// Record failed attempt for rate limiting
			ip := c.IP()
			blocked := a.rateLimiter.RecordAttempt(ip)

			log.Warn().
				Err(err).
				Str("username", username).
				Str("ip", ip).
				Int("remaining_attempts", a.rateLimiter.GetRemainingAttempts(ip)).
				Msg("failed login attempt")

			// If blocked after this attempt, return rate limit error
			if blocked {
				return c.Status(fiber.StatusTooManyRequests).
					SendString("Too many failed login attempts. Please try again later.")
			}

			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

			return templates.LoginWithStyles(
				templates.Flashes(templates.ErrorFlash("Invalid username or password")),
				"",
				a.GetCSRFToken(c),
				a.GetStylesPath(),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		// Successful login - reset rate limit counter
		a.rateLimiter.ResetAttempts(c.IP())

		sess.Set("dn", user.DN())
		if err := sess.Save(); err != nil {
			return handle500(c, err)
		}

		log.Info().
			Str("username", username).
			Str("dn", user.DN()).
			Msg("successful login")

		return c.Redirect("/")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.LoginWithStyles(
		templates.Flashes(),
		version.FormatVersion(),
		a.GetCSRFToken(c),
		a.GetStylesPath(),
	).Render(c.UserContext(), c.Response().BodyWriter())
}
