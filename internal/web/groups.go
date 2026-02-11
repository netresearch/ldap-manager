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
	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer userLDAP.Close()

	groups, err := userLDAP.FindGroups()
	if err != nil {
		return handle500(c, err)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].CN() < groups[j].CN()
	})

	// Use template caching
	return a.templateCache.RenderWithCache(c, templates.Groups(groups))
}

func (a *App) groupHandler(c *fiber.Ctx) error {
	groupDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return handle500(c, err)
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer userLDAP.Close()

	group, unassignedUsers, err := a.loadGroupDataFromLDAP(c, userLDAP, groupDN)
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

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer userLDAP.Close()

	// Perform the group modification using the logged-in user's LDAP connection
	if err := a.performGroupModification(userLDAP, &form, groupDN); err != nil {
		return a.renderGroupWithFlash(c, userLDAP, groupDN, templates.ErrorFlash("Failed to modify: "+err.Error()))
	}

	// Invalidate template cache after successful modification
	a.invalidateTemplateCacheOnGroupModification(groupDN)

	// Render success response
	return a.renderGroupWithFlash(c, userLDAP, groupDN, templates.SuccessFlash("Successfully modified group"))
}

// loadGroupDataFromLDAP loads group data directly from an LDAP client connection.
func (a *App) loadGroupDataFromLDAP(c *fiber.Ctx, userLDAP *ldap.LDAP, groupDN string) (*ldap_cache.FullLDAPGroup, []ldap.User, error) {
	groups, err := userLDAP.FindGroups()
	if err != nil {
		return nil, nil, err
	}

	group := findGroupByDN(groups, groupDN)
	if group == nil {
		return nil, nil, ldap.ErrGroupNotFound
	}

	users, err := userLDAP.FindUsers()
	if err != nil {
		return nil, nil, err
	}

	showDisabledUsers := c.Query("show-disabled", "0") == "1"
	fullGroup := ldap_cache.PopulateUsersForGroupFromData(group, users, groups, showDisabledUsers)

	sort.SliceStable(fullGroup.Members, func(i, j int) bool {
		return fullGroup.Members[i].CN() < fullGroup.Members[j].CN()
	})

	unassignedUsers := filterUnassignedUsers(users, fullGroup)
	sort.SliceStable(unassignedUsers, func(i, j int) bool {
		return unassignedUsers[i].CN() < unassignedUsers[j].CN()
	})

	return fullGroup, unassignedUsers, nil
}

// renderGroupWithFlash renders the group page with a flash message using a user LDAP connection.
func (a *App) renderGroupWithFlash(c *fiber.Ctx, userLDAP *ldap.LDAP, groupDN string, flash templates.Flash) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	group, unassignedUsers, err := a.loadGroupDataFromLDAP(c, userLDAP, groupDN)
	if err != nil {
		return handle500(c, err)
	}

	return templates.Group(
		group, unassignedUsers,
		templates.Flashes(flash),
		a.GetCSRFToken(c),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// filterUnassignedUsers returns users not in the given group.
func filterUnassignedUsers(allUsers []ldap.User, group *ldap_cache.FullLDAPGroup) []ldap.User {
	memberDNs := make(map[string]struct{}, len(group.Members))
	for _, member := range group.Members {
		memberDNs[member.DN()] = struct{}{}
	}

	result := make([]ldap.User, 0)

	for _, u := range allUsers {
		if _, isMember := memberDNs[u.DN()]; !isMember {
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

// invalidateTemplateCacheOnGroupModification invalidates relevant cache entries after group modification
func (a *App) invalidateTemplateCacheOnGroupModification(groupDN string) {
	// Invalidate the specific group page
	a.invalidateTemplateCache("/groups/" + groupDN)

	// Invalidate groups list page (counts may have changed)
	a.invalidateTemplateCache("/groups")

	// Invalidate users pages (user membership may have changed)
	a.invalidateTemplateCache("/users")

	// Clear all cache entries for safety (this could be optimized further)
	a.templateCache.Clear()
}
