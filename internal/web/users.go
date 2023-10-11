package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
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

	return c.Render("views/users", fiber.Map{
		"session":      sess,
		"title":        "All users",
		"activePage":   "/users",
		"headscripts":  "",
		"flashes":      []Flash{},
		"users":        users,
		"showDisabled": showDisabled,
	}, "layouts/logged-in")
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

	unassignedGroups := a.findUnassignedGroups(user)

	return c.Render("views/user", fiber.Map{
		"session":          sess,
		"title":            user.CN(),
		"activePage":       "/users",
		"headscripts":      "",
		"flashes":          []Flash{},
		"user":             user,
		"unassignedGroups": unassignedGroups,
	}, "layouts/logged-in")
}

type userModifyForm struct {
	AddGroup    *string `form:"addgroup"`
	RemoveGroup *string `form:"removegroup"`
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

	executor, err := a.ldapCache.FindUserByDN(sess.Get("dn").(string))
	if err != nil {
		return handle500(c, err)
	}

	l, err := a.ldap.WithCredentials(executor.DN(), sess.Get("password").(string))
	if err != nil {
		return handle500(c, err)
	}

	thinUser, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	user := a.ldapCache.PopulateGroupsForUser(thinUser)
	unassignedGroups := a.findUnassignedGroups(user)

	if form.AddGroup != nil {
		if err := l.AddUserToGroup(userDN, *form.AddGroup); err != nil {
			return c.Render("views/user", fiber.Map{
				"session":     sess,
				"title":       user.CN(),
				"activePage":  "/users",
				"headscripts": "",
				// TODO: properly translate error
				"flashes":          []Flash{NewFlash(FlashTypeError, "Failed to modify: "+err.Error())},
				"user":             user,
				"unassignedGroups": unassignedGroups,
			}, "layouts/logged-in")
		}

		a.ldapCache.OnAddUserToGroup(userDN, *form.AddGroup)
	} else if form.RemoveGroup != nil {
		if err := l.RemoveUserFromGroup(userDN, *form.RemoveGroup); err != nil {
			return c.Render("views/user", fiber.Map{
				"session":          sess,
				"title":            user.CN(),
				"activePage":       "/users",
				"headscripts":      "",
				"flashes":          []Flash{NewFlash(FlashTypeError, "Failed to modify: "+err.Error())},
				"user":             user,
				"unassignedGroups": unassignedGroups,
			}, "layouts/logged-in")
		}

		a.ldapCache.OnRemoveUserFromGroup(userDN, *form.RemoveGroup)
	}

	thinUser, err = a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	user = a.ldapCache.PopulateGroupsForUser(thinUser)
	unassignedGroups = a.findUnassignedGroups(user)

	return c.Render("views/user", fiber.Map{
		"session":          sess,
		"title":            user.CN(),
		"activePage":       "/users",
		"headscripts":      "",
		"flashes":          []Flash{NewFlash(FlashTypeSuccess, "Successfully modified user")},
		"user":             user,
		"unassignedGroups": unassignedGroups,
	}, "layouts/logged-in")
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
