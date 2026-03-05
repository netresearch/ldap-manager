package web

// HTTP handlers for user management endpoints.

import (
	"errors"
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) usersHandler(c *fiber.Ctx) error {
	showDisabled := c.Query("show-disabled", "0") == "1"

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = userLDAP.Close() }()

	allUsers, err := userLDAP.FindUsers()
	if err != nil {
		return handle500(c, err)
	}

	var users []ldap.User
	if !showDisabled {
		for _, u := range allUsers {
			if u.Enabled {
				users = append(users, u)
			}
		}
	} else {
		users = allUsers
	}

	sort.SliceStable(users, func(i, j int) bool {
		return users[i].CN() < users[j].CN()
	})

	// Use template caching with query parameter differentiation
	return a.templateCache.RenderWithCache(c, templates.Users(users, showDisabled, templates.Flashes()))
}

func (a *App) userHandler(c *fiber.Ctx) error {
	userDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = userLDAP.Close() }()

	user, unassignedGroups, err := a.loadUserDataFromLDAP(userLDAP, userDN)
	if err != nil {
		if errors.Is(err, ldap.ErrUserNotFound) {
			c.Status(fiber.StatusNotFound)

			return a.fourOhFourHandler(c)
		}

		return handle500(c, err)
	}

	// Detail pages with CSRF tokens are not cached to avoid stale token issues
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.User(user, unassignedGroups, templates.Flashes(), a.GetCSRFToken(c)).
		Render(c.UserContext(), c.Response().BodyWriter())
}

type userModifyForm struct {
	AddGroup    *string `form:"addgroup"`
	RemoveGroup *string `form:"removegroup"`
}

// nolint:dupl // Similar to groupModifyHandler but operates on different entities with different forms
func (a *App) userModifyHandler(c *fiber.Ctx) error {
	userDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	form := userModifyForm{}
	if err := c.BodyParser(&form); err != nil {
		return handle500(c, err)
	}

	if form.RemoveGroup == nil && form.AddGroup == nil {
		return c.Redirect("/users/" + url.PathEscape(userDN))
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = userLDAP.Close() }()

	// Perform the user modification using the logged-in user's LDAP connection
	if err := a.performUserModification(userLDAP, &form, userDN); err != nil {
		log.Warn().Err(err).Str("userDN", userDN).Msg("failed to modify user")

		return a.renderUserWithFlash(c, userLDAP, userDN, templates.ErrorFlash("Failed to modify user membership"))
	}

	// Invalidate template cache after successful modification
	a.invalidateTemplateCacheOnModification()

	// Render success response
	return a.renderUserWithFlash(c, userLDAP, userDN, templates.SuccessFlash("Successfully modified user"))
}

// loadUserDataFromLDAP loads user data directly from an LDAP client connection.
func (a *App) loadUserDataFromLDAP(userLDAP *ldap.LDAP, userDN string) (*ldap_cache.FullLDAPUser, []ldap.Group, error) {
	allUsers, err := userLDAP.FindUsers()
	if err != nil {
		return nil, nil, err
	}

	user, err := findUserByDN(allUsers, userDN)
	if err != nil {
		return nil, nil, err
	}

	groups, err := userLDAP.FindGroups()
	if err != nil {
		return nil, nil, err
	}

	fullUser := ldap_cache.PopulateGroupsForUserFromData(user, groups)
	sort.SliceStable(fullUser.Groups, func(i, j int) bool {
		return fullUser.Groups[i].CN() < fullUser.Groups[j].CN()
	})

	unassignedGroups := filterUnassignedGroups(groups, fullUser)
	sort.SliceStable(unassignedGroups, func(i, j int) bool {
		return unassignedGroups[i].CN() < unassignedGroups[j].CN()
	})

	return fullUser, unassignedGroups, nil
}

// renderUserWithFlash renders the user page with a flash message using a user LDAP connection.
func (a *App) renderUserWithFlash(c *fiber.Ctx, userLDAP *ldap.LDAP, userDN string, flash templates.Flash) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	user, unassignedGroups, err := a.loadUserDataFromLDAP(userLDAP, userDN)
	if err != nil {
		return handle500(c, err)
	}

	return templates.User(
		user, unassignedGroups,
		templates.Flashes(flash),
		a.GetCSRFToken(c),
	).Render(c.UserContext(), c.Response().BodyWriter())
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
