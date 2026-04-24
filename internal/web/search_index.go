// internal/web/search_index.go
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/gofiber/fiber/v2"
)

// SearchIndexEntry is one record in the client-side search index.
// Kept intentionally narrow: only fields the fuzzy matcher and palette
// display need. Extending the shape requires both a server change here
// and a client change in v2-palette.js.
type SearchIndexEntry struct {
	Type    string `json:"type"`
	DN      string `json:"dn"`
	CN      string `json:"cn"`
	SAM     string `json:"sam,omitempty"`
	OU      string `json:"ou,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// handleSearchIndex renders the JSON search index derived from the
// in-memory ldap_cache. ETag is SHA-256 over the JSON body so clients
// can skip re-downloads while anything is in the cache.
func (a *App) handleSearchIndex(c *fiber.Ctx) error {
	entries := a.buildSearchIndex()
	body, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal search index: %w", err)
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

// buildSearchIndex materialises the cache contents into entries.
func (a *App) buildSearchIndex() []SearchIndexEntry {
	if a.ldapCache == nil {
		return []SearchIndexEntry{}
	}
	users := a.ldapCache.FindUsers(true)
	groups := a.ldapCache.FindGroups()
	computers := a.ldapCache.FindComputers(true)

	out := make([]SearchIndexEntry, 0, len(users)+len(groups)+len(computers))

	for _, u := range users {
		dn, cn := u.DN(), u.CN()
		if dn == "" || cn == "" {
			continue
		}
		enabled := u.Enabled
		out = append(out, SearchIndexEntry{
			Type:    "user",
			DN:      dn,
			CN:      cn,
			SAM:     u.SAMAccountName,
			OU:      immediateOU(dn),
			Enabled: &enabled,
		})
	}
	for _, g := range groups {
		dn, cn := g.DN(), g.CN()
		if dn == "" || cn == "" {
			continue
		}
		out = append(out, SearchIndexEntry{
			Type: "group", DN: dn, CN: cn, OU: immediateOU(dn),
		})
	}
	for _, c := range computers {
		dn, cn := c.DN(), c.CN()
		if dn == "" || cn == "" {
			continue
		}
		out = append(out, SearchIndexEntry{
			Type: "computer", DN: dn, CN: cn, OU: immediateOU(dn),
		})
	}

	// Stable deterministic order so the ETag (SHA-256 of the marshalled
	// JSON body) is stable across handler invocations with the same
	// cache contents. Without this, ldap_cache's iteration order can
	// vary between refreshes and every hit would mint a fresh ETag,
	// defeating the client-side cache entirely. Sort key is
	// (Type, CN, DN) — Type first keeps users / groups / computers
	// grouped in the palette, CN is the primary human label, DN is
	// the tie-breaker for entries that happen to share a CN.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].CN != out[j].CN {
			return out[i].CN < out[j].CN
		}

		return out[i].DN < out[j].DN
	})

	return out
}

// immediateOU returns the first `ou=...` RDN found walking the DN
// root-upward (i.e. the innermost/most-specific OU the entry lives
// in). Returns the full RDN ("ou=Engineering") so callers can pass it
// straight into ?ou=... filters without re-escaping.
//
// Uses go-ldap's ParseDN rather than raw comma splitting so escaped
// commas inside RDN values (e.g. `cn=Last\, First,ou=Sales,…`) don't
// get mistaken for RDN separators. Falls back to the empty string on
// parse failure — the caller degrades gracefully (no OU pivot link
// rendered, no ?ou= filter) rather than producing a nonsense OU name
// from a malformed DN.
func immediateOU(dn string) string {
	parsed, err := goldap.ParseDN(dn)
	if err != nil || parsed == nil {
		return ""
	}

	for _, rdn := range parsed.RDNs {
		for _, ava := range rdn.Attributes {
			if strings.EqualFold(ava.Type, "ou") {
				// Re-emit as "ou=<value>" (lower-case type) so the
				// query string matches whatever the URL helpers
				// produced when rendering the row, regardless of
				// how the directory reported the attribute.
				return "ou=" + ava.Value
			}
		}
	}

	return ""
}
