package web

import (
	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

// isAdmin reports whether the given DN belongs to an administrator.
//
// A viewer is an admin when they carry AD's adminCount=1 marker, or when they
// are a member of the configured admin group. The group covers OpenLDAP, which
// has no adminCount, so a deployment there must set --admin-group to grant
// anyone access; without it, isAdmin is false for everyone on OpenLDAP.
//
// It reads the viewer from the background cache. When no service account is
// configured the cache is nil and there is no way to resolve membership, so
// the answer is false — the roster needs the service account anyway.
func (a *App) isAdmin(userDN string) bool {
	if a.ldapCache == nil {
		return false
	}

	user, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil || user == nil {
		log.Debug().Str("userDN", userDN).Msg("admin check: viewer not found in cache")

		return false
	}

	return a.userIsAdmin(user)
}

// userIsAdmin is the pure admin decision for a resolved user: AD's adminCount
// marker, or membership of the configured admin group. Split from the cache
// lookup so the rule is testable without constructing a DN-addressable cache
// entry (User.DN is not settable outside the LDAP package).
func (a *App) userIsAdmin(user *ldap.User) bool {
	if user.AdminCount {
		return true
	}

	if a.adminGroupDN != "" && user.IsMemberOf(a.adminGroupDN) {
		return true
	}

	return false
}

// resolveAdminCheck returns the admin predicate: the injected one in tests,
// otherwise the real cache-backed isAdmin.
func (a *App) resolveAdminCheck() func(string) bool {
	if a.adminCheck != nil {
		return a.adminCheck
	}

	return a.isAdmin
}

// RequireAdmin gates a route to administrators. It must sit behind RequireAuth,
// which populates the viewer DN into c.Locals. A non-admin gets 403 rather than
// a redirect: they are authenticated, just not permitted.
func (a *App) RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userDN := GetUserDN(c)
		if userDN == "" || !a.resolveAdminCheck()(userDN) {
			log.Warn().
				Str("userDN", userDN).
				Str("path", c.Path()).
				Msg("non-admin access attempt to an admin-only route")

			return c.Status(fiber.StatusForbidden).SendString("Forbidden: administrators only")
		}

		return c.Next()
	}
}
