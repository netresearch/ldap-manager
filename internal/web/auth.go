package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
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

	username := c.Query("username")
	password := c.Query("password")

	if username != "" && password != "" {
		user, err := a.ldap.CheckPasswordForSAMAccountName(username, password)
		if err != nil {
			log.Error().Err(err).Msg("could not check password")

			return c.Render("views/login", fiber.Map{
				"session":     sess,
				"title":       "Login",
				"headscripts": "",
				"flashes":     []string{"Invalid username or password"},
			}, "layouts/base")
		}

		sess.Set("dn", user.DN())
		sess.Set("password", password)
		if err := sess.Save(); err != nil {
			return handle500(c, err)
		}

		return c.Redirect("/")
	}

	return c.Render("views/login", fiber.Map{
		"session":     sess,
		"title":       "Login",
		"headscripts": "",
	}, "layouts/base")
}
