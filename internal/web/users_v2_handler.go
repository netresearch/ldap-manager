// Package web — V2 /users list + user detail handlers (spec §6.2).
package web

import (
	"net/url"

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

	return templates.UserDrawerVM{
		User:        fullUser,
		Pinned:      pinned,
		OUName:      ouFilter,
		OUPivotHref: buildOUPivotHref(ouFilter),
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
	// Prefer the DN already populated by RequireAuth into c.Locals —
	// re-reading the session after CSRF middleware can return a fresh
	// session on this code path and drop the "dn" key.
	viewerDN := GetUserDN(c)
	if viewerDN == "" {
		sess, err := a.sessionStore.Get(c)
		if err != nil {
			return handle500(c, err)
		}
		viewerDN, _ = sess.Get("dn").(string)
	}

	if viewerDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	showDisabled := c.Query("show-disabled") == "1"
	ouFilter := c.Query("ou")

	var all []ldap.User
	if a.ldapCache != nil {
		all = a.ldapCache.FindUsers(showDisabled)
	}

	users := filterUsersByOU(all, ouFilter)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.UsersListV2(users, showDisabled, ouFilter, templates.Flashes()).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// handleUserV2 renders either the drawer fragment (?fragment=drawer) or the
// full user detail page at /users/:dn.
func (a *App) handleUserV2(c *fiber.Ctx) error {
	viewerDN := GetUserDN(c)
	if viewerDN == "" {
		sess, err := a.sessionStore.Get(c)
		if err != nil {
			return handle500(c, err)
		}
		viewerDN, _ = sess.Get("dn").(string)
	}

	if viewerDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	// Route is registered as /users/* — matches the legacy pattern so
	// existing tests keep working. `c.Params("*")` yields the URL-encoded
	// DN exactly as the client sent it.
	encodedDN := c.Params("*")

	userDN, err := url.PathUnescape(encodedDN)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	vm, ok := a.buildUserDrawerVM(userDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("user not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" {
		return templates.UserDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.UserFullV2(vm).
		Render(c.UserContext(), c.Response().BodyWriter())
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
