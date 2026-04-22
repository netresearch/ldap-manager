// Package web — V2 /users list + user detail handlers (spec §6.2).
package web

import (
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// buildUserDrawerVM hydrates the drawer view-model for a given user DN.
// Returns (vm, found). The result is safe to render in both drawer-fragment
// and full-page contexts — the caller chooses the wrapper.
//
// Groups are resolved from the cache against the user's memberOf DN list so
// the template can render them as tags with proper CN/DN. When the cache is
// not available (no service account configured) the group slice is empty.
func (a *App) buildUserDrawerVM(userDN, viewerDN string) (templates.UserDrawerVM, bool) {
	user, ok := a.lookupUserByDN(userDN)
	if !ok {
		return templates.UserDrawerVM{}, false
	}

	pinned := false
	if a.pinnedStore != nil && viewerDN != "" {
		pinned, _ = a.pinnedStore.IsPinned(viewerDN, userDN)
	}

	fullUser := a.populateGroupsForUser(&user)

	ouFilter := immediateOU(userDN)

	var unassigned []ldap.Group
	if a.ldapCache != nil {
		unassigned = filterUnassignedGroups(a.ldapCache.FindGroups(), fullUser)
		sortGroupsByCN(unassigned)
	}

	return templates.UserDrawerVM{
		User:             fullUser,
		Pinned:           pinned,
		OUName:           ouFilter,
		OUPivotHref:      buildOUPivotHref(ouFilter),
		UnassignedGroups: unassigned,
	}, true
}

// populateGroupsForUser resolves the user's memberOf DN list into a
// FullLDAPUser with []ldap.Group. When the cache is nil the result has an
// empty group slice.
func (a *App) populateGroupsForUser(user *ldap.User) *ldap_cache.FullLDAPUser {
	var groups []ldap.Group
	if a.ldapCache != nil {
		groups = a.ldapCache.FindGroups()
	}

	return ldap_cache.PopulateGroupsForUserFromData(user, groups)
}

// buildOUPivotHref returns a `/users?ou=…` pivot link. Empty string when
// the OU cannot be derived from the DN.
func buildOUPivotHref(ou string) string {
	if ou == "" {
		return ""
	}

	v := url.Values{}
	v.Set("ou", ou)

	return "/users?" + v.Encode()
}

// handleUsersV2 renders the new /users list page (spec §6.2).
func (a *App) handleUsersV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	showDisabled := c.Query("show-disabled") == "1"
	ouFilter := c.Query("ou")
	lastLogon := c.Query("last-logon")
	memberOf := c.Query("memberOf")

	var all []ldap.User
	if a.ldapCache != nil {
		all = a.ldapCache.FindUsers(showDisabled)
	}

	ous := distinctImmediateOUsFromUsers(all)

	users := filterUsersByOU(all, ouFilter)
	users = filterUsersByLastLogon(users, lastLogon)
	users = filterUsersByMemberOf(users, memberOf, a.ldapCache)
	sortUsersByCN(users)

	memberOfCN := lookupGroupCN(memberOf, a.ldapCache)
	adminDNs := adminUserDNs(a.ldapCache)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	page := templates.UsersListV2(
		users, showDisabled, ouFilter, lastLogon,
		memberOf, memberOfCN, ous,
		a.takeFlash(c), a.paletteContextFor(viewerDN),
		adminDNs,
	)

	return page.Render(c.UserContext(), c.Response().BodyWriter())
}

// adminUserDNs collects the DNs of users that are members of a group
// whose CN matches a short "privileged" name list (admins, domain
// admins, enterprise admins — case-insensitive). Returns a set so the
// list template's check stays O(1) per row. A nil cache yields nil.
func adminUserDNs(cache *ldap_cache.Manager) map[string]struct{} {
	if cache == nil {
		return nil
	}

	privilegedNames := map[string]struct{}{
		"admins":            {},
		"domain admins":     {},
		"enterprise admins": {},
		"administrators":    {},
		"schema admins":     {},
	}

	out := make(map[string]struct{})

	for _, g := range cache.FindGroups() {
		cn := strings.ToLower(g.CN())
		if _, ok := privilegedNames[cn]; !ok {
			continue
		}

		for _, memberDN := range g.Members {
			out[memberDN] = struct{}{}
		}
	}

	return out
}

// sortUsersByCN sorts a slice of users in place by CN, case-insensitive,
// stable. AD and OpenLDAP both return results in server-specified order
// which is effectively random from an operator's point of view; a
// predictable alphabetic sort makes the list scan-ready.
func sortUsersByCN(users []ldap.User) {
	sort.SliceStable(users, func(i, j int) bool {
		return strings.ToLower(users[i].CN()) < strings.ToLower(users[j].CN())
	})
}

// lookupGroupCN resolves a group DN to its CN via ldap_cache. Empty DN or
// cache miss both return "" — callers can still render a generic "member
// of group" label when only the DN is known.
func lookupGroupCN(groupDN string, cache *ldap_cache.Manager) string {
	if groupDN == "" || cache == nil {
		return ""
	}

	for _, g := range cache.FindGroups() {
		if g.DN() == groupDN {
			return g.CN()
		}
	}

	return ""
}

// handleUserV2 renders either the drawer fragment (?fragment=drawer) or the
// full user detail page at /users/:dn.
//
// Each handler dispatches to a different type-specific VM builder and template;
// unifying into a generic helper would force interface indirection that obscures
// the type contracts. Kept parallel by convention.
//
//nolint:dupl // Intentional structural parallel with handleGroupV2 and handleComputerV2.
func (a *App) handleUserV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	// Route is registered as /users/* — matches the legacy pattern so
	// existing tests keep working. `c.Params("*")` yields the URL-encoded
	// DN exactly as the client sent it.
	userDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		c.Status(fiber.StatusBadRequest)
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

		return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
	}

	vm, ok := a.buildUserDrawerVM(userDN, viewerDN)
	if !ok {
		c.Status(fiber.StatusNotFound)
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

		return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
	}

	vm.CSRFToken = a.GetCSRFToken(c)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	// Only honour ?fragment=drawer for actual htmx requests. A plain reload
	// of the URL (F5) lands here without HX-Request and deserves the full
	// styled page, not a bare fragment that would inherit no CSS.
	if c.Query("fragment") == "drawer" && c.Get("HX-Request") == "true" {
		return templates.UserDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.UserFullV2(vm, a.paletteContextFor(viewerDN)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// filterUsersByLastLogon narrows users by the `last-logon` query-param
// window. Recognised values:
//
//	""       → no filter (input returned unchanged)
//	"24h"    → logged in within the last 24 hours
//	"7d"     → logged in within the last 7 days
//	"30d"    → logged in within the last 30 days
//	"never"  → users with no recorded lastLogonTimestamp (LastLogon == 0)
//
// Unknown values are treated as no-filter to keep the handler tolerant of
// stale URLs.
//
// ldap.User.LastLogon is already a Unix timestamp in seconds (produced by
// simple-ldap-go's parseLastLogonTimestamp), so no AD FILETIME conversion
// is needed here.
func filterUsersByLastLogon(users []ldap.User, window string) []ldap.User {
	if window == "" {
		return users
	}

	if window == "never" {
		out := make([]ldap.User, 0, len(users))
		for _, u := range users {
			if u.LastLogon == 0 {
				out = append(out, u)
			}
		}

		return out
	}

	var cutoff time.Time

	switch window {
	case "24h":
		cutoff = time.Now().Add(-24 * time.Hour)
	case "7d":
		cutoff = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		cutoff = time.Now().Add(-30 * 24 * time.Hour)
	default:
		return users
	}

	out := make([]ldap.User, 0, len(users))
	for _, u := range users {
		if u.LastLogon == 0 {
			continue
		}

		if time.Unix(u.LastLogon, 0).After(cutoff) {
			out = append(out, u)
		}
	}

	return out
}

// filterUsersByOU returns users whose immediate OU matches ou. When ou is
// empty the input is returned unchanged.
func filterUsersByOU(users []ldap.User, ou string) []ldap.User {
	if ou == "" {
		return users
	}

	out := make([]ldap.User, 0, len(users))
	for _, u := range users {
		if immediateOU(u.DN()) == ou {
			out = append(out, u)
		}
	}

	return out
}

// filterUsersByMemberOf narrows to users belonging to groupDN. Looks up the
// target group from the cache and keeps only users whose DN appears in that
// group's member list. Empty groupDN is a no-op; missing group is a no-op
// (better UX than dropping everything to zero with no feedback).
func filterUsersByMemberOf(users []ldap.User, groupDN string, cache *ldap_cache.Manager) []ldap.User {
	if groupDN == "" || cache == nil {
		return users
	}

	var groupMembers map[string]struct{}
	for _, g := range cache.FindGroups() {
		if g.DN() != groupDN {
			continue
		}

		groupMembers = make(map[string]struct{}, len(g.Members))
		for _, memberDN := range g.Members {
			groupMembers[memberDN] = struct{}{}
		}

		break
	}

	if groupMembers == nil {
		return users
	}

	out := make([]ldap.User, 0, len(groupMembers))
	for _, u := range users {
		if _, ok := groupMembers[u.DN()]; ok {
			out = append(out, u)
		}
	}

	return out
}
