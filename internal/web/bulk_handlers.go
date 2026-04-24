// internal/web/bulk_handlers.go — bulk actions for /users, /groups,
// /computers lists (Phase 3 + 4).
//
// Each entity type has its own POST /<kind>/bulk endpoint; the action
// identifier comes in on the query string (?action=…) to keep the POST
// body a plain target_dn[] list.
//
// Write operations that simple-ldap-go does not currently expose in a
// portable way (DisableUser/DisableComputer on non-AD) return HTTP 501
// rather than a half-baked DIY.  Delete uses the generic DeleteByDN
// added in simple-ldap-go v1.11 — works for users, groups, computers.
package web

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// bulkRedirectAfter computes the post-action redirect target for a bulk
// handler. It preserves the originating filters (ou=, enabled=, member_of=,
// …) by reusing the Referer URL's query string. It also keeps or drops
// the ?panel= drawer-state parameter depending on whether the entity the
// drawer was showing is still there after the op:
//
//   - dropPanel=true (delete): always strips ?panel= because the
//     referenced entity is gone. If the Referer path was a single-entity
//     detail page (fallbackList + "/:dn"), it is rewritten to the parent
//     list so the user doesn't land on a dangling detail route.
//   - dropPanel=false (disable et al.): keeps the full Referer verbatim
//     so the drawer reopens on the same entity (now with updated state)
//     and the filter chips stay applied.
//
// Only same-origin Referer values are honoured to avoid open-redirect
// risk; cross-origin or unparseable Referer falls back to fallbackList.
func bulkRedirectAfter(c *fiber.Ctx, fallbackList string, dropPanel bool) string {
	ref := c.Get(fiber.HeaderReferer)
	if ref == "" {
		return fallbackList
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return fallbackList
	}

	// Reject cross-origin Referers. Allow relative Referers (empty Host),
	// which some clients still emit for same-origin POSTs.
	//
	// Compare hostnames via url.URL.Hostname() (strips any port) so dev
	// and proxy deployments on non-default ports (e.g. localhost:3000)
	// are not incorrectly flagged as cross-origin against Fiber's
	// port-less c.Hostname().
	if refURL.Host != "" && refURL.Hostname() != c.Hostname() {
		return fallbackList
	}

	// Use EscapedPath so percent-encoded DNs (e.g. /users/cn%3Dbob%2Cdc%3Dx)
	// round-trip unchanged; refURL.Path decodes by default which would
	// emit raw "=" and "," that the router then has to re-interpret.
	path := refURL.EscapedPath()
	q := refURL.Query()

	if dropPanel {
		q.Del("panel")
		// The Referer may have been a per-entity detail page
		// (fallbackList + "/:dn"); after delete the entity is gone, so
		// collapse back to the parent list route.
		if path != fallbackList && strings.HasPrefix(path, fallbackList+"/") {
			path = fallbackList
		}
	}

	// Only redirect to paths on the same list surface — defence in depth
	// against an attacker setting a crafted Referer to bounce through the
	// 303 into an unrelated route.
	if path != fallbackList && !strings.HasPrefix(path, fallbackList+"/") {
		return fallbackList
	}

	if len(q) == 0 {
		return path
	}

	return path + "?" + q.Encode()
}

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
		if !a.ldapConfig.IsActiveDirectory {
			// OpenLDAP inetOrgPerson has no portable enable/disable
			// attribute. Return 501 with the same message the template
			// used to render so the contract is unchanged for non-AD.
			return bulkNotImplemented(c, "disable users", "")
		}

		return a.bulkDisableUsers(c)
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
		return a.bulkDeleteGroups(c)
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
		if !a.ldapConfig.IsActiveDirectory {
			// Non-AD: no userAccountControl to flip. Keep the 501
			// contract in place so non-AD deployments see a clear
			// message rather than a silent no-op.
			return bulkNotImplemented(c, "disable computers", "")
		}

		return a.bulkDisableComputers(c)
	case "delete":
		return a.bulkDeleteComputers(c)
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
		return c.Redirect(bulkRedirectAfter(c, "/users", false), fiber.StatusSeeOther)
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

	return c.Redirect(bulkRedirectAfter(c, "/users", false), fiber.StatusSeeOther)
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
		return c.Redirect(bulkRedirectAfter(c, "/users", false), fiber.StatusSeeOther)
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

	return c.Redirect(bulkRedirectAfter(c, "/users", false), fiber.StatusSeeOther)
}

// bulkDeleteUsers deletes each user in target_dn[]. Per-entry failures
// are logged and summarised in the flash banner but do not abort the
// batch — callers land on /users regardless.
func (a *App) bulkDeleteUsers(c *fiber.Ctx) error {
	targets := collectTargetDNs(c)
	if len(targets) == 0 {
		return c.Redirect(bulkRedirectAfter(c, "/users", true), fiber.StatusSeeOther)
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	deleted := 0
	var firstErr error

	for _, userDN := range targets {
		if err := client.DeleteUser(userDN); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			log.Warn().Err(err).Str("user", userDN).Msg("bulk delete-user failed")

			continue
		}

		// Optimistic cache update: drop the user and scrub any group
		// memberships pointing at them, so the redirected list renders
		// correctly on the next request even if AD replication to the
		// readonly-bind DC hasn't caught up yet.
		if a.ldapCache != nil {
			a.ldapCache.OnDeleteUser(userDN)
		}

		deleted++
	}

	a.finaliseBulkDelete(c, "user", deleted, len(targets), firstErr)

	return c.Redirect(bulkRedirectAfter(c, "/users", true), fiber.StatusSeeOther)
}

// bulkDeleteGroups deletes each DN in target_dn[] as an LDAP entry
// via simple-ldap-go's generic DeleteByDN (which performs a raw
// ldap.Del on the DN). Flash summarises the result on the list page.
func (a *App) bulkDeleteGroups(c *fiber.Ctx) error {
	return a.bulkDeleteByDN(c, "group", "/groups", func(dn string) {
		if a.ldapCache != nil {
			a.ldapCache.OnDeleteGroup(dn)
		}
	})
}

// bulkDeleteComputers mirrors bulkDeleteGroups against the computers
// list. Uses the same generic DeleteByDN because computers and groups
// are both single-entry deletes without a type-specific helper.
func (a *App) bulkDeleteComputers(c *fiber.Ctx) error {
	return a.bulkDeleteByDN(c, "computer", "/computers", func(dn string) {
		if a.ldapCache != nil {
			a.ldapCache.OnDeleteComputer(dn)
		}
	})
}

// bulkDeleteByDN is the shared body of bulkDeleteGroups /
// bulkDeleteComputers: collect target_dn[], open a per-user LDAP
// binding, DeleteByDN each target, count successes, flash a summary,
// and redirect back to the list page at `redirectTo`. `kind` is the
// singular noun used in the flash and the log field. onCacheSuccess is
// invoked per successfully-deleted DN so the caller can apply an
// optimistic cache update (scrubbing the entity before the next
// Refresh() round-trip).
//
// Users have their own handler (bulkDeleteUsers) because we call the
// type-specific `client.DeleteUser` which also fires cache-hook work
// in simple-ldap-go that DeleteByDN bypasses.
func (a *App) bulkDeleteByDN(c *fiber.Ctx, kind, redirectTo string, onCacheSuccess func(dn string)) error {
	targets := collectTargetDNs(c)
	if len(targets) == 0 {
		return c.Redirect(bulkRedirectAfter(c, redirectTo, true), fiber.StatusSeeOther)
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	deleted := 0
	var firstErr error

	for _, dn := range targets {
		if err := ldap.DeleteByDN(c.UserContext(), client, dn); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			log.Warn().Err(err).Str("dn", dn).Str("kind", kind).Msg("bulk delete failed")

			continue
		}

		if onCacheSuccess != nil {
			onCacheSuccess(dn)
		}

		deleted++
	}

	a.finaliseBulkDelete(c, kind, deleted, len(targets), firstErr)

	return c.Redirect(bulkRedirectAfter(c, redirectTo, true), fiber.StatusSeeOther)
}

// bulkDisableUsers flips the ACCOUNTDISABLE bit (0x2) on each user DN
// in target_dn[] via simple-ldap-go v1.12's DisableUserContext. AD
// only — the caller in handleBulkUsers gates this on
// a.ldapConfig.IsActiveDirectory so non-AD deployments never reach here.
func (a *App) bulkDisableUsers(c *fiber.Ctx) error {
	return a.bulkUACDisable(c, "user", "/users",
		func(client *ldap.LDAP, dn string) error {
			return client.DisableUserContext(c.UserContext(), dn)
		},
		func(dn string) {
			if a.ldapCache != nil {
				a.ldapCache.OnDisableUser(dn)
			}
		})
}

// bulkDisableComputers mirrors bulkDisableUsers for computer entries.
// AD-only, same gating in handleBulkComputers.
func (a *App) bulkDisableComputers(c *fiber.Ctx) error {
	return a.bulkUACDisable(c, "computer", "/computers",
		func(client *ldap.LDAP, dn string) error {
			return client.DisableComputerContext(c.UserContext(), dn)
		},
		func(dn string) {
			if a.ldapCache != nil {
				a.ldapCache.OnDisableComputer(dn)
			}
		})
}

// bulkUACDisable is the shared body for bulkDisableUsers /
// bulkDisableComputers: open a per-user LDAP binding, run the given
// per-DN disable op, count successes, flash "Disabled N / M <kind>s".
// onCacheSuccess is invoked per successfully-disabled DN so the caller
// can flip Enabled=false in the local cache without waiting for the
// next Refresh() to notice via the readonly-bind DC.
// Pattern matches bulkDeleteByDN — different op, same batching.
func (a *App) bulkUACDisable(
	c *fiber.Ctx,
	kind, redirectTo string,
	op func(*ldap.LDAP, string) error,
	onCacheSuccess func(dn string),
) error {
	targets := collectTargetDNs(c)
	if len(targets) == 0 {
		return c.Redirect(bulkRedirectAfter(c, redirectTo, false), fiber.StatusSeeOther)
	}

	client, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = client.Close() }()

	disabled := 0
	var firstErr error

	for _, dn := range targets {
		if err := op(client, dn); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			log.Warn().Err(err).Str("dn", dn).Str("kind", kind).Msg("bulk disable failed")

			continue
		}

		if onCacheSuccess != nil {
			onCacheSuccess(dn)
		}

		disabled++
	}

	a.finaliseBulkDisable(c, kind, disabled, len(targets), firstErr)

	return c.Redirect(bulkRedirectAfter(c, redirectTo, false), fiber.StatusSeeOther)
}

// finaliseBulkDisable is the disable analogue of finaliseBulkDelete:
// invalidate the rendered-template cache so the redirected list page
// is re-rendered, log the summary, and flash the result.
//
// The LDAP cache itself was already updated optimistically in the
// per-entity loop via OnDisable{User,Computer} — we intentionally do
// NOT call ldapCache.Refresh() here: Refresh() queries the readonly-
// bind DC, which under normal AD replication delay returns the
// pre-disable state (Enabled=true) and overwrites our optimistic
// update. The 30 s background refresh picks up the upstream state
// once replication has caught up.
func (a *App) finaliseBulkDisable(c *fiber.Ctx, kind string, disabled, total int, firstErr error) {
	if disabled > 0 {
		a.invalidateTemplateCacheOnModification()
	}

	log.Info().
		Int("targeted", total).
		Int("disabled", disabled).
		Str("kind", kind).
		Msg("bulk disable complete")

	switch disabled {
	case total:
		a.setFlash(c, templates.SuccessFlash(
			fmt.Sprintf("Disabled %d %s%s.", disabled, kind, pluralSuffix(disabled))))
	case 0:
		a.setFlash(c, templates.ErrorFlash(
			fmt.Sprintf("Failed to disable any of %d %s%s: %s",
				total, kind, pluralSuffix(total), humaniseLDAPError(firstErr))))
	default:
		a.setFlash(c, templates.ErrorFlash(
			fmt.Sprintf("Disabled %d / %d %s%s (%s)",
				disabled, total, kind, pluralSuffix(total), humaniseLDAPError(firstErr))))
	}
}

// finaliseBulkDelete is the shared post-loop cleanup for all three
// bulk-delete handlers: invalidate the rendered-template cache and
// drop a "Deleted N / M <kind>s" flash on the next list page load.
// `kind` is the singular noun ("user", "group", "computer");
// pluralisation is naive "s"-suffix which is correct for all three.
//
// The LDAP cache itself was already updated optimistically in the
// per-entity loop via OnDelete{User,Group,Computer} — we intentionally
// do NOT call ldapCache.Refresh() here: Refresh() queries the readonly-
// bind DC, which under normal AD replication delay still sees the
// just-deleted entity and overwrites our optimistic scrub. The 30 s
// background refresh picks up the upstream state once replication
// has caught up.
func (a *App) finaliseBulkDelete(c *fiber.Ctx, kind string, deleted, total int, firstErr error) {
	if deleted > 0 {
		a.invalidateTemplateCacheOnModification()
	}

	log.Info().
		Int("targeted", total).
		Int("deleted", deleted).
		Str("kind", kind).
		Msg("bulk delete complete")

	switch deleted {
	case total:
		a.setFlash(c, templates.SuccessFlash(
			fmt.Sprintf("Deleted %d %s%s.", deleted, kind, pluralSuffix(deleted))))
	case 0:
		a.setFlash(c, templates.ErrorFlash(
			fmt.Sprintf("Failed to delete any of %d %s%s: %s",
				total, kind, pluralSuffix(total), humaniseLDAPError(firstErr))))
	default:
		a.setFlash(c, templates.ErrorFlash(
			fmt.Sprintf("Deleted %d / %d %s%s (%s)",
				deleted, total, kind, pluralSuffix(total), humaniseLDAPError(firstErr))))
	}
}

// pluralSuffix returns "s" when count != 1. Matches English
// pluralisation for "user(s)" / "group(s)" / "computer(s)".
func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
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
		return c.Redirect(bulkRedirectAfter(c, "/groups", false), fiber.StatusSeeOther)
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

	return c.Redirect(bulkRedirectAfter(c, "/groups", false), fiber.StatusSeeOther)
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
