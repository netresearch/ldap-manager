package web

// Session-backed flash-message plumbing. Handlers that do a POST +
// redirect (bulk actions, mutations) call setFlash before the
// redirect, and list handlers on the receiving end call takeFlash to
// pop the stored message and render it above the page.
//
// Flashes are consumed on read: takeFlash deletes the session key so
// a stale flash doesn't survive across an unrelated later navigation.
// The payload is JSON-marshalled templates.Flash — Fiber's session
// storage uses gob under the hood and would require us to register
// custom types, JSON avoids that.

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/web/templates"
)

const flashSessionKey = "flash"

// setFlash stores a flash message in the current session. The flash is
// consumed by the next handler that calls takeFlash (typically a list
// page after a POST+redirect). Errors are logged but not surfaced —
// a failed flash is a UX regression, not a correctness one.
func (a *App) setFlash(c *fiber.Ctx, f templates.Flash) {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		log.Warn().Err(err).Msg("setFlash: session.Get failed")

		return
	}

	raw, err := json.Marshal(f)
	if err != nil {
		log.Warn().Err(err).Msg("setFlash: marshal failed")

		return
	}

	sess.Set(flashSessionKey, string(raw))

	if err := sess.Save(); err != nil {
		log.Warn().Err(err).Msg("setFlash: session.Save failed")
	}
}

// takeFlash reads and consumes any queued flash message. Returns nil
// when no flash is pending. Always returns at most one flash (our
// list pages show a single banner).
func (a *App) takeFlash(c *fiber.Ctx) []templates.Flash {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return nil
	}

	raw := sess.Get(flashSessionKey)
	if raw == nil {
		return nil
	}

	sess.Delete(flashSessionKey)
	if err := sess.Save(); err != nil {
		log.Warn().Err(err).Msg("takeFlash: session.Save after Delete failed")
	}

	s, ok := raw.(string)
	if !ok {
		return nil
	}

	var f templates.Flash
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		log.Warn().Err(err).Msg("takeFlash: unmarshal failed")

		return nil
	}

	return []templates.Flash{f}
}
