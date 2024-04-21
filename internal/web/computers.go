package web

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) computersHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	showDisabled := c.Query("show-disabled", "0") == "1"
	computers := a.ldapCache.FindComputers(showDisabled)
	sort.SliceStable(computers, func(i, j int) bool {
		return computers[i].CN() < computers[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.Computers(computers).Render(c.UserContext(), c.Response().BodyWriter())
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
	sort.SliceStable(computer.Groups, func(i, j int) bool {
		return computer.Groups[i].CN() < computer.Groups[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.Computer(computer).Render(c.UserContext(), c.Response().BodyWriter())
}
