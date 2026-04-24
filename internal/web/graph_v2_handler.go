// internal/web/graph_v2_handler.go — /graph and /api/graph.json.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	if data == nil {
		return c.Status(status).SendString(errMsg)
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	if match := c.Get("If-None-Match"); match != "" && match == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}

	c.Set("Content-Type", "application/json; charset=utf-8")
	c.Set("ETag", etag)
	c.Set("Cache-Control", "private, must-revalidate")

	return c.Send(body)
}

// buildGraphFromQuery parses ?entity= and ?depth= from c and returns the
// resulting graph along with the HTTP status the caller should emit when
// data is nil. On success returns (data, 0, ""). On failure returns
// (nil, status, message) so the caller can render the response with the
// project's standard `c.Status(...).SendString(...)` idiom rather than
// writing the response from inside the helper.
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
		if n, err := strconv.Atoi(raw); err == nil {
			depth = n
		}
	}
	// Clamping happens inside BuildGraph.

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
