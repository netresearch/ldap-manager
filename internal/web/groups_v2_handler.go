// Package web — V2 /groups list + group detail handlers (spec §6.2).
package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// buildGroupDrawerVM hydrates the group drawer view-model for a given group
// DN. Returns (vm, found). The result is safe to render in both
// drawer-fragment and full-page contexts — the caller chooses the wrapper.
//
// Members are resolved from the cache against the group's Members DN list so
// the template can render them as tags with proper CN/DN. When the cache is
// not available the member slice is empty.
func (a *App) buildGroupDrawerVM(groupDN, viewerDN string) (templates.GroupDrawerVM, bool) {
	group, ok := a.lookupGroupByDN(groupDN)
	if !ok {
		return templates.GroupDrawerVM{}, false
	}

	pinned := false
	if a.pinnedStore != nil && viewerDN != "" {
		pinned, _ = a.pinnedStore.IsPinned(viewerDN, groupDN)
	}

	fullGroup := a.populateMembersForGroup(&group)
	ouName := immediateOU(groupDN)

	return templates.GroupDrawerVM{
		Group:       fullGroup,
		Pinned:      pinned,
		OUName:      ouName,
		OUPivotHref: buildGroupOUPivotHref(ouName),
	}, true
}

// populateMembersForGroup resolves the group's Members DN list into a
// FullLDAPGroup with []ldap.User. When the cache is nil the result has an
// empty member slice.
func (a *App) populateMembersForGroup(group *ldap.Group) *ldap_cache.FullLDAPGroup {
	var users []ldap.User
	if a.ldapCache != nil {
		users = a.ldapCache.FindUsers(true)
	}

	return ldap_cache.PopulateMembersForGroupFromData(group, users)
}

// buildGroupOUPivotHref returns a `/groups?ou=…` pivot link. Empty string
// when the OU cannot be derived from the DN.
func buildGroupOUPivotHref(ou string) string {
	if ou == "" {
		return ""
	}

	v := url.Values{}
	v.Set("ou", ou)

	return "/groups?" + v.Encode()
}

// handleGroupsV2 renders the new /groups list page (spec §6.2).
func (a *App) handleGroupsV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	ouFilter := c.Query("ou")

	var all []ldap.Group
	if a.ldapCache != nil {
		all = a.ldapCache.FindGroups()
	}

	ous := distinctImmediateOUsFromGroups(all)
	groups := filterGroupsByOU(all, ouFilter)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.GroupsListV2(groups, ouFilter, ous, templates.Flashes(), a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// handleGroupV2 renders either the drawer fragment (?fragment=drawer) or the
// full group detail page at /groups/:dn.
//
// Each handler dispatches to a different type-specific VM builder and template;
// unifying into a generic helper would force interface indirection that obscures
// the type contracts. Kept parallel by convention.
//
//nolint:dupl // Intentional structural parallel with handleUserV2 and handleComputerV2.
func (a *App) handleGroupV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	// Route is registered as /groups/* — matches the legacy pattern so
	// existing tests keep working. `c.Params("*")` yields the URL-encoded
	// DN exactly as the client sent it.
	groupDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	vm, ok := a.buildGroupDrawerVM(groupDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("group not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" && c.Get("HX-Request") == "true" {
		return templates.GroupDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.GroupFullV2(vm, a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// filterGroupsByOU returns groups whose immediate OU matches ou. When ou is
// empty the input is returned unchanged.
func filterGroupsByOU(groups []ldap.Group, ou string) []ldap.Group {
	if ou == "" {
		return groups
	}

	out := make([]ldap.Group, 0, len(groups))
	for _, g := range groups {
		if immediateOU(g.DN()) == ou {
			out = append(out, g)
		}
	}

	return out
}
