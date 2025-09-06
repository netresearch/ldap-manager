package web

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
	ldap "github.com/netresearch/simple-ldap-go"
)

func (a *App) usersHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	showDisabled := c.Query("show-disabled", "0") == "1"
	users := a.ldapCache.FindUsers(showDisabled)
	sort.SliceStable(users, func(i, j int) bool {
		return users[i].CN() < users[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.Users(users, showDisabled, templates.Flashes()).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) userHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	userDN, err := url.PathUnescape(c.Params("userDN"))
	if err != nil {
		return handle500(c, err)
	}

	thinUser, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	user := a.ldapCache.PopulateGroupsForUser(thinUser)
	sort.SliceStable(user.Groups, func(i, j int) bool {
		return user.Groups[i].CN() < user.Groups[j].CN()
	})
	unassignedGroups := a.findUnassignedGroups(user)
	sort.SliceStable(unassignedGroups, func(i, j int) bool {
		return unassignedGroups[i].CN() < unassignedGroups[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.User(user, unassignedGroups, templates.Flashes()).Render(c.UserContext(), c.Response().BodyWriter())
}

type userModifyForm struct {
	AddGroup         *string `form:"addgroup"`
	RemoveGroup      *string `form:"removegroup"`
	PasswordConfirm  string  `form:"password_confirm"`
}

func (a *App) userModifyHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

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
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		user, err := a.ldapCache.FindUserByDN(userDN)
		if err != nil {
			return handle500(c, err)
		}
		unassignedGroups, err := a.ldapCache.FindUnassignedGroupsForUser(*user)
		if err != nil {
			return handle500(c, err)
		}
		return templates.User(user, unassignedGroups, templates.Flashes(templates.ErrorFlash("Password confirmation required for modifications"))).Render(c.UserContext(), c.Response().BodyWriter())
	}

	executorDN := sess.Get("dn").(string)
	l, err := a.authenticateLDAPClient(executorDN, form.PasswordConfirm)
	if err != nil {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		targetUser, err := a.ldapCache.FindUserByDN(userDN)
		if err != nil {
			return handle500(c, err)
		}
		unassignedGroups, err := a.ldapCache.FindUnassignedGroupsForUser(*targetUser)
		if err != nil {
			return handle500(c, err)
		}
		return templates.User(targetUser, unassignedGroups, templates.Flashes(templates.ErrorFlash("Invalid password"))).Render(c.UserContext(), c.Response().BodyWriter())
	}

	thinUser, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	user := a.ldapCache.PopulateGroupsForUser(thinUser)
	sort.SliceStable(user.Groups, func(i, j int) bool {
		return user.Groups[i].CN() < user.Groups[j].CN()
	})
	unassignedGroups := a.findUnassignedGroups(user)
	sort.SliceStable(unassignedGroups, func(i, j int) bool {
		return unassignedGroups[i].CN() < unassignedGroups[j].CN()
	})

	if form.AddGroup != nil {
		if err := l.AddUserToGroup(userDN, *form.AddGroup); err != nil {
			return templates.User(
				user, unassignedGroups, templates.Flashes(
					templates.ErrorFlash("Failed to modify: "+err.Error()),
				),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		a.ldapCache.OnAddUserToGroup(userDN, *form.AddGroup)
	} else if form.RemoveGroup != nil {
		if err := l.RemoveUserFromGroup(userDN, *form.RemoveGroup); err != nil {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return templates.User(
				user, unassignedGroups, templates.Flashes(
					templates.ErrorFlash("Failed to modify: "+err.Error()),
				),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		a.ldapCache.OnRemoveUserFromGroup(userDN, *form.RemoveGroup)
	}

	thinUser, err = a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	user = a.ldapCache.PopulateGroupsForUser(thinUser)
	sort.SliceStable(user.Groups, func(i, j int) bool {
		return user.Groups[i].CN() < user.Groups[j].CN()
	})
	unassignedGroups = a.findUnassignedGroups(user)
	sort.SliceStable(unassignedGroups, func(i, j int) bool {
		return unassignedGroups[i].CN() < unassignedGroups[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.User(
		user, unassignedGroups, templates.Flashes(
			templates.SuccessFlash("Successfully modified user"),
		),
	).Render(c.UserContext(), c.Response().BodyWriter())
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
