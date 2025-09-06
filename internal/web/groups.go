package web

import (
	"net/url"
	"sort"

	"github.com/gofiber/fiber/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
	ldap "github.com/netresearch/simple-ldap-go"
)

func (a *App) groupsHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

	groups := a.ldapCache.FindGroups()
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].CN() < groups[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.Groups(groups).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) groupHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

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

func (a *App) groupModifyHandler(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	if sess.Fresh() {
		return c.Redirect("/login")
	}

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
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		group, err := a.ldapCache.FindGroupByDN(groupDN)
		if err != nil {
			return handle500(c, err)
		}
		unassignedUsers, err := a.ldapCache.FindUnassignedUsersForGroup(*group)
		if err != nil {
			return handle500(c, err)
		}
		return templates.Group(group, unassignedUsers, templates.Flashes(templates.ErrorFlash("Password confirmation required for modifications"))).Render(c.UserContext(), c.Response().BodyWriter())
	}

	executorDN := sess.Get("dn").(string)
	l, err := a.authenticateLDAPClient(executorDN, form.PasswordConfirm)
	if err != nil {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		group, err := a.ldapCache.FindGroupByDN(groupDN)
		if err != nil {
			return handle500(c, err)
		}
		unassignedUsers, err := a.ldapCache.FindUnassignedUsersForGroup(*group)
		if err != nil {
			return handle500(c, err)
		}
		return templates.Group(group, unassignedUsers, templates.Flashes(templates.ErrorFlash("Invalid password"))).Render(c.UserContext(), c.Response().BodyWriter())
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

	if form.AddUser != nil {
		if err := l.AddUserToGroup(*form.AddUser, thinGroup.DN()); err != nil {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return templates.Group(
				group, unassignedUsers, templates.Flashes(
					templates.ErrorFlash("Failed to modify: "+err.Error()),
				),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		a.ldapCache.OnAddUserToGroup(*form.AddUser, thinGroup.DN())
	} else if form.RemoveUser != nil {
		if err := l.RemoveUserFromGroup(*form.RemoveUser, thinGroup.DN()); err != nil {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return templates.Group(
				group, unassignedUsers, templates.Flashes(
					templates.ErrorFlash("Failed to modify: "+err.Error()),
				),
			).Render(c.UserContext(), c.Response().BodyWriter())
		}

		a.ldapCache.OnRemoveUserFromGroup(*form.RemoveUser, thinGroup.DN())
	}

	thinGroup, err = a.ldapCache.FindGroupByDN(groupDN)
	if err != nil {
		return handle500(c, err)
	}

	group = a.ldapCache.PopulateUsersForGroup(thinGroup, showDisabledUsers)
	sort.SliceStable(group.Members, func(i, j int) bool {
		return group.Members[i].CN() < group.Members[j].CN()
	})
	unassignedUsers = a.findUnassignedUsers(group)
	sort.SliceStable(unassignedUsers, func(i, j int) bool {
		return unassignedUsers[i].CN() < unassignedUsers[j].CN()
	})

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.Group(
		group, unassignedUsers, templates.Flashes(templates.SuccessFlash("Successfully modified group")),
	).Render(c.UserContext(), c.Response().BodyWriter())
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
