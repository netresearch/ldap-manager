// internal/web/users_edit.go — inline attribute edit for users (Phase 2).
package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// userInlineEditFields is the whitelist of attributes that can be edited via
// the drawer inline-edit forms. The map key is the form field name and the
// value is the LDAP attribute name.
var userInlineEditFields = map[string]string{
	"email":       "mail",
	"description": "description",
}

// handleUserV2Edit applies a single-field inline edit and re-renders the
// drawer fragment. Triggered by the htmx POST from the drawer `kv-edit`
// forms. Whitelisted fields only (see userInlineEditFields).
//
// Form payload:
//
//	field=email        value=<new>
//	field=description  value=<new>
//
// Registered alongside the existing POST /users/* route — this handler
// dispatches based on the presence of a `field` form value so we do not
// need a new top-level route or URL scheme.
func (a *App) handleUserV2Edit(c *fiber.Ctx, userDN, field, value string) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	attr, ok := userInlineEditFields[field]
	if !ok {
		return c.Status(fiber.StatusBadRequest).SendString("field not editable")
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	if err := modifyUserAttribute(client, userDN, attr, value); err != nil {
		log.Warn().Err(err).Str("user", userDN).Str("field", field).Msg("inline edit failed")

		return handle500(c, err)
	}

	a.invalidateTemplateCacheOnModification()

	// Refresh the users cache so the freshly rendered drawer reflects the new
	// value. Best-effort: on failure we fall through to render the drawer
	// with the pre-change cached user; the user can reload to see the value.
	if a.ldapCache != nil {
		if refreshErr := a.ldapCache.RefreshUsers(); refreshErr != nil {
			log.Warn().Err(refreshErr).Str("user", userDN).Msg("post-edit user cache refresh failed")
		}
	}

	vm, ok := a.buildUserDrawerVM(userDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("user not found")
	}

	vm.CSRFToken = a.GetCSRFToken(c)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	// When htmx requested the edit we return the fresh drawer fragment. For a
	// non-htmx fallback we redirect back to the detail page.
	if c.Get("HX-Request") == "true" {
		return templates.UserDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return c.Redirect("/users/"+url.PathEscape(userDN), fiber.StatusSeeOther)
}

// modifyUserAttribute calls simple-ldap-go's ModifyUser with a single
// attribute update. An empty value produces an explicit delete (empty
// string slice value) so the server removes the attribute rather than
// storing an empty string.
func modifyUserAttribute(client *ldap.LDAP, dn, attr, value string) error {
	attrs := map[string][]string{}
	if value == "" {
		attrs[attr] = []string{}
	} else {
		attrs[attr] = []string{value}
	}

	return client.ModifyUser(dn, attrs)
}
