package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
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

	return c.Render("views/groups", fiber.Map{
		"session":     sess,
		"title":       "All groups",
		"activePage":  "/groups",
		"headscripts": "",
		"flashes":     []Flash{},
		"groups":      groups,
	}, "layouts/logged-in")
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
	unassignedUsers := a.findUnassignedUsers(group)

	return c.Render("views/group", fiber.Map{
		"session":         sess,
		"title":           group.CN(),
		"activePage":      "/groups",
		"headscripts":     "",
		"flashes":         []Flash{},
		"group":           group,
		"unassignedUsers": unassignedUsers,
	}, "layouts/logged-in")
}

type groupModifyForm struct {
	AddUser    *string `form:"adduser"`
	RemoveUser *string `form:"removeuser"`
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

	l, err := a.sessionToLDAPClient(sess)
	if err != nil {
		return handle500(c, err)
	}

	thinGroup, err := a.ldapCache.FindGroupByDN(groupDN)
	if err != nil {
		return handle500(c, err)
	}

	showDisabledUsers := c.Query("show-disabled", "0") == "1"
	group := a.ldapCache.PopulateUsersForGroup(thinGroup, showDisabledUsers)
	unassignedUsers := a.findUnassignedUsers(group)

	if form.AddUser != nil {
		if err := l.AddUserToGroup(*form.AddUser, thinGroup.DN()); err != nil {
			return c.Render("views/group", fiber.Map{
				"session":     sess,
				"title":       group.CN(),
				"activePage":  "/groups",
				"headscripts": "",
				// TODO: properly translate error
				"flashes":         []Flash{NewFlash(FlashTypeError, "Failed to modify: "+err.Error())},
				"group":           group,
				"unassignedUsers": unassignedUsers,
			}, "layouts/logged-in")
		}

		a.ldapCache.OnAddUserToGroup(*form.AddUser, thinGroup.DN())
	} else if form.RemoveUser != nil {
		if err := l.RemoveUserFromGroup(*form.RemoveUser, thinGroup.DN()); err != nil {
			return c.Render("views/group", fiber.Map{
				"session":         sess,
				"title":           group.CN(),
				"activePage":      "/groups",
				"headscripts":     "",
				"flashes":         []Flash{NewFlash(FlashTypeError, "Failed to modify: "+err.Error())},
				"group":           group,
				"unassignedUsers": unassignedUsers,
			}, "layouts/logged-in")
		}

		a.ldapCache.OnRemoveUserFromGroup(*form.RemoveUser, thinGroup.DN())
	}

	thinGroup, err = a.ldapCache.FindGroupByDN(groupDN)
	if err != nil {
		return handle500(c, err)
	}

	group = a.ldapCache.PopulateUsersForGroup(thinGroup, showDisabledUsers)
	unassignedUsers = a.findUnassignedUsers(group)

	return c.Render("views/group", fiber.Map{
		"session":         sess,
		"title":           group.CN(),
		"activePage":      "/groups",
		"headscripts":     "",
		"flashes":         []Flash{NewFlash(FlashTypeSuccess, "Successfully modified group")},
		"group":           group,
		"unassignedUsers": unassignedUsers,
	}, "layouts/logged-in")
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
