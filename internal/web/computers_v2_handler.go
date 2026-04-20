// Package web — V2 /computers list + computer detail handlers (spec §6.2).
package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// buildComputerDrawerVM hydrates the computer drawer view-model for a given
// computer DN. Returns (vm, found). The result is safe to render in both the
// drawer-fragment and full-page contexts — the caller chooses the wrapper.
func (a *App) buildComputerDrawerVM(computerDN, viewerDN string) (templates.ComputerDrawerVM, bool) {
	computer, ok := a.lookupComputerByDN(computerDN)
	if !ok {
		return templates.ComputerDrawerVM{}, false
	}

	pinned := false
	if a.pinnedStore != nil && viewerDN != "" {
		pinned, _ = a.pinnedStore.IsPinned(viewerDN, computerDN)
	}

	ouName := immediateOU(computerDN)

	return templates.ComputerDrawerVM{
		Computer:    computer,
		Pinned:      pinned,
		OUName:      ouName,
		OUPivotHref: buildComputerOUPivotHref(ouName),
	}, true
}

// buildComputerOUPivotHref returns a `/computers?ou=…` pivot link. Empty
// string when the OU cannot be derived from the DN.
func buildComputerOUPivotHref(ou string) string {
	if ou == "" {
		return ""
	}

	v := url.Values{}
	v.Set("ou", ou)

	return "/computers?" + v.Encode()
}

// handleComputersV2 renders the new /computers list page (spec §6.2).
func (a *App) handleComputersV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	ouFilter := c.Query("ou")

	var computers []ldap.Computer
	if a.ldapCache != nil {
		all := a.ldapCache.FindComputers(true)
		computers = filterComputersByOU(all, ouFilter)
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.ComputersListV2(computers, ouFilter, templates.Flashes(), a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// handleComputerV2 renders either the drawer fragment (?fragment=drawer) or
// the full computer detail page at /computers/:dn.
//
// Each handler dispatches to a different type-specific VM builder and template;
// unifying into a generic helper would force interface indirection that obscures
// the type contracts. Kept parallel by convention.
//
//nolint:dupl // Intentional structural parallel with handleUserV2 and handleGroupV2.
func (a *App) handleComputerV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	// Route is registered as /computers/* — matches the legacy pattern so
	// existing tests keep working. `c.Params("*")` yields the URL-encoded
	// DN exactly as the client sent it.
	computerDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	vm, ok := a.buildComputerDrawerVM(computerDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("computer not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" {
		return templates.ComputerDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.ComputerFullV2(vm, a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// filterComputersByOU returns computers whose immediate OU matches ou. When
// ou is empty the input is returned unchanged.
func filterComputersByOU(computers []ldap.Computer, ou string) []ldap.Computer {
	if ou == "" {
		return computers
	}

	out := make([]ldap.Computer, 0, len(computers))
	for _, cp := range computers {
		if immediateOU(cp.DN()) == ou {
			out = append(out, cp)
		}
	}

	return out
}
