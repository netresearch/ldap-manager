// internal/web/graph_view.go — list-page view-mode helpers.
package web

import (
	"github.com/gofiber/fiber/v2"
)

// graphViewCookie is the cookie name used to remember the user's
// preferred list-page view mode. Reads / writes go through pickView so
// the storage detail stays local to this file.
const graphViewCookie = "graph-view"

// pickView resolves the effective list-page view mode for the current
// request and persists explicit choices to a cookie so the preference
// follows the user across /users, /groups, and /computers.
//
// Resolution order:
//  1. ?view=… query parameter (explicit choice — set the cookie).
//  2. graph-view cookie (sticky preference from a previous request).
//  3. "list" (default).
//
// Unknown values normalise to "list" so a typo'd URL doesn't render
// nothing.
func pickView(c *fiber.Ctx) string {
	if raw := c.Query("view"); raw != "" {
		v := normaliseView(raw)
		setViewCookie(c, v)

		return v
	}

	return normaliseView(c.Cookies(graphViewCookie))
}

// normaliseView clamps an unknown view string to the safe default. Any
// new view modes need to be added here AND to the segmented toggle in
// internal/web/templates/graph_toggle.templ.
func normaliseView(s string) string {
	switch s {
	case "list", "table", "graph":
		return s
	default:
		return "list"
	}
}

// setViewCookie writes the user's explicit choice so future requests
// without ?view= still resolve to the same mode. SameSite=Strict
// matches the rest of the session security profile; HTTPOnly keeps the
// preference out of JS reach (the segmented toggle reads `currentView`
// from the rendered template, not from the cookie).
func setViewCookie(c *fiber.Ctx, v string) {
	c.Cookie(&fiber.Cookie{
		Name:     graphViewCookie,
		Value:    v,
		Path:     "/",
		MaxAge:   30 * 24 * 3600, // 30 days
		HTTPOnly: true,
		SameSite: "Strict",
	})
}
