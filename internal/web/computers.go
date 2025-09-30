package web

// HTTP handlers for computer management endpoints.

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// computersHandler handles GET /computers requests to list all computer accounts in the LDAP directory.
// Supports optional show-disabled query parameter to include disabled computer accounts.
// Computers are sorted alphabetically by CN (Common Name) and returned as HTML using template caching.
//
// Query Parameters:
//   - show-disabled: Set to "1" to include disabled computers in the listing
//
// Returns:
//   - 200: HTML page with computer listing
//   - 500: Internal server error if LDAP query fails
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

// computerHandler handles GET /computers/:computerDN requests to display detailed information for a specific computer.
// The computerDN path parameter must be URL-encoded Distinguished Name of the computer account.
// Returns computer details including attributes, group memberships, and system information.
//
// Path Parameters:
//   - computerDN: URL-encoded Distinguished Name of the computer
//     (e.g. "CN=WORKSTATION01,OU=Computers,DC=example,DC=com")
//
// Returns:
//   - 200: HTML page with computer details and group memberships
//   - 500: Internal server error if computer not found or LDAP query fails
//
// Example:
//
//	GET /computers/CN%3DWORKSTATION01%2COU%3DComputers%2CDC%3Dexample%2CDC%3Dcom
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
