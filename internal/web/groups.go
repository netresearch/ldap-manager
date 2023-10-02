package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
)

func (a *App) groupsHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	groups := a.ldapCache.FindGroups()

	return c.Render("views/groups", fiber.Map{
		"session":     sess,
		"title":       "All groups",
		"activePage":  "/groups",
		"headscripts": "",
		"groups":      groups,
	}, "layouts/logged-in")
}

func (a *App) groupHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	groupDN, err := url.PathUnescape(c.Params("groupDN"))
	if err != nil {
		return handle500(c, err)
	}

	thinGroup, err := a.ldapCache.FindGroupByDN(groupDN)
	if err != nil {
		return handle500(c, err)
	}

	group := a.ldapCache.PopulateUsersForGroup(&thinGroup)

	return c.Render("views/group", fiber.Map{
		"session":     sess,
		"title":       "All groups",
		"activePage":  "/groups",
		"headscripts": "",
		"group":       group,
	}, "layouts/logged-in")
}
