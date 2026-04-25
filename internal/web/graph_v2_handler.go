// internal/web/graph_v2_handler.go — /graph and /api/graph.json.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// handleGraphJSON serves /api/graph.json?entity=<dn>&depth=<N>. Response
// shape documented in the spec §4.1. ETag is sha256 of the marshalled
// body to mirror /api/search-index.json.
func (a *App) handleGraphJSON(c *fiber.Ctx) error {
	data, status, errMsg := a.buildGraphFromQuery(c)
	if status != 0 {
		return c.Status(status).SendString(errMsg)
	}

	body, err := json.Marshal(data)
	if err != nil {
		log.Error().
			Err(err).
			Str("entity", c.Query("entity")).
			Msg("graph: marshal failed")

		return c.Status(fiber.StatusInternalServerError).SendString("internal error")
	}

	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	c.Set("ETag", etag)
	c.Set("Cache-Control", "private, must-revalidate")

	if match := c.Get("If-None-Match"); match != "" && match == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}

	c.Set("Content-Type", "application/json; charset=utf-8")

	return c.Send(body)
}

// buildGraphFromQuery parses ?entity= and ?depth= from c and returns the
// resulting graph. The integer return is the HTTP status the caller should
// emit when the request was rejected; 0 means "no error, render data".
// On success returns (data, 0, ""). On failure returns
// (nil, status, message) so the caller can render the response with the
// project's standard `c.Status(...).SendString(...)` idiom rather than
// writing the response from inside the helper. Callers MUST branch on
// `status != 0` (not on `data == nil`) so the error path stays explicit.
func (a *App) buildGraphFromQuery(c *fiber.Ctx) (*ldap_cache.GraphData, int, string) {
	entity := c.Query("entity")
	if entity == "" {
		return nil, fiber.StatusBadRequest, "missing entity"
	}
	if _, err := goldap.ParseDN(entity); err != nil {
		return nil, fiber.StatusBadRequest, "invalid DN"
	}

	depth := 2
	if raw := c.Query("depth"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			// Fail fast on non-numeric input rather than silently
			// falling back to the default — matches the "validate
			// early" rule in internal/CLAUDE.md.
			return nil, fiber.StatusBadRequest, "invalid depth"
		}
		depth = n
	}
	// Clamping of in-range numeric values happens inside BuildGraph.

	data, err := a.ldapCache.BuildGraph(entity, depth)
	if err != nil {
		if errors.Is(err, ldap_cache.ErrGraphNotFound) {
			return nil, fiber.StatusNotFound, "entity not found"
		}

		// Log the underlying error with context, but return a generic
		// message to the client — `err.Error()` may contain DN fragments
		// or other internals that should not appear in the response body
		// (see internal/web/CLAUDE.md: "Error responses don't leak
		// sensitive data").
		log.Error().
			Err(err).
			Str("entity", entity).
			Int("depth", depth).
			Msg("BuildGraph failed")

		return nil, fiber.StatusInternalServerError, "internal error"
	}

	return data, 0, ""
}

// handleGraphV2 serves /graph?entity=<dn>&depth=<N> as HTML. Shares the
// build path with handleGraphJSON; wraps the result in the Templ page.
func (a *App) handleGraphV2(c *fiber.Ctx) error {
	data, status, errMsg := a.buildGraphFromQuery(c)
	if status != 0 {
		return c.Status(status).SendString(errMsg)
	}

	// Best-effort viewer DN for the VM: in production this route is
	// behind RequireAuth so c.Locals already carries the DN, and the
	// session fallback never fires. In test harnesses (and any future
	// no-auth callers) we fall back to an empty string rather than
	// invoking resolveViewerDN — the latter writes a 303 to /login on
	// missing session, which would clobber the response we're about to
	// render.
	viewer := GetUserDN(c)
	if viewer == "" {
		if sess, err := a.sessionStore.Get(c); err == nil {
			viewer, _ = sess.Get("dn").(string)
		}
	}

	vm := templates.GraphPageVM{
		Data:       data,
		FocusLabel: graphFocusLabel(data),
		FocusType:  graphFocusType(data),
		ViewerDN:   viewer,
	}

	return a.templateCache.RenderWithCache(c, templates.GraphPageV2(vm))
}

// graphFocusLabel returns the human-friendly label for the focus node
// (the ring-0 entry), or "" when no focus (list-page Graph mode, Slice 5).
func graphFocusLabel(data *ldap_cache.GraphData) string {
	if data.Focus == "" {
		return ""
	}

	for _, n := range data.Nodes {
		if n.Ring == 0 {
			return n.Label
		}
	}

	return data.Focus
}

// graphFocusType returns the type ("user", "group", "computer", "ou") of
// the ring-0 focus node, or "" when no focus.
func graphFocusType(data *ldap_cache.GraphData) string {
	for _, n := range data.Nodes {
		if n.Ring == 0 {
			return string(n.Type)
		}
	}

	return ""
}
