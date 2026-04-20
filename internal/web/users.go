package web

// HTTP handlers for user modification endpoints. The read-only list and
// detail pages are served by the V2 handlers in users_v2_handler.go. This
// file retains only the POST handler and its helpers, which invalidate the
// template cache after an LDAP membership change and redirect the user to
// the V2 detail page.

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

type userModifyForm struct {
	AddGroup    *string `form:"addgroup"`
	RemoveGroup *string `form:"removegroup"`
}

// userModifyHandler applies an add/remove-group action and redirects back to
// the V2 user detail page. Flash messages from the legacy V1 template have
// been dropped; failures are logged and surfaced via the server log.
//
//nolint:dupl // Similar to groupModifyHandler but operates on different entities with different forms
func (a *App) userModifyHandler(c *fiber.Ctx) error {
	userDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	detailURL := "/users/" + url.PathEscape(userDN)

	form := userModifyForm{}
	if err := c.BodyParser(&form); err != nil {
		return handle500(c, err)
	}

	if form.RemoveGroup == nil && form.AddGroup == nil {
		return c.Redirect(detailURL)
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = userLDAP.Close() }()

	if err := a.performUserModification(userLDAP, &form, userDN); err != nil {
		log.Warn().Err(err).Str("userDN", userDN).Msg("failed to modify user")
	} else {
		a.invalidateTemplateCacheOnModification()
	}

	return c.Redirect(detailURL)
}

// filterUnassignedGroups returns groups the user is not a member of.
func filterUnassignedGroups(allGroups []ldap.Group, user *ldap_cache.FullLDAPUser) []ldap.Group {
	memberGroupDNS := make(map[string]struct{}, len(user.Groups))
	for _, g := range user.Groups {
		memberGroupDNS[g.DN()] = struct{}{}
	}

	result := make([]ldap.Group, 0)

	for _, g := range allGroups {
		if _, isMember := memberGroupDNS[g.DN()]; !isMember {
			result = append(result, g)
		}
	}

	return result
}

// findUserByDN searches for a user by DN in a slice.
func findUserByDN(users []ldap.User, dn string) (*ldap.User, error) {
	for i := range users {
		if users[i].DN() == dn {
			return &users[i], nil
		}
	}

	return nil, ldap.ErrUserNotFound
}

// performUserModification handles the actual LDAP user modification operation.
func (a *App) performUserModification(
	ldapClient *ldap.LDAP, form *userModifyForm, userDN string,
) error {
	if form.AddGroup != nil {
		if err := ldapClient.AddUserToGroup(userDN, *form.AddGroup); err != nil {
			return err
		}

		if a.ldapCache != nil {
			a.ldapCache.OnAddUserToGroup(userDN, *form.AddGroup)
		}
	} else if form.RemoveGroup != nil {
		if err := ldapClient.RemoveUserFromGroup(userDN, *form.RemoveGroup); err != nil {
			return err
		}

		if a.ldapCache != nil {
			a.ldapCache.OnRemoveUserFromGroup(userDN, *form.RemoveGroup)
		}
	}

	return nil
}

// invalidateTemplateCacheOnModification clears the template cache after any modification.
// Membership changes can affect multiple pages, so we clear the entire cache.
func (a *App) invalidateTemplateCacheOnModification() {
	a.templateCache.Clear()
	log.Debug().Msg("Template cache cleared after modification")
}
