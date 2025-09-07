package web

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	ldappool "github.com/netresearch/ldap-manager/internal/ldap"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

func (a *App) usersHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	showDisabled := c.Query("show-disabled", "0") == "1"
	users := a.ldapCache.FindUsers(showDisabled)
	sort.SliceStable(users, func(i, j int) bool {
		return users[i].CN() < users[j].CN()
	})

	// Use template caching with query parameter differentiation
	return a.templateCache.RenderWithCache(c, templates.Users(users, showDisabled, templates.Flashes()))
}

func (a *App) userHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	userDN, err := url.PathUnescape(c.Params("userDN"))
	if err != nil {
		return handle500(c, err)
	}

	user, unassignedGroups, err := a.loadUserData(userDN)
	if err != nil {
		return handle500(c, err)
	}

	// Use template caching with user DN as additional cache data
	return a.templateCache.RenderWithCache(
		c,
		templates.User(user, unassignedGroups, templates.Flashes(), a.GetCSRFToken(c)),
		"userDN:"+userDN,
	)
}

type userModifyForm struct {
	AddGroup        *string `form:"addgroup"`
	RemoveGroup     *string `form:"removegroup"`
	PasswordConfirm string  `form:"password_confirm"`
}

// nolint:dupl // Similar to groupModifyHandler but operates on different entities with different forms
func (a *App) userModifyHandler(c *fiber.Ctx) error {
	// Authentication handled by middleware, no need to check session
	userDN, err := url.PathUnescape(c.Params("userDN"))
	if err != nil {
		return handle500(c, err)
	}

	form := userModifyForm{}
	if err := c.BodyParser(&form); err != nil {
		return handle500(c, err)
	}

	if form.RemoveGroup == nil && form.AddGroup == nil {
		return c.Redirect("/users/" + userDN)
	}

	// Require password confirmation for sensitive operations
	if form.PasswordConfirm == "" {
		return a.renderUserWithError(c, userDN, "Password confirmation required for modifications")
	}

	executorDN, err := RequireUserDN(c)
	if err != nil {
		return err
	}

	pooledClient, err := a.authenticateLDAPClient(c.UserContext(), executorDN, form.PasswordConfirm)
	if err != nil {
		return a.renderUserWithError(c, userDN, "Invalid password")
	}
	defer pooledClient.Close()

	// Perform the user modification
	if err := a.performUserModification(pooledClient, &form, userDN); err != nil {
		return a.renderUserWithError(c, userDN, "Failed to modify: "+err.Error())
	}

	// Invalidate template cache after successful modification
	a.invalidateTemplateCacheOnUserModification(userDN)

	// Render success response
	return a.renderUserWithSuccess(c, userDN, "Successfully modified user")
}

func (a *App) findUnassignedGroups(user *ldap_cache.FullLDAPUser) []ldap.Group {
	return a.ldapCache.Groups.Filter(func(g ldap.Group) bool {
		for _, ug := range user.Groups {
			if ug.DN() == g.DN() {
				return false
			}
		}

		return true
	})
}

// loadUserData loads and prepares user data with proper sorting
func (a *App) loadUserData(userDN string) (*ldap_cache.FullLDAPUser, []ldap.Group, error) {
	thinUser, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return nil, nil, err
	}

	user := a.ldapCache.PopulateGroupsForUser(thinUser)
	sort.SliceStable(user.Groups, func(i, j int) bool {
		return user.Groups[i].CN() < user.Groups[j].CN()
	})
	unassignedGroups := a.findUnassignedGroups(user)
	sort.SliceStable(unassignedGroups, func(i, j int) bool {
		return unassignedGroups[i].CN() < unassignedGroups[j].CN()
	})

	return user, unassignedGroups, nil
}

// renderUserWithError renders the user page with an error message
func (a *App) renderUserWithError(c *fiber.Ctx, userDN, errorMsg string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	user, unassignedGroups, err := a.loadUserData(userDN)
	if err != nil {
		return handle500(c, err)
	}

	return templates.User(
		user, unassignedGroups,
		templates.Flashes(templates.ErrorFlash(errorMsg)),
		a.GetCSRFToken(c),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// renderUserWithSuccess renders the user page with a success message
func (a *App) renderUserWithSuccess(c *fiber.Ctx, userDN, successMsg string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	user, unassignedGroups, err := a.loadUserData(userDN)
	if err != nil {
		return handle500(c, err)
	}

	return templates.User(
		user, unassignedGroups,
		templates.Flashes(templates.SuccessFlash(successMsg)),
		a.GetCSRFToken(c),
	).Render(c.UserContext(), c.Response().BodyWriter())
}

// performUserModification handles the actual LDAP user modification operation
func (a *App) performUserModification(pooledClient *ldappool.PooledLDAPClient, form *userModifyForm, userDN string) error {
	if form.AddGroup != nil {
		if err := pooledClient.AddUserToGroup(userDN, *form.AddGroup); err != nil {
			return err
		}
		a.ldapCache.OnAddUserToGroup(userDN, *form.AddGroup)
	} else if form.RemoveGroup != nil {
		if err := pooledClient.RemoveUserFromGroup(userDN, *form.RemoveGroup); err != nil {
			return err
		}
		a.ldapCache.OnRemoveUserFromGroup(userDN, *form.RemoveGroup)
	}

	return nil
}

// invalidateTemplateCacheOnUserModification invalidates relevant cache entries after user modification
func (a *App) invalidateTemplateCacheOnUserModification(userDN string) {
	// Invalidate the specific user page
	a.invalidateTemplateCache("/users/" + userDN)

	// Invalidate users list page (counts may have changed)
	a.invalidateTemplateCache("/users")

	// Invalidate groups pages (group membership may have changed)
	a.invalidateTemplateCache("/groups")

	// Clear all cache entries for safety (this could be optimized further)
	// In a high-traffic environment, you might want to be more selective
	a.templateCache.Clear()
}
