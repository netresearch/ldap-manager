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
			log.Error().Err(err).Msg("could not check password")

			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

			return templates.Login(
				templates.Flashes(templates.ErrorFlash("Invalid username or password")),
				"",
				a.GetCSRFToken(c),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		sess.Set("dn", user.DN())
		if err := sess.Save(); err != nil {
			return handle500(c, err)
		}

		return c.Redirect("/")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.Login(templates.Flashes(), version.FormatVersion(), a.GetCSRFToken(c)).Render(
		c.UserContext(), c.Response().BodyWriter())
}
