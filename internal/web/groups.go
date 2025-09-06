package web

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) groupsHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	groups := a.ldapCache.FindGroups()
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].CN() < groups[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.Groups(groups).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) groupHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	groupDN, err := url.PathUnescape(c.Params("groupDN"))
	if err != nil {
		return handle500(c, err)
	}

	thinGroup, err := a.ldapCache.FindGroupByDN(groupDN)
	if err != nil {
		return handle500(c, err)
	}

	showDisabledUsers := c.Query("show-disabled", "0") == "1"
	group := a.ldapCache.PopulateUsersForGroup(thinGroup, showDisabledUsers)
	sort.SliceStable(group.Members, func(i, j int) bool {
		return group.Members[i].CN() < group.Members[j].CN()
	})
	unassignedUsers := a.findUnassignedUsers(group)
	sort.SliceStable(unassignedUsers, func(i, j int) bool {
		return unassignedUsers[i].CN() < unassignedUsers[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.Group(group, unassignedUsers, templates.Flashes()).Render(c.UserContext(), c.Response().BodyWriter())
}

type groupModifyForm struct {
	AddUser         *string `form:"adduser"`
	RemoveUser      *string `form:"removeuser"`
	PasswordConfirm string  `form:"password_confirm"`
}

// nolint:dupl // Similar to userModifyHandler but operates on different entities with different forms
func (a *App) groupModifyHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	groupDN, err := url.PathUnescape(c.Params("groupDN"))
	if err != nil {
		return handle500(c, err)
	}

	form := groupModifyForm{}
	if err := c.BodyParser(&form); err != nil {
		return handle500(c, err)
	}

	if form.RemoveUser == nil && form.AddUser == nil {
		return c.Redirect("/groups/" + groupDN)
	}

	// Require password confirmation for sensitive operations
	if form.PasswordConfirm == "" {
		return a.renderGroupWithError(c, groupDN, "Password confirmation required for modifications")
	}

	executorDN, err := RequireUserDN(c)
	if err != nil {
		return err
	}
	l, err := a.authenticateLDAPClient(executorDN, form.PasswordConfirm)
	if err != nil {
		return a.renderGroupWithError(c, groupDN, "Invalid password")
	}

	// Perform the group modification
	if err := a.performGroupModification(l, &form, groupDN); err != nil {
		return a.renderGroupWithError(c, groupDN, "Failed to modify: "+err.Error())
	}

	// Render success response
	return a.renderGroupWithSuccess(c, groupDN, "Successfully modified group")
}

func (a *App) findUnassignedUsers(group *ldap_cache.FullLDAPGroup) []ldap.User {
	return a.ldapCache.Users.Filter(func(u ldap.User) bool {
		for _, g := range u.Groups {
			if g == group.DN() {
				return false
			}
		}

		return true
	})
}

// loadGroupData loads and prepares group data with proper sorting
func (a *App) loadGroupData(c *fiber.Ctx, groupDN string) (*ldap_cache.FullLDAPGroup, []ldap.User, error) {
	thinGroup, err := a.ldapCache.FindGroupByDN(groupDN)
	if err != nil {
		return nil, nil, err
	}

	showDisabledUsers := c.Query("show-disabled", "0") == "1"
	group := a.ldapCache.PopulateUsersForGroup(thinGroup, showDisabledUsers)
	sort.SliceStable(group.Members, func(i, j int) bool {
		return group.Members[i].CN() < group.Members[j].CN()
	})
	unassignedUsers := a.findUnassignedUsers(group)
	sort.SliceStable(unassignedUsers, func(i, j int) bool {
		return unassignedUsers[i].CN() < unassignedUsers[j].CN()
	})

	return group, unassignedUsers, nil
}

// renderGroupWithError renders the group page with an error message
func (a *App) renderGroupWithError(c *fiber.Ctx, groupDN, errorMsg string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	group, unassignedUsers, err := a.loadGroupData(c, groupDN)
	if err != nil {
		return handle500(c, err)
	}

	return templates.Group(
		group, unassignedUsers,
		templates.Flashes(templates.ErrorFlash(errorMsg)),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// renderGroupWithSuccess renders the group page with a success message
func (a *App) renderGroupWithSuccess(c *fiber.Ctx, groupDN, successMsg string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	group, unassignedUsers, err := a.loadGroupData(c, groupDN)
	if err != nil {
		return handle500(c, err)
	}

	return templates.Group(
		group, unassignedUsers,
		templates.Flashes(templates.SuccessFlash(successMsg)),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// performGroupModification handles the actual LDAP group modification operation
func (a *App) performGroupModification(l *ldap.LDAP, form *groupModifyForm, groupDN string) error {
	if form.AddUser != nil {
		if err := l.AddUserToGroup(*form.AddUser, groupDN); err != nil {
			return err
		}
		a.ldapCache.OnAddUserToGroup(*form.AddUser, groupDN)
	} else if form.RemoveUser != nil {
		if err := l.RemoveUserFromGroup(*form.RemoveUser, groupDN); err != nil {
			return err
		}
		a.ldapCache.OnRemoveUserFromGroup(*form.RemoveUser, groupDN)
	}

	return nil
}
