// Package web — V2 /groups list + group detail handlers (spec §6.2).
package web

import (
	"net/url"
	"sort"
	"strings"

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

	var unassigned []ldap.User
	if a.ldapCache != nil {
		unassigned = filterUnassignedUsers(a.ldapCache.FindUsers(true), fullGroup)
		sortUsersByCN(unassigned)
	}

	return templates.GroupDrawerVM{
		Group:           fullGroup,
		Pinned:          pinned,
		OUName:          ouName,
		OUPivotHref:     buildGroupOUPivotHref(ouName),
		UnassignedUsers: unassigned,
	}, true
}

// populateMembersForGroup resolves the group's Members DN list into a
// FullLDAPGroup with []ldap.User, and additionally resolves its MemberOf
// list into ParentGroups. When the cache is nil the result has both
// slices empty.
func (a *App) populateMembersForGroup(group *ldap.Group) *ldap_cache.FullLDAPGroup {
	var (
		users  []ldap.User
		groups []ldap.Group
	)

	if a.ldapCache != nil {
		users = a.ldapCache.FindUsers(true)
		groups = a.ldapCache.FindGroups()
	}

	return ldap_cache.PopulateUsersForGroupFromData(group, users, groups, true)
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
	memberDN := c.Query("member")

	var all []ldap.Group
	if a.ldapCache != nil {
		all = a.ldapCache.FindGroups()
	}

	ous := distinctImmediateOUsFromGroups(all)
	groups := filterGroupsByOU(all, ouFilter)
	groups = filterGroupsByMember(groups, memberDN)
	sortGroupsByCN(groups)

	memberCN := lookupUserCN(memberDN, a.ldapCache)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.GroupsListV2(groups, ouFilter, memberDN, memberCN, ous, templates.Flashes(), a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// filterGroupsByMember narrows to groups that list the given user DN in
// their Members attribute. Empty memberDN is a no-op. Inverse of
// filterUsersByMemberOf; both look up the group list the same way.
func filterGroupsByMember(groups []ldap.Group, memberDN string) []ldap.Group {
	if memberDN == "" {
		return groups
	}

	out := make([]ldap.Group, 0, len(groups))
	for _, g := range groups {
		for _, m := range g.Members {
			if m == memberDN {
				out = append(out, g)

				break
			}
		}
	}

	return out
}

// lookupUserCN resolves a user DN to its CN via ldap_cache. Empty DN or
// cache miss both yield "".
func lookupUserCN(userDN string, cache *ldap_cache.Manager) string {
	if userDN == "" || cache == nil {
		return ""
	}

	for _, u := range cache.FindUsers(true) {
		if u.DN() == userDN {
			return u.CN()
		}
	}

	return ""
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
		c.Status(fiber.StatusBadRequest)
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

		return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
	}

	vm, ok := a.buildGroupDrawerVM(groupDN, viewerDN)
	if !ok {
		c.Status(fiber.StatusNotFound)
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

		return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
	}

	vm.CSRFToken = a.GetCSRFToken(c)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" && c.Get("HX-Request") == "true" {
		return templates.GroupDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.GroupFullV2(vm, a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// sortGroupsByCN sorts a slice of groups in place by CN, case-insensitive.
func sortGroupsByCN(groups []ldap.Group) {
	sort.SliceStable(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].CN()) < strings.ToLower(groups[j].CN())
	})
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
