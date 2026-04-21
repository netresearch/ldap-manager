// internal/web/pin_handlers.go
package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// handlePin pins a target DN for the authenticated user (spec §6.5).
// Expects form field "target" carrying the DN. On HX-Request, responds
// with the updated pin-star fragment so htmx swaps the glyph in place;
// otherwise returns 204.
func (a *App) handlePin(c *fiber.Ctx) error { return a.togglePin(c, true) }

// handleUnpin removes a pinned target DN for the authenticated user
// (spec §6.5). Idempotent: unpinning a non-existent target is still a
// success response.
func (a *App) handleUnpin(c *fiber.Ctx) error { return a.togglePin(c, false) }

// togglePin is the shared implementation for Add/Remove. add=true pins,
// add=false unpins. On HX-Request, the response body is the updated
// pin-star form so the star glyph flips inline without a reload.
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

	// htmx rountrip: swap in the updated fragment so the UI reflects the
	// new state without a full page reload.
	if c.Get("HX-Request") == "true" {
		c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")

		// Home-page unpin clicks target #pinned-block and expect the whole
		// nav back with the remaining pins.
		if c.Query("context") == "home" {
			pinned, _ := a.pinnedEntriesFor(userDN)

			return templates.PinnedBlock(pinned).Render(c.UserContext(), c.Response().BodyWriter())
		}

		entityType := c.FormValue("type")
		if entityType == "" {
			entityType = "item"
		}

		return templates.PinStarFragment(entityType, target, add).Render(c.UserContext(), c.Response().BodyWriter())
	}

	return c.SendStatus(fiber.StatusNoContent)
}
