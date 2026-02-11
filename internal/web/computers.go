package web

// HTTP handlers for computer management endpoints.

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) computersHandler(c *fiber.Ctx) error {
	showDisabled := c.Query("show-disabled", "0") == "1"

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer userLDAP.Close()

	allComputers, err := userLDAP.FindComputers()
	if err != nil {
		return handle500(c, err)
	}

	computers := allComputers
	if !showDisabled {
		computers = nil
		for _, comp := range allComputers {
			if comp.Enabled {
				computers = append(computers, comp)
			}
		}
	}

	sort.SliceStable(computers, func(i, j int) bool {
		return computers[i].CN() < computers[j].CN()
	})

	// Use template caching with query parameter differentiation
	return a.templateCache.RenderWithCache(c, templates.Computers(computers))
}

func (a *App) computerHandler(c *fiber.Ctx) error {
	computerDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer userLDAP.Close()

	computers, err := userLDAP.FindComputers()
	if err != nil {
		return handle500(c, err)
	}

	computer := findComputerByDN(computers, computerDN)
	if computer == nil {
		return handle500(c, ldap.ErrComputerNotFound)
	}

	groups, err := userLDAP.FindGroups()
	if err != nil {
		return handle500(c, err)
	}

	fullComputer := ldap_cache.PopulateGroupsForComputerFromData(computer, groups)
	sort.SliceStable(fullComputer.Groups, func(i, j int) bool {
		return fullComputer.Groups[i].CN() < fullComputer.Groups[j].CN()
	})

	// Use template caching with computer DN as additional cache data
	return a.templateCache.RenderWithCache(
		c,
		templates.Computer(fullComputer),
		"computerDN:"+computerDN,
	)
}

// findComputerByDN searches for a computer by DN in a slice.
func findComputerByDN(computers []ldap.Computer, dn string) *ldap.Computer {
	for i := range computers {
		if computers[i].DN() == dn {
			return &computers[i]
		}
	}

	return nil
}
