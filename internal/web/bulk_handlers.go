// internal/web/bulk_handlers.go — bulk actions for /users, /groups,
// /computers lists (Phase 3 + 4).
//
// Each entity type has its own POST /<kind>/bulk endpoint; the action
// identifier comes in on the query string (?action=…) to keep the POST
// body a plain target_dn[] list.
//
// Write operations that simple-ldap-go does not currently expose (e.g.
// DeleteGroup, DeleteComputer, DisableUser, DisableComputer) return
// HTTP 501 Not Implemented with a short explanation rather than a
// half-baked DIY implementation — the latter risks corrupting directory
// state on unexpected schemas.
package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// bulkNotImplementedMessage is the response body for bulk actions that
// require LDAP operations simple-ldap-go does not yet expose. The HTTP
// status is 501.
const bulkNotImplementedMessage = "This bulk action is not yet implemented. " +
	"The underlying LDAP operation is not exposed by the current client library."

// handleBulkUsers dispatches multi-selected bulk actions from the /users
// list page.
func (a *App) handleBulkUsers(c *fiber.Ctx) error {
	_, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	action := c.Query("action")
	switch action {
	case "add-to-group":
		return a.bulkAddToGroup(c)
	case "remove-from-group":
		return a.bulkRemoveFromGroup(c)
	case "delete":
		return a.bulkDeleteUsers(c)
	case "disable":
		// sAMAccountName enable/disable on OpenLDAP (inetOrgPerson) has no
		// portable bit to flip — userAccountControl is AD-specific. Rather
		// than silently muddle the description attribute we stub here.
		return bulkNotImplemented(c, "disable users", "")
	default:
		return c.Status(fiber.StatusBadRequest).SendString("unknown bulk action")
	}
}

// handleBulkGroups dispatches multi-selected bulk actions from the /groups
// list page.
func (a *App) handleBulkGroups(c *fiber.Ctx) error {
	_, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	action := c.Query("action")
	switch action {
	case "add-members":
		return a.bulkAddMembersToGroups(c)
	case "delete":
		// simple-ldap-go does not expose DeleteGroup — stub.
		return bulkNotImplemented(c, "delete groups", "")
	default:
		return c.Status(fiber.StatusBadRequest).SendString("unknown bulk action")
	}
}

// handleBulkComputers dispatches multi-selected bulk actions from the
// /computers list page.
func (a *App) handleBulkComputers(c *fiber.Ctx) error {
	_, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	action := c.Query("action")
	switch action {
	case "disable":
		// Same reasoning as users "disable" — no portable write op.
		return bulkNotImplemented(c, "disable computers", "")
	case "delete":
		// simple-ldap-go does not expose DeleteComputer — stub.
		return bulkNotImplemented(c, "delete computers", "")
	default:
		return c.Status(fiber.StatusBadRequest).SendString("unknown bulk action")
	}
}

// bulkNotImplemented logs a TODO breadcrumb and returns 501 with a short
// human-readable message. The flavor string is only used for the log line
// so operators can grep for which branch fired.
func bulkNotImplemented(c *fiber.Ctx, flavor, extra string) error {
	targets := collectTargetDNs(c)

	log.Warn().
		Str("flavor", flavor).
		Int("targeted", len(targets)).
		Str("extra", extra).
		Msg("TODO: bulk action stubbed — LDAP op not exposed by simple-ldap-go")

	return c.Status(fiber.StatusNotImplemented).SendString(bulkNotImplementedMessage)
}

// bulkAddToGroup adds each user in target_dn[] to the group_dn.
// Failures are logged but do not abort the whole batch; the user lands
// back on /users afterwards regardless of per-entry outcomes.
//
// Fiber's FormValue("target_dn") collapses repeated fields to the first
// occurrence only, so we pull the raw body args via PeekMulti to get the
// full slice. The MultipartForm path is also covered for clients that
// prefer multipart/form-data.
//
//nolint:dupl // Parallel structure with bulkRemoveFromGroup and bulkAddMembersToGroups.
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

// bulkRemoveFromGroup removes each user in target_dn[] from the group_dn.
// Same batching semantics as bulkAddToGroup: per-entry failures are logged
// but do not abort the batch.
//
//nolint:dupl // Parallel structure with bulkAddToGroup and bulkAddMembersToGroups.
func (a *App) bulkRemoveFromGroup(c *fiber.Ctx) error {
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

	removed := 0
	for _, userDN := range targets {
		if err := client.RemoveUserFromGroup(userDN, groupDN); err != nil {
			log.Warn().Err(err).Str("user", userDN).Str("group", groupDN).Msg("bulk remove-from-group failed for user")

			continue
		}

		if a.ldapCache != nil {
			a.ldapCache.OnRemoveUserFromGroup(userDN, groupDN)
		}

		removed++
	}

	if removed > 0 {
		a.invalidateTemplateCacheOnModification()
	}

	log.Info().
		Int("targeted", len(targets)).
		Int("removed", removed).
		Str("group", groupDN).
		Msg("bulk remove-from-group complete")

	return c.Redirect("/users", fiber.StatusSeeOther)
}

// bulkDeleteUsers deletes each user in target_dn[]. Per-entry failures are
// logged but do not abort the batch — callers land on /users regardless.
func (a *App) bulkDeleteUsers(c *fiber.Ctx) error {
	targets := collectTargetDNs(c)
	if len(targets) == 0 {
		return c.Redirect("/users", fiber.StatusSeeOther)
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	deleted := 0
	for _, userDN := range targets {
		if err := client.DeleteUser(userDN); err != nil {
			log.Warn().Err(err).Str("user", userDN).Msg("bulk delete-user failed")

			continue
		}

		deleted++
	}

	if deleted > 0 {
		// No dedicated OnDelete hook in the cache — force a full refresh
		// by invalidating the template cache so stale lists don't linger.
		a.invalidateTemplateCacheOnModification()

		if a.ldapCache != nil {
			a.ldapCache.Refresh()
		}
	}

	log.Info().
		Int("targeted", len(targets)).
		Int("deleted", deleted).
		Msg("bulk delete-users complete")

	return c.Redirect("/users", fiber.StatusSeeOther)
}

// bulkAddMembersToGroups adds user_dn to each group listed in target_dn[].
// Inverse shape of bulkAddToGroup: one user → many groups. Useful for
// onboarding a newcomer into several team groups at once.
//
//nolint:dupl // Parallel structure with bulkAddToGroup and bulkRemoveFromGroup.
func (a *App) bulkAddMembersToGroups(c *fiber.Ctx) error {
	userDN := c.FormValue("user_dn")
	if userDN == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing user_dn")
	}

	targets := collectTargetDNs(c)
	if len(targets) == 0 {
		return c.Redirect("/groups", fiber.StatusSeeOther)
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	added := 0
	for _, groupDN := range targets {
		if err := client.AddUserToGroup(userDN, groupDN); err != nil {
			log.Warn().Err(err).Str("user", userDN).Str("group", groupDN).Msg("bulk add-members failed for group")

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
		Str("user", userDN).
		Msg("bulk add-members complete")

	return c.Redirect("/groups", fiber.StatusSeeOther)
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
