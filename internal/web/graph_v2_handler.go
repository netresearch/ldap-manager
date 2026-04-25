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
