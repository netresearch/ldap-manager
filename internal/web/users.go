package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
)

func (a *App) usersHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	users := a.ldapCache.FindUsers()

	return c.Render("views/users", fiber.Map{
		"session":     sess,
		"title":       "All users",
		"activePage":  "/users",
		"headscripts": "",
		"users":       users,
	}, "layouts/logged-in")
}

func (a *App) userHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	userDN, err := url.PathUnescape(c.Params("userDN"))
	if err != nil {
		return handle500(c, err)
	}

	thinUser, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	user := a.ldapCache.PopulateGroupsForUser(thinUser)

	return c.Render("views/user", fiber.Map{
		"session":     sess,
		"title":       user.CN(),
		"activePage":  "/users",
		"headscripts": "",
		"user":        user,
	}, "layouts/logged-in")
}
