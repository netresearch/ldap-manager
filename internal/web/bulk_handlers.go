// internal/web/bulk_handlers.go — bulk actions for the /users list (Phase 3).
package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// handleBulkUsers dispatches multi-selected bulk actions from the /users
// list page. Phase 3 ships a single action — add-to-group — so the
// dispatch is hard-coded rather than wired through a registry. Adding
// another action (e.g. remove-from-group) is a matter of growing the
// switch and handing off to a new helper.
func (a *App) handleBulkUsers(c *fiber.Ctx) error {
	_, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	action := c.Query("action")
	switch action {
	case "add-to-group":
		return a.bulkAddToGroup(c)
	default:
		return c.Status(fiber.StatusBadRequest).SendString("unknown bulk action")
	}
}

// bulkAddToGroup adds each user in target_dn[] to the group_dn.
// Failures are logged but do not abort the whole batch; the user lands
// back on /users afterwards regardless of per-entry outcomes.
//
// Fiber's FormValue("target_dn") collapses repeated fields to the first
// occurrence only, so we pull the raw body args via PeekMulti to get the
// full slice. The MultipartForm path is also covered for clients that
// prefer multipart/form-data.
func (a *App) bulkAddToGroup(c *fiber.Ctx) error {
	groupDN := c.FormValue("group_dn")
	if groupDN == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing group_dn")
	}

	targets := collectTargetDNs(c)
	if len(targets) == 0 {
		return c.Redirect("/users", fiber.StatusSeeOther)
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	added := 0
	for _, userDN := range targets {
		if err := client.AddUserToGroup(userDN, groupDN); err != nil {
			log.Warn().Err(err).Str("user", userDN).Str("group", groupDN).Msg("bulk add-to-group failed for user")

			continue
		}

		if a.ldapCache != nil {
			a.ldapCache.OnAddUserToGroup(userDN, groupDN)
		}

		added++
	}

	if added > 0 {
		a.invalidateTemplateCacheOnModification()
	}

	log.Info().
		Int("targeted", len(targets)).
		Int("added", added).
		Str("group", groupDN).
		Msg("bulk add-to-group complete")

	return c.Redirect("/users", fiber.StatusSeeOther)
}

// collectTargetDNs extracts the target_dn[] list from both URL-encoded
// form bodies (via PostArgs().PeekMulti) and multipart bodies (via
// MultipartForm.Value).
func collectTargetDNs(c *fiber.Ctx) []string {
	if form, err := c.MultipartForm(); err == nil && form != nil {
		if vs := form.Value["target_dn"]; len(vs) > 0 {
			return vs
		}
	}

	raw := c.Request().PostArgs().PeekMulti("target_dn")
	if len(raw) == 0 {
		return nil
	}

	out := make([]string, 0, len(raw))
	for _, v := range raw {
		out = append(out, string(v))
	}

	return out
}
