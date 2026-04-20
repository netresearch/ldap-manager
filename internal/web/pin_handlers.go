// internal/web/pin_handlers.go
package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// handlePin pins a target DN for the authenticated user (spec §6.5).
// Expects form field "target" carrying the DN. Returns 204 on success.
func (a *App) handlePin(c *fiber.Ctx) error { return a.togglePin(c, true) }

// handleUnpin removes a pinned target DN for the authenticated user
// (spec §6.5). Idempotent: unpinning a non-existent target is still 204.
func (a *App) handleUnpin(c *fiber.Ctx) error { return a.togglePin(c, false) }

// togglePin is the shared implementation for Add/Remove. add=true pins,
// add=false unpins.
func (a *App) togglePin(c *fiber.Ctx, add bool) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	userDN, _ := sess.Get("dn").(string)
	if userDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	target := c.FormValue("target")
	if target == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing target")
	}

	if add {
		if err := a.pinnedStore.Add(userDN, target); err != nil {
			log.Error().Err(err).Str("user", userDN).Str("target", target).Msg("pin failed")

			return handle500(c, err)
		}
	} else {
		if err := a.pinnedStore.Remove(userDN, target); err != nil {
			log.Error().Err(err).Str("user", userDN).Str("target", target).Msg("unpin failed")

			return handle500(c, err)
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}
