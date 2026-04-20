// internal/web/search_index.go
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

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
	return out
}

// immediateOU returns the first `ou=...` RDN found when walking a DN
// left to right. Empty string if none.
func immediateOU(dn string) string {
	for i := 0; i < len(dn); i++ {
		if dn[i] == ',' {
			rdn := dn[i+1:]
			end := len(rdn)
			for j := 0; j < len(rdn); j++ {
				if rdn[j] == ',' {
					end = j
					break
				}
			}
			if end >= 3 && (rdn[:3] == "ou=" || rdn[:3] == "OU=") {
				return rdn[:end]
			}
		}
	}
	return ""
}
