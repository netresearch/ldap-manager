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

	// Use template caching
	return a.templateCache.RenderWithCache(c, templates.Groups(groups))
}

func (a *App) groupHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	groupDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	group, unassignedUsers, err := a.loadGroupData(c, groupDN)
	if err != nil {
		return handle500(c, err)
	}

	// Use template caching with group DN as additional cache data
	return a.templateCache.RenderWithCache(
		c,
		templates.Group(group, unassignedUsers, templates.Flashes(), a.GetCSRFToken(c)),
		"groupDN:"+groupDN,
	)
}

type groupModifyForm struct {
	AddUser    *string `form:"adduser"`
	RemoveUser *string `form:"removeuser"`
}

// nolint:dupl // Similar to userModifyHandler but operates on different entities with different forms
func (a *App) groupModifyHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	groupDN, err := url.PathUnescape(c.Params("*"))
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

	// Perform the group modification using the readonly LDAP client
	// User is already authenticated via session middleware
	if err := a.performGroupModification(a.ldapReadonly, &form, groupDN); err != nil {
		return a.renderGroupWithError(c, groupDN, "Failed to modify: "+err.Error())
	}

	// Invalidate template cache after successful modification
	a.invalidateTemplateCacheOnGroupModification(groupDN)

	// Render success response
	return a.renderGroupWithSuccess(c, groupDN, "Successfully modified group")
}

func (a *App) findUnassignedUsers(group *ldap_cache.FullLDAPGroup) []ldap.User {
	return a.ldapCache.Users.Filter(func(u ldap.User) bool {
		// Check if user is already a member of this group
		for _, member := range group.Members {
			if member.DN() == u.DN() {
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
		a.GetCSRFToken(c),
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
		a.GetCSRFToken(c),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// performGroupModification handles the actual LDAP group modification operation
func (a *App) performGroupModification(
	ldapClient *ldap.LDAP, form *groupModifyForm, groupDN string,
) error {
	if form.AddUser != nil {
		if err := ldapClient.AddUserToGroup(*form.AddUser, groupDN); err != nil {
			return err
		}
		a.ldapCache.OnAddUserToGroup(*form.AddUser, groupDN)
	} else if form.RemoveUser != nil {
		if err := ldapClient.RemoveUserFromGroup(*form.RemoveUser, groupDN); err != nil {
			return err
		}
		a.ldapCache.OnRemoveUserFromGroup(*form.RemoveUser, groupDN)
	}

	return nil
}

// invalidateTemplateCacheOnGroupModification invalidates relevant cache entries after group modification
func (a *App) invalidateTemplateCacheOnGroupModification(groupDN string) {
	// Invalidate the specific group page
	a.invalidateTemplateCache("/groups/" + groupDN)

	// Invalidate groups list page (counts may have changed)
	a.invalidateTemplateCache("/groups")

	// Invalidate users pages (user membership may have changed)
	a.invalidateTemplateCache("/users")

	// Clear all cache entries for safety (this could be optimized further)
	// In a high-traffic environment, you might want to be more selective
	a.templateCache.Clear()
}
