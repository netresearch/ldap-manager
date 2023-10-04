package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
)

func (a *App) computersHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	computers := a.ldapCache.FindComputers()

	return c.Render("views/computers", fiber.Map{
		"session":     sess,
		"title":       "All computers",
		"activePage":  "/computers",
		"headscripts": "",
		"flashes":     []Flash{},
		"computers":   computers,
	}, "layouts/logged-in")
}

func (a *App) computerHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	computerDN, err := url.PathUnescape(c.Params("computerDN"))
	if err != nil {
		return handle500(c, err)
	}

	thinComputer, err := a.ldapCache.FindComputerByDN(computerDN)
	if err != nil {
		return handle500(c, err)
	}

	computer := a.ldapCache.PopulateGroupsForComputer(thinComputer)

	return c.Render("views/computer", fiber.Map{
		"session":     sess,
		"title":       computer.CN(),
		"activePage":  "/computers",
		"headscripts": "",
		"flashes":     []Flash{},
		"computer":    computer,
	}, "layouts/logged-in")
}
