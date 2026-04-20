// Package web — signed-in home page (spec §6.6).
package web

import (
	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// handleHomeV2 renders the signed-in home page (spec §6.6).
func (a *App) handleHomeV2(c *fiber.Ctx) error {
	// Prefer the DN already populated by RequireAuth into c.Locals —
	// re-reading the session after CSRF middleware can return a fresh
	// session on this code path and drop the "dn"/"username" keys.
	userDN := GetUserDN(c)
	username, _ := c.Locals("username").(string)
	if userDN == "" {
		sess, err := a.sessionStore.Get(c)
		if err != nil {
			return handle500(c, err)
		}
		userDN, _ = sess.Get("dn").(string)
		username, _ = sess.Get("username").(string)
	}
	if userDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	pinned, _ := a.pinnedEntriesFor(userDN)

	cn := username
	if u, ok := a.lookupUserByDN(userDN); ok {
		cn = u.CN()
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.HomeV2(cn, pinned).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// pinnedEntriesFor hydrates DN strings from PinnedStore into PinnedEntry
// values via ldap_cache lookups. Missing targets are silently dropped.
func (a *App) pinnedEntriesFor(userDN string) ([]templates.PinnedEntry, error) {
	dns, err := a.pinnedStore.List(userDN)
	if err != nil {
		return nil, err
	}
	out := make([]templates.PinnedEntry, 0, len(dns))
	for _, dn := range dns {
		if u, ok := a.lookupUserByDN(dn); ok {
			out = append(out, templates.PinnedEntry{Type: "user", DN: dn, CN: u.CN()})

			continue
		}
		if g, ok := a.lookupGroupByDN(dn); ok {
			out = append(out, templates.PinnedEntry{Type: "group", DN: dn, CN: g.CN()})

			continue
		}
		if cp, ok := a.lookupComputerByDN(dn); ok {
			out = append(out, templates.PinnedEntry{Type: "computer", DN: dn, CN: cp.CN()})

			continue
		}
	}

	return out, nil
}

// lookupUserByDN returns the cached user with the given DN, including
// disabled users so pinned items remain resolvable. When no service
// account is configured the cache is nil and this returns (zero, false).
func (a *App) lookupUserByDN(dn string) (ldap.User, bool) {
	if a.ldapCache == nil {
		return ldap.User{}, false
	}
	for _, u := range a.ldapCache.FindUsers(true) {
		if u.DN() == dn {
			return u, true
		}
	}

	return ldap.User{}, false
}

// lookupGroupByDN returns the cached group with the given DN.
func (a *App) lookupGroupByDN(dn string) (ldap.Group, bool) {
	if a.ldapCache == nil {
		return ldap.Group{}, false
	}
	for _, g := range a.ldapCache.FindGroups() {
		if g.DN() == dn {
			return g, true
		}
	}

	return ldap.Group{}, false
}

// lookupComputerByDN returns the cached computer with the given DN,
// including disabled computers for the same reason as lookupUserByDN.
func (a *App) lookupComputerByDN(dn string) (ldap.Computer, bool) {
	if a.ldapCache == nil {
		return ldap.Computer{}, false
	}
	for _, cp := range a.ldapCache.FindComputers(true) {
		if cp.DN() == dn {
			return cp, true
		}
	}

	return ldap.Computer{}, false
}
