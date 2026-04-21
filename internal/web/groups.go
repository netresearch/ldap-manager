package web

// HTTP handlers for group modification endpoints. The read-only list and
// detail pages are served by the V2 handlers in groups_v2_handler.go. This
// file retains only the POST handler and its helpers.

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

type groupModifyForm struct {
	AddUser    *string `form:"adduser"`
	RemoveUser *string `form:"removeuser"`
}

// groupModifyHandler applies an add/remove-user action and redirects back to
// the V2 group detail page. Flash messages from the legacy V1 template have
// been dropped; failures are logged and surfaced via the server log.
//
//nolint:dupl // Similar to userModifyHandler but operates on different entities with different forms
func (a *App) groupModifyHandler(c *fiber.Ctx) error {
	groupDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	detailURL := "/groups/" + url.PathEscape(groupDN)

	form := groupModifyForm{}
	if err := c.BodyParser(&form); err != nil {
		return handle500(c, err)
	}

	if form.RemoveUser == nil && form.AddUser == nil {
		return c.Redirect(detailURL)
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = userLDAP.Close() }()

	if err := a.performGroupModification(userLDAP, &form, groupDN); err != nil {
		log.Warn().Err(err).Str("groupDN", groupDN).Msg("failed to modify group")
	} else {
		a.invalidateTemplateCacheOnModification()
	}

	if c.Get("HX-Request") == "true" {
		viewerDN := GetUserDN(c)

		vm, ok := a.buildGroupDrawerVM(groupDN, viewerDN)
		if !ok {
			return c.Redirect(detailURL)
		}

		vm.CSRFToken = a.GetCSRFToken(c)

		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

		return templates.GroupDrawerFragment(vm).Render(c.UserContext(), c.Response().BodyWriter())
	}

	return c.Redirect(detailURL)
}

// filterUnassignedUsers returns users not in the given group.
func filterUnassignedUsers(allUsers []ldap.User, group *ldap_cache.FullLDAPGroup) []ldap.User {
	memberDNS := make(map[string]struct{}, len(group.Members))
	for _, member := range group.Members {
		memberDNS[member.DN()] = struct{}{}
	}

	result := make([]ldap.User, 0)

	for _, u := range allUsers {
		if _, isMember := memberDNS[u.DN()]; !isMember {
			result = append(result, u)
		}
	}

	return result
}

// findGroupByDN searches for a group by DN in a slice.
func findGroupByDN(groups []ldap.Group, dn string) *ldap.Group {
	for i := range groups {
		if groups[i].DN() == dn {
			return &groups[i]
		}
	}

	return nil
}

// performGroupModification handles the actual LDAP group modification operation.
func (a *App) performGroupModification(
	ldapClient *ldap.LDAP, form *groupModifyForm, groupDN string,
) error {
	if form.AddUser != nil {
		if err := ldapClient.AddUserToGroup(*form.AddUser, groupDN); err != nil {
			return err
		}

		if a.ldapCache != nil {
			a.ldapCache.OnAddUserToGroup(*form.AddUser, groupDN)
		}
	} else if form.RemoveUser != nil {
		if err := ldapClient.RemoveUserFromGroup(*form.RemoveUser, groupDN); err != nil {
			return err
		}

		if a.ldapCache != nil {
			a.ldapCache.OnRemoveUserFromGroup(*form.RemoveUser, groupDN)
		}
	}

	return nil
}
