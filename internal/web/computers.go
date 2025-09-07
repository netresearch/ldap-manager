package web

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) computersHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	showDisabled := c.Query("show-disabled", "0") == "1"
	computers := a.ldapCache.FindComputers(showDisabled)
	sort.SliceStable(computers, func(i, j int) bool {
		return computers[i].CN() < computers[j].CN()
	})

	// Use template caching with query parameter differentiation
	return a.templateCache.RenderWithCache(c, templates.Computers(computers))
}

func (a *App) computerHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
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

	// Use template caching with computer DN as additional cache data
	return a.templateCache.RenderWithCache(
		c,
		templates.Computer(computer),
		"computerDN:"+computerDN,
	)
}
