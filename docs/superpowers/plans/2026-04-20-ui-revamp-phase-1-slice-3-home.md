# UI Revamp — Phase 1 Slice 3: Home + Shell + Command Palette

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the command-first home page — top nav, greeting, pinned (server), recents (client), and a keyboard-driven ⌘K palette that navigates to existing pages. Also introduces the pin/unpin backend and the search-index endpoint that later slices will reuse.

**Architecture:** Build on the Pico + app.css foundation from Slices 1–2. Still no Alpine.js (strict CSP disallows its default build; Phase 1 doesn't need reactive state). Add **htmx** for pin/unpin partial swaps. Palette is plain JS on top of the native HTML `<dialog>` element. Server exposes a JSON search index derived from the existing `ldap_cache`.

**Tech Stack:**
- Go + Fiber + Templ (existing)
- Pico CSS + hand-written `app.css` (existing)
- **htmx v2** (vendored in Slice 1; loaded here for the first time)
- Plain ES5 JS for palette and recents (no Alpine, no TS)
- bbolt for per-user pinned store (reusing the existing session bbolt file)
- stdlib `encoding/json` for the search index

**Spec reference:** `docs/superpowers/specs/2026-04-20-ui-revamp-design.md` §5 (IA), §6.1–6.8 (features), §9 slice 3.

**Out of scope for this plan (future slices):**
- Rewriting `/users`, `/groups`, `/computers` list pages (Slices 4–6) — the palette still navigates to them, styled in the old Tailwind template until those slices.
- Detail drawer / pivot-link system (Slice 4).
- Inline edit, bulk actions, graph view (Phase 2/3).

---

## File Structure

**New files:**
- `internal/web/pinned.go` — per-user pinned-items store (bbolt bucket wrapper; `List`, `Add`, `Remove`, `IsPinned`).
- `internal/web/pinned_test.go` — unit tests against a temp bbolt file.
- `internal/web/search_index.go` — HTTP handler returning JSON list of `{type, dn, cn, sam, ou}` derived from `ldap_cache`. Sets `ETag` based on cache version.
- `internal/web/search_index_test.go` — table-driven unit tests.
- `internal/web/pin_handlers.go` — HTTP handlers for POST /pin, POST /unpin.
- `internal/web/pin_handlers_test.go` — handler tests.
- `internal/web/home_handler.go` — HTTP handler for GET / (HomeV2).
- `internal/web/templates/topnav_v2.templ` — top nav component.
- `internal/web/templates/home_v2.templ` — home page.
- `internal/web/templates/palette_v2.templ` — `<dialog>` overlay shell.
- `internal/web/templates/pinned_fragment.templ` — list-of-pins fragment for htmx swap.
- `internal/web/static/js/v2-palette.js` — palette logic (open/close, fuzzy match, keyboard nav). **All DOM construction uses createElement + textContent — no innerHTML with dynamic content.**
- `internal/web/static/js/v2-recents.js` — localStorage helpers + home-page renderer. Same safe-DOM rules.

**Modified files:**
- `internal/web/server.go` — register `POST /pin`, `POST /unpin`, `GET /api/search-index.json`; swap `/` to `HomeV2`.
- `internal/web/templates/base_v2.templ` — load htmx + palette + recents JS deferred.
- `internal/web/static/app.css` — append topnav, home layout, pinned/recents blocks, palette overlay, keyboard-hint styles.

**Not modified:** old `base.templ`, `login.templ`, `index.templ`, `users.templ`, `groups.templ`, `computers.templ` — still power every route other than `/login` and `/` until later slices.

---

## Pre-flight

- [ ] **Step 0.1: Confirm branch + state**

```bash
cd /home/cybot/projects/ldap-manager-ui-revamp-phase-1a
git status --short
git log --oneline ddea7d9..HEAD | wc -l
go test ./... 2>&1 | tail -3
```

Expected: clean tree, ~17 commits above base, tests pass. This plan stacks commits on top of the existing feature branch; no new branch.

---

## Task 1 — Pinned store (bbolt)

**Files:**
- Create: `internal/web/pinned.go`
- Create: `internal/web/pinned_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/web/pinned_test.go
package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func newTestPinStore(t *testing.T) *PinnedStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pinned.bbolt")
	db, err := bolt.Open(path, 0o600, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _ = os.Remove(path) })
	store, err := NewPinnedStore(db)
	require.NoError(t, err)
	return store
}

func TestPinnedStore_AddListRemove(t *testing.T) {
	s := newTestPinStore(t)

	user := "uid=alice,ou=Users,dc=test"
	g1 := "cn=admins,ou=Groups,dc=test"
	g2 := "cn=devs,ou=Groups,dc=test"

	got, err := s.List(user)
	require.NoError(t, err)
	assert.Empty(t, got)

	require.NoError(t, s.Add(user, g1))
	require.NoError(t, s.Add(user, g2))

	got, err = s.List(user)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{g1, g2}, got)

	pinned, err := s.IsPinned(user, g1)
	require.NoError(t, err)
	assert.True(t, pinned)

	pinned, err = s.IsPinned(user, "cn=never,dc=test")
	require.NoError(t, err)
	assert.False(t, pinned)

	require.NoError(t, s.Remove(user, g1))
	got, err = s.List(user)
	require.NoError(t, err)
	assert.Equal(t, []string{g2}, got)

	require.NoError(t, s.Remove(user, g1)) // double-remove is idempotent
}

func TestPinnedStore_PerUser(t *testing.T) {
	s := newTestPinStore(t)
	_ = s.Add("uid=alice,dc=test", "cn=x,dc=test")
	_ = s.Add("uid=bob,dc=test", "cn=y,dc=test")

	alice, _ := s.List("uid=alice,dc=test")
	bob, _ := s.List("uid=bob,dc=test")

	assert.Equal(t, []string{"cn=x,dc=test"}, alice)
	assert.Equal(t, []string{"cn=y,dc=test"}, bob)
}

func TestPinnedStore_RejectsEmpty(t *testing.T) {
	s := newTestPinStore(t)
	assert.Error(t, s.Add("", "cn=x"))
	assert.Error(t, s.Add("uid=alice", ""))
	assert.Error(t, s.Remove("", "cn=x"))
	assert.Error(t, s.Remove("uid=alice", ""))
}
```

- [ ] **Step 2: Run — expect FAIL (undefined types)**

```bash
go test ./internal/web/ -run TestPinnedStore -v
```

Expected: compile error (`PinnedStore`, `NewPinnedStore` undefined).

- [ ] **Step 3: Implement the store**

```go
// internal/web/pinned.go
package web

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// pinnedBucketName is the top-level bucket. Each user gets a nested
// sub-bucket keyed by their DN, inside which target-DN bytes map to an
// ISO-8601 creation timestamp.
var pinnedBucketName = []byte("pinned")

// PinnedStore is a per-user pinned-items store backed by bbolt.
type PinnedStore struct {
	db *bolt.DB
}

// NewPinnedStore ensures the top bucket exists and returns a ready store.
func NewPinnedStore(db *bolt.DB) (*PinnedStore, error) {
	if db == nil {
		return nil, errors.New("pinned store: db is nil")
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(pinnedBucketName)
		return err
	}); err != nil {
		return nil, fmt.Errorf("pinned store: init bucket: %w", err)
	}
	return &PinnedStore{db: db}, nil
}

// List returns the target DNs pinned by the given user.
func (s *PinnedStore) List(userDN string) ([]string, error) {
	if userDN == "" {
		return nil, errors.New("pinned: empty user DN")
	}
	var out []string
	err := s.db.View(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket([]byte(userDN))
		if sub == nil {
			return nil
		}
		return sub.ForEach(func(k, _ []byte) error {
			out = append(out, string(bytes.Clone(k)))
			return nil
		})
	})
	return out, err
}

// Add records a pin. Idempotent: re-adding updates the timestamp.
func (s *PinnedStore) Add(userDN, targetDN string) error {
	if userDN == "" || targetDN == "" {
		return errors.New("pinned: empty user or target DN")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return s.db.Update(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return errors.New("pinned: top bucket missing")
		}
		sub, err := top.CreateBucketIfNotExists([]byte(userDN))
		if err != nil {
			return fmt.Errorf("pinned: create user bucket: %w", err)
		}
		return sub.Put([]byte(targetDN), []byte(now))
	})
}

// Remove deletes a pin. No error if the pin doesn't exist.
func (s *PinnedStore) Remove(userDN, targetDN string) error {
	if userDN == "" || targetDN == "" {
		return errors.New("pinned: empty user or target DN")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket([]byte(userDN))
		if sub == nil {
			return nil
		}
		return sub.Delete([]byte(targetDN))
	})
}

// IsPinned returns true iff (userDN, targetDN) exists in the store.
func (s *PinnedStore) IsPinned(userDN, targetDN string) (bool, error) {
	if userDN == "" || targetDN == "" {
		return false, errors.New("pinned: empty user or target DN")
	}
	var pinned bool
	err := s.db.View(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket([]byte(userDN))
		if sub == nil {
			return nil
		}
		pinned = sub.Get([]byte(targetDN)) != nil
		return nil
	})
	return pinned, err
}
```

- [ ] **Step 4: Run — expect PASS**

```bash
go test ./internal/web/ -run TestPinnedStore -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/web/pinned.go internal/web/pinned_test.go
git commit -S --signoff -m "feat(web): add per-user PinnedStore on bbolt (spec §6.5)"
```

---

## Task 2 — Search index endpoint

**Files:**
- Create: `internal/web/search_index.go`
- Create: `internal/web/search_index_test.go`
- Modify: `internal/web/server.go` (register route)

- [ ] **Step 1: Write the failing test**

```go
// internal/web/search_index_test.go
package web

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIndex_ShapeAndContentType(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/search-index.json", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	assert.NotEmpty(t, resp.Header.Get("ETag"))

	body, _ := io.ReadAll(resp.Body)
	var entries []SearchIndexEntry
	require.NoError(t, json.Unmarshal(body, &entries))

	for _, e := range entries {
		assert.Contains(t, []string{"user", "group", "computer"}, e.Type)
		assert.NotEmpty(t, e.DN)
		assert.NotEmpty(t, e.CN)
	}
}

func TestSearchIndex_ETagRespected(t *testing.T) {
	app, _ := setupFullTestApp(t)

	resp1, err := app.fiber.Test(httptest.NewRequest("GET", "/api/search-index.json", nil))
	require.NoError(t, err)
	etag := resp1.Header.Get("ETag")
	require.NotEmpty(t, etag)
	_ = resp1.Body.Close()

	req2 := httptest.NewRequest("GET", "/api/search-index.json", nil)
	req2.Header.Set("If-None-Match", etag)
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, fiber.StatusNotModified, resp2.StatusCode)
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/web/ -run TestSearchIndex -v
```

- [ ] **Step 3: Implement**

```go
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
	users := a.ldapCache.FindUsers()
	groups := a.ldapCache.FindGroups()
	computers := a.ldapCache.FindComputers()

	out := make([]SearchIndexEntry, 0, len(users)+len(groups)+len(computers))

	for _, u := range users {
		enabled := u.Enabled
		out = append(out, SearchIndexEntry{
			Type:    "user",
			DN:      u.DN(),
			CN:      u.CN(),
			SAM:     u.SAMAccountName,
			OU:      immediateOU(u.DN()),
			Enabled: &enabled,
		})
	}
	for _, g := range groups {
		out = append(out, SearchIndexEntry{
			Type: "group", DN: g.DN(), CN: g.CN(), OU: immediateOU(g.DN()),
		})
	}
	for _, c := range computers {
		out = append(out, SearchIndexEntry{
			Type: "computer", DN: c.DN(), CN: c.CN(), OU: immediateOU(c.DN()),
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
```

- [ ] **Step 4: Register the route**

In `internal/web/server.go`, among the other authenticated GETs:

```go
app.Get("/api/search-index.json", a.handleSearchIndex)
```

- [ ] **Step 5: Run — expect PASS**

```bash
go test ./internal/web/ -run TestSearchIndex -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/web/search_index.go internal/web/search_index_test.go internal/web/server.go
git commit -S --signoff -m "feat(web): GET /api/search-index.json with ETag caching (spec §6.1)"
```

---

## Task 3 — Pin / unpin HTTP handlers

**Files:**
- Create: `internal/web/pin_handlers.go`
- Create: `internal/web/pin_handlers_test.go`
- Modify: `internal/web/server.go` (wire store into `App`; register routes)

- [ ] **Step 1: Plumbing — add the store to `App`**

Open `internal/web/server.go`. In the `App` struct, add:

```go
pinnedStore *PinnedStore
```

In `NewApp` (or the constructor that opens the bbolt session DB), after the bbolt DB is opened, add:

```go
ps, err := NewPinnedStore(sessionDB)
if err != nil {
    return nil, fmt.Errorf("init pinned store: %w", err)
}
a.pinnedStore = ps
```

If the session store doesn't expose its DB, open the same file path with a second `bolt.Open` — bbolt supports multi-open from the same process.

- [ ] **Step 2: Write the failing handler test**

```go
// internal/web/pin_handlers_test.go
package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPin_RequiresAuth(t *testing.T) {
	app, _ := setupFullTestApp(t)

	form := url.Values{"target": {"cn=admins,dc=test"}}
	req := httptest.NewRequest("POST", "/pin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t,
		[]int{fiber.StatusFound, fiber.StatusSeeOther, fiber.StatusUnauthorized},
		resp.StatusCode)
}

func TestPinUnpin_RoundTrip(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, store)

	target := "cn=demo-group,ou=Groups,dc=test"
	const authDN = "cn=admin,dc=test,dc=local"

	form := url.Values{"target": {target}}

	req := httptest.NewRequest("POST", "/pin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Contains(t, []int{fiber.StatusNoContent, fiber.StatusOK}, resp.StatusCode)

	pinned, err := app.pinnedStore.IsPinned(authDN, target)
	require.NoError(t, err)
	assert.True(t, pinned)

	req2 := httptest.NewRequest("POST", "/unpin", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	_ = resp2.Body.Close()

	pinned, _ = app.pinnedStore.IsPinned(authDN, target)
	assert.False(t, pinned)
}
```

If `createAuthSession` isn't scoped into this test file's package yet, follow the pattern used by the existing auth-requiring tests (see `internal/web/CLAUDE.md` → "Auth Session Testing").

- [ ] **Step 3: Run — expect FAIL**

```bash
go test ./internal/web/ -run TestPin -v
```

- [ ] **Step 4: Implement handlers**

```go
// internal/web/pin_handlers.go
package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

func (a *App) handlePin(c *fiber.Ctx) error   { return a.togglePin(c, true) }
func (a *App) handleUnpin(c *fiber.Ctx) error { return a.togglePin(c, false) }

func (a *App) togglePin(c *fiber.Ctx, add bool) error {
	sess, err := a.sessions.Get(c)
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
```

- [ ] **Step 5: Register routes**

In `internal/web/server.go`, register inside the authenticated + CSRF-protected block:

```go
app.Post("/pin", a.handlePin)
app.Post("/unpin", a.handleUnpin)
```

- [ ] **Step 6: Run — expect PASS**

```bash
go test ./internal/web/ -run TestPin -v
```

- [ ] **Step 7: Commit**

```bash
git add internal/web/pin_handlers.go internal/web/pin_handlers_test.go internal/web/server.go
git commit -S --signoff -m "feat(web): POST /pin, POST /unpin (CSRF-protected, spec §6.5)"
```

---

## Task 4 — Top nav + home templates

**Files:**
- Create: `internal/web/templates/topnav_v2.templ`
- Create: `internal/web/templates/home_v2.templ`
- Create: `internal/web/templates/pinned_fragment.templ`
- Modify: `internal/web/static/app.css` (append nav + home styles)

- [ ] **Step 1: Top nav templ**

```go
// internal/web/templates/topnav_v2.templ
package templates

// PinnedEntry is the minimal view-model the pinned_fragment renders.
// Produced server-side from PinnedStore + ldap_cache lookups.
type PinnedEntry struct {
	Type string // "user" | "group" | "computer"
	DN   string
	CN   string
}

// topnavV2 renders the sticky top navigation shared across signed-in
// routes.
templ topnavV2(activePath string) {
	<header class="topnav">
		<a class="topnav__logo" href="/" aria-label="Home">LDAP Manager</a>
		<span class="topnav__spacer"></span>
		<button type="button" class="topnav__cmdk" data-open-palette aria-label="Open command palette">
			<span aria-hidden="true">⌕</span>
			<span class="topnav__cmdk-label">Search</span>
			<kbd>⌘K</kbd>
		</button>
		<button type="button" class="icon-btn" data-toggle="theme" aria-label="Toggle theme">
			<span aria-hidden="true">◐</span>
		</button>
		<button type="button" class="icon-btn" data-toggle="density" aria-label="Toggle density">
			<span aria-hidden="true">⇥</span>
		</button>
		<a class="icon-btn" href="/logout" aria-label="Log out">
			<span aria-hidden="true">↗</span>
		</a>
	</header>
	<nav class="topnav-secondary" aria-label="Primary">
		<a href="/users" class={ navLinkClass(activePath, "/users") }>Users</a>
		<a href="/groups" class={ navLinkClass(activePath, "/groups") }>Groups</a>
		<a href="/computers" class={ navLinkClass(activePath, "/computers") }>Computers</a>
	</nav>
}

func navLinkClass(activePath, linkPath string) string {
	if activePath == linkPath {
		return "topnav-secondary__link topnav-secondary__link--active"
	}
	return "topnav-secondary__link"
}
```

- [ ] **Step 2: Home page templ**

```go
// internal/web/templates/home_v2.templ
package templates

// HomeV2 is the signed-in home page. Greeting + prominent search entry
// (opens the palette) + pinned (server-driven, htmx-swap) + recents
// (client-side, populated by v2-recents.js).
templ HomeV2(userCN string, pinned []PinnedEntry) {
	@baseV2("Home") {
		@topnavV2("/")

		<main class="home">
			<h1 class="home__greet">Hi { userCN } — what are you looking for?</h1>
			<p class="home__lede">
				Search users, groups, computers, or run an action.
				Press <kbd>⌘K</kbd> from anywhere.
			</p>

			<button type="button" class="home__search" data-open-palette aria-label="Open command palette">
				<span aria-hidden="true">⌕</span>
				<span>Search for a user, group, or action…</span>
				<kbd>⌘K</kbd>
			</button>

			<section class="home__blocks">
				<nav aria-label="Pinned" id="pinned-block">
					<h2 class="home__heading">Pinned</h2>
					@pinnedFragment(pinned)
				</nav>

				<nav aria-label="Recent" id="recents-block" data-recents>
					<h2 class="home__heading">Recent</h2>
					<ul class="home__list" data-recents-list>
						<li class="home__list-empty" data-recents-empty>Nothing yet — browse a user or group and it will show up here.</li>
					</ul>
				</nav>
			</section>
		</main>

		@paletteV2()
	}
}
```

- [ ] **Step 3: Pinned fragment templ**

```go
// internal/web/templates/pinned_fragment.templ
package templates

templ pinnedFragment(pinned []PinnedEntry) {
	<ul class="home__list" data-pinned-list>
		if len(pinned) == 0 {
			<li class="home__list-empty">Nothing pinned. Open any detail page and click the star.</li>
		} else {
			for _, p := range pinned {
				<li class="home__list-item">
					<span class="home__list-type">{ p.Type }</span>
					<a class="home__list-link" href={ pinnedHref(p) }>{ p.CN }</a>
					<form action="/unpin" method="post"
						hx-post="/unpin"
						hx-target="#pinned-block"
						hx-swap="outerHTML">
						<input type="hidden" name="target" value={ p.DN }/>
						<button type="submit" class="icon-btn icon-btn--remove" aria-label={ "Unpin " + p.CN }>
							<span aria-hidden="true">★</span>
						</button>
					</form>
				</li>
			}
		}
	</ul>
}

func pinnedHref(p PinnedEntry) templ.SafeURL {
	switch p.Type {
	case "user":
		return templ.URL("/users/" + p.DN)
	case "group":
		return templ.URL("/groups/" + p.DN)
	case "computer":
		return templ.URL("/computers/" + p.DN)
	}
	return templ.URL("/")
}
```

- [ ] **Step 4: Append CSS**

Append to `internal/web/static/app.css`:

```css
/* ──────────────────────────── top nav ─────────────────────────────── */

.topnav {
    position: sticky; top: 0; z-index: 50;
    display: flex; align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    min-height: var(--density-touch-size);
}

.topnav__logo {
    font-weight: 600; letter-spacing: -0.02em;
    color: var(--fg);
    text-decoration: none;
}

.topnav__spacer { flex: 1; }

.topnav__cmdk {
    display: inline-flex; align-items: center; gap: 0.5rem;
    padding: 0.25rem 0.75rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    color: var(--fg-muted);
    cursor: pointer;
    font: inherit;
}

.topnav__cmdk-label { font-size: 0.875rem; }

.topnav__cmdk kbd {
    font: inherit; font-size: 0.75rem;
    padding: 0.1rem 0.35rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 0.25rem;
    color: var(--fg-muted);
}

.topnav-secondary {
    display: flex; gap: 1rem;
    padding: 0.25rem 1rem;
    border-bottom: 1px solid var(--border);
    background: var(--bg-subtle);
    font-size: 0.875rem;
}

.topnav-secondary__link {
    color: var(--fg-muted);
    text-decoration: none;
    padding: 0.25rem 0.5rem;
    border-radius: 0.25rem;
}

.topnav-secondary__link:hover,
.topnav-secondary__link:focus-visible {
    color: var(--fg);
    background: var(--bg);
}

.topnav-secondary__link--active {
    color: var(--fg);
    font-weight: 600;
}

/* ──────────────────────────── home ────────────────────────────────── */

.home {
    max-width: 48rem;
    margin: 2rem auto;
    padding: 0 1rem;
}

.home__greet {
    font-size: 1.75rem;
    letter-spacing: -0.02em;
    font-weight: 600;
    font-family: var(--font-heading);
    margin: 0 0 0.25rem;
}

.home__lede {
    color: var(--fg-muted);
    margin: 0 0 1.5rem;
}

.home__search {
    display: flex; align-items: center; gap: 0.75rem;
    width: 100%;
    padding: 0.875rem 1rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.75rem;
    color: var(--fg-muted);
    text-align: left;
    font: inherit;
    cursor: text;
    margin: 0 0 1.5rem;
}

.home__search kbd {
    margin-left: auto;
    font: inherit; font-size: 0.75rem;
    padding: 0.15rem 0.4rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 0.25rem;
    color: var(--fg-muted);
}

.home__blocks {
    display: grid; gap: 1.5rem;
    grid-template-columns: 1fr 1fr;
}

@media (max-width: 640px) {
    .home__blocks { grid-template-columns: 1fr; }
}

.home__heading {
    font-size: 0.75rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-muted);
    margin: 0 0 0.5rem;
    font-weight: 600;
}

.home__list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.125rem; }

.home__list-item {
    display: grid;
    grid-template-columns: auto 1fr auto;
    gap: 0.5rem;
    align-items: center;
    padding: 0.35rem 0.5rem;
    border-radius: 0.25rem;
}

.home__list-item:hover { background: var(--bg-subtle); }

.home__list-type {
    font-size: 0.625rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-muted);
    padding: 0.1rem 0.4rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.25rem;
}

.home__list-link { color: var(--fg); text-decoration: none; }
.home__list-link:hover,
.home__list-link:focus-visible { text-decoration: underline; }

.home__list-empty { color: var(--fg-muted); font-style: italic; padding: 0.5rem; }

.icon-btn--remove { color: var(--fg-muted); }
.icon-btn--remove:hover, .icon-btn--remove:focus-visible { color: var(--fg); }
```

- [ ] **Step 5: Regenerate + build**

```bash
rm -f internal/web/templates/*_templ.go
templ generate
go build ./...
go test ./internal/web/ -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/web/templates/topnav_v2.templ \
        internal/web/templates/home_v2.templ \
        internal/web/templates/pinned_fragment.templ \
        internal/web/static/app.css
git commit -S --signoff -m "feat(ui): add topnavV2, HomeV2, pinnedFragment templates + home CSS"
```

---

## Task 5 — Palette template + styles

**Files:**
- Create: `internal/web/templates/palette_v2.templ`
- Append: `internal/web/static/app.css`

- [ ] **Step 1: Palette templ**

```go
// internal/web/templates/palette_v2.templ
package templates

// paletteV2 is the ⌘K command palette shell. Body is a native <dialog>
// so keyboard focus trapping and Esc-to-close come from the browser.
// Result rendering and keyboard nav live in /static/js/v2-palette.js.
templ paletteV2() {
	<dialog id="cmd-palette" class="palette" aria-label="Command palette">
		<form method="dialog" class="palette__form">
			<div class="palette__input-row">
				<span aria-hidden="true" class="palette__icon">⌕</span>
				<input
					type="text"
					class="palette__input"
					data-palette-input
					placeholder="Search users, groups, computers…"
					aria-label="Search"
					autocomplete="off"
					spellcheck="false"
				/>
				<kbd class="palette__esc">esc</kbd>
			</div>
			<ul class="palette__results" role="listbox" aria-label="Results" data-palette-results></ul>
			<div class="palette__footer">
				<span><kbd>↵</kbd> open</span>
				<span><kbd>↑</kbd><kbd>↓</kbd> navigate</span>
				<span><kbd>esc</kbd> close</span>
			</div>
		</form>
	</dialog>
}
```

- [ ] **Step 2: Append palette CSS**

Append to `internal/web/static/app.css`:

```css
/* ──────────────────────────── palette ─────────────────────────────── */

.palette {
    width: min(36rem, 100vw - 2rem);
    max-height: min(28rem, 100vh - 4rem);
    padding: 0;
    border: 1px solid var(--border);
    border-radius: 0.75rem;
    background: var(--bg);
    color: var(--fg);
    box-shadow: 0 20px 60px rgb(0 0 0 / 0.15);
    margin-top: 3rem;
}

.palette::backdrop { background: rgb(0 0 0 / 0.35); }

.palette__form { display: flex; flex-direction: column; max-height: inherit; }

.palette__input-row {
    display: flex; align-items: center; gap: 0.625rem;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border);
}

.palette__icon { color: var(--fg-muted); }

.palette__input {
    flex: 1;
    background: transparent;
    color: var(--fg);
    border: 0;
    font: inherit; font-size: 1rem;
    outline: none;
}

.palette__esc {
    font: inherit; font-size: 0.75rem;
    padding: 0.1rem 0.4rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.25rem;
    color: var(--fg-muted);
}

.palette__results {
    list-style: none; margin: 0; padding: 0.25rem;
    overflow: auto; max-height: 18rem;
    display: flex; flex-direction: column; gap: 0.125rem;
}

.palette__item {
    display: grid;
    grid-template-columns: auto 1fr auto;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    border-radius: 0.375rem;
    cursor: pointer;
}

.palette__item[aria-selected="true"] {
    background: var(--fg);
    color: var(--bg);
}

.palette__item[aria-selected="true"] .palette__type {
    background: var(--bg-subtle); color: var(--fg);
}

.palette__type {
    font-size: 0.625rem; letter-spacing: 0.06em; text-transform: uppercase;
    padding: 0.1rem 0.4rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.25rem;
    color: var(--fg-muted);
    align-self: center;
}

.palette__ctx { color: var(--fg-muted); font-size: 0.8125rem; align-self: center; }

.palette__empty {
    padding: 1rem;
    color: var(--fg-muted);
    text-align: center;
}

.palette__footer {
    display: flex; gap: 1rem;
    padding: 0.5rem 1rem;
    border-top: 1px solid var(--border);
    color: var(--fg-muted);
    font-size: 0.75rem;
}

.palette__footer kbd {
    font: inherit; font-size: 0.6875rem;
    padding: 0.05rem 0.3rem;
    margin: 0 0.125rem;
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.15rem;
}
```

- [ ] **Step 3: Regenerate, build, commit**

```bash
rm -f internal/web/templates/palette_v2_templ.go
templ generate
go build ./...
git add internal/web/templates/palette_v2.templ internal/web/static/app.css
git commit -S --signoff -m "feat(ui): add command-palette <dialog> template + styles"
```

---

## Task 6 — Palette JS (fuzzy search + keyboard)

**Files:**
- Create: `internal/web/static/js/v2-palette.js`

**Security rule — do NOT use `.innerHTML =` or `.insertAdjacentHTML` with any dynamic value in this file.** All option rendering MUST use `createElement` + `textContent` + `appendChild`. The search-index entries contain strings derived from LDAP and must never be interpreted as HTML.

- [ ] **Step 1: Write the file**

```js
/*
 * Command palette — vanilla JS on top of <dialog>.
 *
 * Responsibilities:
 *   - Open via ⌘K / Ctrl+K / "/" / any [data-open-palette] click.
 *   - Close via Esc (dialog does this for free) or backdrop click.
 *   - Fetch /api/search-index.json on first open, cache in sessionStorage.
 *   - Fuzzy-match query against index on every keystroke (40ms debounced).
 *   - Keyboard: ↑/↓ change aria-selected, Enter navigates.
 *
 * CSP-safe — no inline code, no eval, no innerHTML with user content.
 */
(function () {
  "use strict";

  var dialog = document.getElementById("cmd-palette");
  if (!dialog) return;

  var input = dialog.querySelector("[data-palette-input]");
  var results = dialog.querySelector("[data-palette-results]");

  var INDEX_KEY = "ldap-manager:search-index:v1";
  var ETAG_KEY  = "ldap-manager:search-index-etag:v1";

  var index = null;
  var focused = -1;

  function openPalette() {
    if (dialog.open) return;
    if (typeof dialog.showModal === "function") dialog.showModal();
    else dialog.setAttribute("open", "");
    input.value = "";
    focused = -1;
    renderEmptyState("Type to search.");
    input.focus();
    loadIndex();
  }

  function closePalette() {
    if (!dialog.open) return;
    try { dialog.close(); } catch (_e) { dialog.removeAttribute("open"); }
  }

  function loadIndex() {
    if (index) return;
    var cachedIndex = null, cachedETag = null;
    try {
      cachedETag = sessionStorage.getItem(ETAG_KEY);
      var raw = sessionStorage.getItem(INDEX_KEY);
      if (raw) cachedIndex = JSON.parse(raw);
    } catch (_e) {}

    var headers = {};
    if (cachedETag) headers["If-None-Match"] = cachedETag;

    fetch("/api/search-index.json", { headers: headers, credentials: "same-origin" })
      .then(function (r) {
        if (r.status === 304 && cachedIndex) {
          index = cachedIndex;
          renderQuery(input.value);
          return;
        }
        if (!r.ok) throw new Error("search-index " + r.status);
        var etag = r.headers.get("ETag");
        return r.json().then(function (data) {
          index = data;
          try {
            sessionStorage.setItem(INDEX_KEY, JSON.stringify(data));
            if (etag) sessionStorage.setItem(ETAG_KEY, etag);
          } catch (_e) {}
          renderQuery(input.value);
        });
      })
      .catch(function (err) {
        console.error(err);
        renderEmptyState("Could not load search index.");
      });
  }

  // Score: lower = better; -1 = reject.
  function scoreEntry(q, entry) {
    if (!q) return 0;
    var qlc = q.toLowerCase();
    var name = (entry.cn || "").toLowerCase();
    var sam = (entry.sam || "").toLowerCase();
    var ou  = (entry.ou  || "").toLowerCase();

    if (name === qlc || sam === qlc) return 0;
    if (name.indexOf(qlc) === 0 || sam.indexOf(qlc) === 0) return 1;
    if (name.indexOf(qlc) >= 0 || sam.indexOf(qlc) >= 0) return 2;

    var initials = name.split(/\s+|[._-]/).map(function (w) { return w.charAt(0); }).join("");
    if (initials.indexOf(qlc) >= 0) return 3;

    if (ou.indexOf(qlc) >= 0) return 4;
    return -1;
  }

  function clearResults() {
    while (results.firstChild) results.removeChild(results.firstChild);
  }

  function renderEmptyState(message) {
    clearResults();
    var li = document.createElement("li");
    li.className = "palette__empty";
    li.textContent = message;
    results.appendChild(li);
    focused = -1;
  }

  // Build one result row using only safe DOM methods.
  function buildItem(entry, isFocused) {
    var li = document.createElement("li");
    li.className = "palette__item";
    li.setAttribute("role", "option");
    li.setAttribute("data-href", hrefFor(entry));
    li.setAttribute("aria-selected", isFocused ? "true" : "false");

    var type = document.createElement("span");
    type.className = "palette__type";
    type.textContent = entry.type;

    var name = document.createElement("span");
    var nameText = document.createElement("span");
    nameText.textContent = entry.cn;
    name.appendChild(nameText);
    if (entry.sam) {
      var sam = document.createElement("span");
      sam.className = "palette__ctx";
      sam.textContent = " (" + entry.sam + ")";
      name.appendChild(sam);
    }

    var ctx = document.createElement("span");
    ctx.className = "palette__ctx";
    ctx.textContent = entry.ou || "";

    li.appendChild(type);
    li.appendChild(name);
    li.appendChild(ctx);

    li.addEventListener("click", function () {
      var href = li.getAttribute("data-href");
      if (href) navigateTo(href);
    });
    return li;
  }

  function renderQuery(q) {
    if (!index) return;

    var matched = [];
    for (var i = 0; i < index.length; i++) {
      var s = scoreEntry(q, index[i]);
      if (s >= 0) matched.push({ s: s, e: index[i] });
    }
    matched.sort(function (a, b) {
      if (a.s !== b.s) return a.s - b.s;
      return a.e.cn.localeCompare(b.e.cn);
    });

    var top = matched.slice(0, 50);
    clearResults();

    if (top.length === 0) {
      renderEmptyState(q ? "No matches." : "Start typing.");
      return;
    }
    for (var j = 0; j < top.length; j++) {
      results.appendChild(buildItem(top[j].e, j === 0));
    }
    focused = 0;
  }

  function hrefFor(e) {
    var p = encodeURIComponent(e.dn);
    if (e.type === "user") return "/users/" + p;
    if (e.type === "group") return "/groups/" + p;
    if (e.type === "computer") return "/computers/" + p;
    return "/";
  }

  function navigateTo(href) {
    closePalette();
    window.location.href = href;
  }

  function moveFocus(delta) {
    var items = results.querySelectorAll("[role=option]");
    if (items.length === 0) return;
    focused = Math.max(0, Math.min(items.length - 1, focused + delta));
    for (var i = 0; i < items.length; i++) {
      items[i].setAttribute("aria-selected", i === focused ? "true" : "false");
    }
    items[focused].scrollIntoView({ block: "nearest" });
  }

  function enterFocused() {
    var items = results.querySelectorAll("[role=option]");
    if (focused < 0 || focused >= items.length) return;
    var href = items[focused].getAttribute("data-href");
    if (href) navigateTo(href);
  }

  // --- wire up ---
  document.addEventListener("click", function (ev) {
    var t = ev.target instanceof Element ? ev.target.closest("[data-open-palette]") : null;
    if (t) { ev.preventDefault(); openPalette(); }
  });

  document.addEventListener("keydown", function (ev) {
    var mod = ev.metaKey || ev.ctrlKey;
    if (mod && (ev.key === "k" || ev.key === "K" || ev.key === "/")) {
      ev.preventDefault();
      openPalette();
      return;
    }
    if (ev.key === "/" && !mod && !dialog.open) {
      var a = document.activeElement;
      var tag = a && a.tagName;
      if (tag !== "INPUT" && tag !== "TEXTAREA" && !(a && a.isContentEditable)) {
        ev.preventDefault();
        openPalette();
      }
    }
  });

  dialog.addEventListener("click", function (ev) {
    if (ev.target === dialog) closePalette();
  });

  var t = null;
  input.addEventListener("input", function () {
    if (t) clearTimeout(t);
    t = setTimeout(function () { renderQuery(input.value); }, 40);
  });

  input.addEventListener("keydown", function (ev) {
    if (ev.key === "ArrowDown") { ev.preventDefault(); moveFocus(1); return; }
    if (ev.key === "ArrowUp")   { ev.preventDefault(); moveFocus(-1); return; }
    if (ev.key === "Enter")     { ev.preventDefault(); enterFocused(); return; }
  });
})();
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/static/js/v2-palette.js
git commit -S --signoff -m "feat(ui): command-palette JS (fuzzy match, keyboard nav, index caching)"
```

---

## Task 7 — Recents JS (client-side)

**Files:**
- Create: `internal/web/static/js/v2-recents.js`

**Security rule — no innerHTML with dynamic values in this file either.**

- [ ] **Step 1: Write the file**

```js
/*
 * Recents — per-user localStorage ring buffer.
 *
 * Records: {type, dn, cn, lastSeenAt}
 * Key:     ldap-manager:recents:v1
 * Cap:     10, FIFO eviction.
 */
(function () {
  "use strict";

  var KEY = "ldap-manager:recents:v1";
  var LIMIT = 10;

  function read() {
    try { return JSON.parse(localStorage.getItem(KEY) || "[]"); }
    catch (_e) { return []; }
  }

  function write(arr) {
    try { localStorage.setItem(KEY, JSON.stringify(arr)); } catch (_e) {}
  }

  function push(entry) {
    if (!entry || !entry.dn || !entry.type || !entry.cn) return;
    var arr = read().filter(function (e) { return e.dn !== entry.dn; });
    arr.unshift({
      type: entry.type,
      dn: entry.dn,
      cn: entry.cn,
      lastSeenAt: new Date().toISOString()
    });
    if (arr.length > LIMIT) arr = arr.slice(0, LIMIT);
    write(arr);
  }

  function hrefFor(e) {
    var p = encodeURIComponent(e.dn);
    if (e.type === "user") return "/users/" + p;
    if (e.type === "group") return "/groups/" + p;
    if (e.type === "computer") return "/computers/" + p;
    return "/";
  }

  function render(container) {
    var list = container.querySelector("[data-recents-list]");
    var empty = container.querySelector("[data-recents-empty]");
    if (!list) return;

    var arr = read();
    if (arr.length === 0) return; // leave empty-message in place

    if (empty) empty.remove();
    while (list.firstChild) list.removeChild(list.firstChild);

    for (var i = 0; i < arr.length; i++) {
      var e = arr[i];
      var li = document.createElement("li");
      li.className = "home__list-item";

      var type = document.createElement("span");
      type.className = "home__list-type";
      type.textContent = e.type;

      var a = document.createElement("a");
      a.className = "home__list-link";
      a.href = hrefFor(e);
      a.textContent = e.cn;

      li.appendChild(type);
      li.appendChild(a);
      list.appendChild(li);
    }
  }

  window.ldapManagerPushRecent = push;

  var container = document.querySelector("[data-recents]");
  if (container) render(container);
})();
```

- [ ] **Step 2: Commit**

```bash
git add internal/web/static/js/v2-recents.js
git commit -S --signoff -m "feat(ui): client-side recents ring buffer + home-page renderer"
```

---

## Task 8 — Wire base_v2 + home handler + htmx

**Files:**
- Modify: `internal/web/templates/base_v2.templ` (add htmx + palette JS + recents JS)
- Create: `internal/web/home_handler.go`
- Modify: `internal/web/server.go` (register `/` → `handleHomeV2`)

- [ ] **Step 1: Update `base_v2.templ`**

Replace the single deferred script line in `<body>` with:

```go
			<script defer src="/static/vendor/htmx.min.js"></script>
			<script defer src="/static/js/v2-toggles.js"></script>
			<script defer src="/static/js/v2-palette.js"></script>
			<script defer src="/static/js/v2-recents.js"></script>
```

- [ ] **Step 2: Write the home handler**

```go
// internal/web/home_handler.go
package web

import (
	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// handleHomeV2 renders the signed-in home page (spec §6.6).
func (a *App) handleHomeV2(c *fiber.Ctx) error {
	sess, err := a.sessions.Get(c)
	if err != nil {
		return handle500(c, err)
	}
	userDN, _ := sess.Get("dn").(string)
	username, _ := sess.Get("username").(string)
	if userDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	pinned, _ := a.pinnedEntriesFor(userDN)

	cn := username
	if u, ok := a.lookupUserByDN(userDN); ok {
		cn = u.CN()
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.HomeV2(cn, pinned).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// pinnedEntriesFor hydrates DN strings from PinnedStore into PinnedEntry
// values via ldap_cache lookups. Missing targets are silently dropped.
func (a *App) pinnedEntriesFor(userDN string) ([]templates.PinnedEntry, error) {
	dns, err := a.pinnedStore.List(userDN)
	if err != nil {
		return nil, err
	}
	out := make([]templates.PinnedEntry, 0, len(dns))
	for _, dn := range dns {
		if u, ok := a.lookupUserByDN(dn); ok {
			out = append(out, templates.PinnedEntry{Type: "user", DN: dn, CN: u.CN()})
			continue
		}
		if g, ok := a.lookupGroupByDN(dn); ok {
			out = append(out, templates.PinnedEntry{Type: "group", DN: dn, CN: g.CN()})
			continue
		}
		if cp, ok := a.lookupComputerByDN(dn); ok {
			out = append(out, templates.PinnedEntry{Type: "computer", DN: dn, CN: cp.CN()})
			continue
		}
	}
	return out, nil
}

func (a *App) lookupUserByDN(dn string) (ldap.User, bool) {
	for _, u := range a.ldapCache.FindUsers() {
		if u.DN() == dn {
			return u, true
		}
	}
	return ldap.User{}, false
}

func (a *App) lookupGroupByDN(dn string) (ldap.Group, bool) {
	for _, g := range a.ldapCache.FindGroups() {
		if g.DN() == dn {
			return g, true
		}
	}
	return ldap.Group{}, false
}

func (a *App) lookupComputerByDN(dn string) (ldap.Computer, bool) {
	for _, cp := range a.ldapCache.FindComputers() {
		if cp.DN() == dn {
			return cp, true
		}
	}
	return ldap.Computer{}, false
}
```

- [ ] **Step 3: Swap the `/` route**

In `internal/web/server.go`, find the route `app.Get("/", ...)` and replace the handler with `a.handleHomeV2`. The previous handler may call a function that's still used by legacy users/groups/computers pages — leave that function alone; just change the `/` registration.

- [ ] **Step 4: Build + test**

```bash
rm -f internal/web/templates/*_templ.go
templ generate
go build ./...
go test ./internal/web/ -count=1
```

Some existing tests may fail if they assert on the old Index markup. Classify per the prior plan's Task 2.3 guidance (Class A assertions updated to match new markup; Class B behavior assertions preserved).

- [ ] **Step 5: Commit**

```bash
git add internal/web/home_handler.go \
        internal/web/server.go \
        internal/web/templates/base_v2.templ
# and any test files touched
git commit -S --signoff -m "feat(ui): swap / to HomeV2 handler; load htmx + palette + recents JS"
```

---

## Task 9 — E2E: HomeV2 visibility + AAA + palette opens

**Files:**
- Create: `internal/e2e/home_v2_test.go`

- [ ] **Step 1: Write the test**

```go
//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHomeV2_VisibleAndAAA verifies the signed-in home page renders
// visibly, is AAA-clean per axe-core, and that ⌘K opens the palette.
func TestHomeV2_VisibleAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser(), "sign in as seeded test user")

	tp.Navigate("/")

	// Visibility: greeting has a non-zero bounding box.
	box, err := page.Locator("h1.home__greet").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, box)
	assert.Greater(t, box.Height, 0.0)

	// AAA axe pass (reuse Evaluate-based injection pattern from axe_test.go).
	axePath, _ := filepath.Abs("testdata/axe.min.js")
	axeSrc, err := os.ReadFile(axePath)
	require.NoError(t, err)

	_, err = page.Evaluate(string(axeSrc))
	require.NoError(t, err, "inject axe")

	raw, err := page.Evaluate(`
		() => axe.run({
			runOnly: { type: 'tag', values: ['wcag2a','wcag2aa','wcag2aaa','wcag21a','wcag21aa','wcag22aa'] },
			resultTypes: ['violations'],
		})
	`)
	require.NoError(t, err)

	b, _ := json.Marshal(raw)
	var ar struct {
		Violations []struct {
			ID, Description string
			Nodes           []struct{ Target []string }
		} `json:"violations"`
	}
	require.NoError(t, json.Unmarshal(b, &ar))

	if len(ar.Violations) > 0 {
		for _, v := range ar.Violations {
			t.Errorf("axe violation [%s]: %s (%d nodes)", v.ID, v.Description, len(v.Nodes))
			for _, n := range v.Nodes {
				t.Logf("  %v", n.Target)
			}
		}
		t.FailNow()
	}

	// Palette opens on ⌘K (use Control+K on Linux CI where Meta isn't bound).
	require.NoError(t, page.Keyboard().Press("Control+k"))
	_ = playwright.String // keep import used

	openAttr, err := page.Evaluate(`document.getElementById('cmd-palette').hasAttribute('open')`)
	require.NoError(t, err)
	assert.Equal(t, true, openAttr, "palette dialog must open on Ctrl+K")
}
```

- [ ] **Step 2: Run — expect PASS**

```bash
go test -tags e2e ./internal/e2e/ -run TestHomeV2_VisibleAndAAA -v
```

If AAA violations surface, fix them in `app.css` / templates and iterate until clean.

- [ ] **Step 3: Commit**

```bash
git add internal/e2e/home_v2_test.go
git commit -S --signoff -m "test(e2e): HomeV2 visible, AAA-clean, palette opens on Ctrl+K"
```

---

## Task 10 — Wrap-up

- [ ] **Step 1: Full suite**

```bash
make lint-go
go test ./... -count=1
go test -tags e2e ./internal/e2e/ -count=1
```

Expected: green (vuln-check pre-existing Go stdlib issue unchanged).

- [ ] **Step 2: Manual spot-check** *(optional, if preview is still running)*

Rebuild and restart the preview per the Slice 2 instructions, then browser-check:
- Home renders; greeting uses the signed-in CN.
- Top nav: logo · ⌘K hint · theme · density · logout.
- Secondary row: Users · Groups · Computers.
- Empty states show for Pinned + Recent.
- ⌘K opens palette; typing filters; Enter navigates; Esc closes.

---

## Self-review notes

- **Security — no innerHTML with dynamic content.** All JS DOM construction uses `createElement` + `textContent` + `appendChild`. LDAP-derived strings never reach an HTML parser.
- **Palette Enter is a full navigation** (`window.location.href = href`) — lists are still old-stack; drawer-style in-page swap arrives in Slice 4.
- **Orphan-pin handling:** `pinnedEntriesFor` silently drops DNs no longer in the cache. Acceptable for Phase 1.
- **Native `<dialog>` handles focus trapping + Esc.**
- **Type consistency:** `PinnedEntry.Type` and `SearchIndexEntry.Type` both use `"user" | "group" | "computer"`. Palette + recents use the same strings. Do not drift.
- **No new Go deps.** `go.mod` unchanged.
- **Spec coverage:** §5 IA — Tasks 4, 8. §6.1 palette index — Tasks 2, 6. §6.4 recents — Task 7. §6.5 pinned — Tasks 1, 3. §6.6 home — Tasks 4, 8. §7 AAA — Task 9.
