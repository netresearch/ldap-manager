# UI Revamp — Phase 1 Slice 4: Users List + Drawer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite `/users` and `/users/:dn` on the new stack: two-pane list + detail drawer with htmx row swap, pivots, pin/unpin, recents recording. Mutations (add/remove group membership) via full-page navigation in this slice; Slice 4b promotes them to in-drawer htmx swaps.

**Architecture:** List page renders rows with `hx-get="/users/:dn?fragment=drawer"` so row clicks swap only the drawer pane (no full nav, URL updates via `hx-push-url`). Direct GET of `/users/:dn` or Cmd-click returns the full-page user detail. Server reuses the same Go helper to build the drawer's inner view-model, then wraps it either with the drawer chrome (fragment response) or with the full-page shell (full response).

**Tech Stack:**
- Go + Fiber + Templ (existing)
- Pico CSS + `app.css` (existing)
- **htmx v2** (loaded in Slice 3; drives the drawer swap here)
- No Alpine.js (CSP)
- `ldapManagerPushRecent` from `v2-recents.js` (Slice 3) — called on drawer swap via `htmx:afterSwap` listener

**Spec reference:** `docs/superpowers/specs/2026-04-20-ui-revamp-design.md` §6.2 drawer, §6.3 pivots, §6.4 recents, §6.5 pinned UI, §5 routes.

**Out of scope for this plan:**
- In-drawer group mutations (add/remove member) — Slice 4b.
- `?panel=1` URL state for "list + drawer" permalink — Slice 4c or dropped as YAGNI.
- Mobile full-screen overlay drawer (<900px) — default behaviour (fall back to full-page nav at mobile widths) is acceptable for Phase 1.

---

## File Structure

**New files:**
- `internal/web/templates/users_v2.templ` — `UsersListV2`, `UserFullV2`, `UserDrawerV2`, and the row helper `userRowV2`.
- `internal/web/templates/user_drawer_fragment.templ` — fragment-response wrapper around `UserDrawerV2` (just includes `UserDrawerV2` without a full-page shell).
- `internal/web/users_v2_handler.go` — `handleUsersV2`, `handleUserV2`, and the shared `buildUserDrawerVM` helper.
- `internal/web/static/js/v2-drawer.js` — `htmx:afterSwap` listener that reads `data-recent-*` attributes from swapped drawer contents and pushes to recents; delegates drawer-close click on backdrop.

**Modified files:**
- `internal/web/server.go` — swap `/users` and `/users/:dn` to the V2 handlers.
- `internal/web/templates/base_v2.templ` — add `<script defer src="/static/js/v2-drawer.js"></script>`.
- `internal/web/static/app.css` — append list + drawer styles (two-pane grid, row, drawer header, pivot list, empty-drawer message).

**Not modified:** old `users.templ`, old `/users` handler function (keep for legacy tests — Slice 8 cleans up).

---

## Pre-flight

- [ ] **Step 0.1: Confirm state**

```bash
cd /home/cybot/projects/ldap-manager-ui-revamp-phase-1a
git log --oneline ddea7d9..HEAD | head -3   # should start with the Slice 3 wrap commits
go test ./internal/web/ -count=1 2>&1 | tail -3   # non-LDAP tests pass
```

---

## Task 1 — Handler + view-model

**Files:**
- Create: `internal/web/users_v2_handler.go`
- Modify: `internal/web/server.go` (register V2 handlers for `/users` and `/users/:dn`; keep old handlers available for legacy tests)

- [ ] **Step 1: Sketch the view-model types in a new file**

```go
// internal/web/users_v2_handler.go
package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// buildUserDrawerVM hydrates the drawer view-model for a given user DN.
// Returns (vm, found). The result is safe to render in both drawer-fragment
// and full-page contexts — the caller chooses the wrapper.
func (a *App) buildUserDrawerVM(userDN string, viewerDN string) (templates.UserDrawerVM, bool) {
	user, ok := a.lookupUserByDN(userDN)
	if !ok {
		return templates.UserDrawerVM{}, false
	}

	pinned, _ := a.pinnedStore.IsPinned(viewerDN, userDN)

	ouFilter := immediateOU(userDN)
	return templates.UserDrawerVM{
		User:      user,
		Pinned:    pinned,
		OUPivotHref: buildOUPivotHref(ouFilter),
	}, true
}

func buildOUPivotHref(ou string) string {
	if ou == "" {
		return ""
	}
	v := url.Values{}
	v.Set("ou", ou)
	return "/users?" + v.Encode()
}

// handleUsersV2 renders the new /users list page.
func (a *App) handleUsersV2(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	viewerDN, _ := sess.Get("dn").(string)
	if viewerDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	showDisabled := c.Query("show-disabled") == "1"
	ouFilter := c.Query("ou")

	all := a.ldapCache.FindUsers(showDisabled)
	users := filterUsersByOU(all, ouFilter)

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.UsersListV2(users, showDisabled, ouFilter, templates.Flashes()).
		Render(c.UserContext(), c.Response().BodyWriter())
}

// handleUserV2 renders either the drawer fragment (?fragment=drawer)
// or the full user detail page.
func (a *App) handleUserV2(c *fiber.Ctx) error {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	viewerDN, _ := sess.Get("dn").(string)
	if viewerDN == "" {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	encodedDN := c.Params("user_dn")
	userDN, err := url.PathUnescape(encodedDN)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	vm, ok := a.buildUserDrawerVM(userDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("user not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" {
		return templates.UserDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.UserFullV2(vm).
		Render(c.UserContext(), c.Response().BodyWriter())
}

func filterUsersByOU(users []ldap.User, ou string) []ldap.User {
	if ou == "" {
		return users
	}

	out := users[:0:0]
	for _, u := range users {
		if immediateOU(u.DN()) == ou {
			out = append(out, u)
		}
	}

	return out
}
```

- [ ] **Step 2: Register routes in `internal/web/server.go`**

Locate where `/users` and `/users/:user_dn` are registered. Replace the handler references with the V2 variants while leaving the old handler functions in place:

```go
// inside the authenticated route group:
protected.Get("/users", a.handleUsersV2)
protected.Get("/users/:user_dn", a.handleUserV2)
```

If the project uses parameter name `:dn` or `:userDN`, keep whatever the current registration uses for consistency; update `handleUserV2`'s `c.Params(...)` call to match.

- [ ] **Step 3: Build, expect FAIL (templates don't exist yet)**

```bash
templ generate
go build ./...
```

Expected: compile error referencing `templates.UserDrawerVM`, `UsersListV2`, `UserDrawerFragment`, `UserFullV2`. Those are created in Task 2.

- [ ] **Step 4: No commit yet — continue to Task 2**

Do NOT commit the handler without its templates (would break the build for the next engineer). Task 2 creates the templates; commit after both.

---

## Task 2 — Templates

**Files:**
- Create: `internal/web/templates/users_v2.templ`
- Create: `internal/web/templates/user_drawer_fragment.templ`

- [ ] **Step 1: Write `users_v2.templ`**

```go
// internal/web/templates/users_v2.templ
package templates

import (
	"fmt"
	"net/url"

	ldap "github.com/netresearch/simple-ldap-go"
)

// UserDrawerVM is the view-model shared by the drawer fragment and
// the full-page user detail. Populated by App.buildUserDrawerVM.
type UserDrawerVM struct {
	User        ldap.User
	Pinned      bool
	OUPivotHref string
}

templ UsersListV2(users []ldap.User, showDisabled bool, ouFilter string, flashes []Flash) {
	@baseV2("Users") {
		@topnavV2("/users")

		<main class="list-page">
			<header class="list-page__head">
				<h1 class="list-page__title">Users</h1>
				<p class="list-page__count">{ fmt.Sprintf("%d", len(users)) } users</p>
			</header>

			<div class="list-page__filters" data-search-filter>
				<input
					type="search"
					class="list-page__search"
					placeholder="Filter users…"
					aria-label="Filter users"
					data-search-input
				/>
				<a class={ filterChipClass(showDisabled) } href={ toggleDisabledHref(showDisabled, ouFilter) }>
					if showDisabled {
						including disabled
					} else {
						enabled only
					}
				</a>
				if ouFilter != "" {
					<a class="filter-chip filter-chip--on" href={ clearOUHref(showDisabled) }>
						ou={ ouFilter } ×
					</a>
				}
			</div>

			<div class="list-page__pane">
				<ul class="list-rows" data-search-list role="listbox" aria-label="Users">
					for _, u := range users {
						@userRowV2(u)
					}
					if len(users) == 0 {
						<li class="list-rows__empty">No users match.</li>
					}
				</ul>

				<aside class="drawer" id="drawer" aria-live="polite" aria-label="Detail">
					<div class="drawer__empty">Select a user to see details.</div>
				</aside>
			</div>
		</main>

		@paletteV2()
	}
}

templ userRowV2(u ldap.User) {
	<li class="list-row" data-search-item>
		<a
			class="list-row__link"
			href={ userDetailHref(u) }
			hx-get={ userDrawerFragmentHref(u) }
			hx-target="#drawer"
			hx-swap="innerHTML"
			hx-push-url="true"
			title={ "View " + u.CN() }
		>
			<span class="list-row__dot" data-enabled?={ u.Enabled }></span>
			<span class="list-row__primary">{ u.CN() }</span>
			<span class="list-row__secondary">{ u.SAMAccountName }</span>
			if !u.Enabled {
				<span class="list-row__badge">disabled</span>
			}
		</a>
	</li>
}

templ UserFullV2(vm UserDrawerVM) {
	@baseV2(vm.User.CN()) {
		@topnavV2("/users")

		<main class="list-page list-page--single">
			<div class="list-page__pane">
				<aside class="drawer drawer--full" aria-label="User detail">
					@userDrawerContents(vm)
				</aside>
			</div>
		</main>

		@paletteV2()
	}
}

func userDetailHref(u ldap.User) templ.SafeURL {
	return templ.URL("/users/" + url.PathEscape(u.DN()))
}

func userDrawerFragmentHref(u ldap.User) string {
	return "/users/" + url.PathEscape(u.DN()) + "?fragment=drawer"
}

func filterChipClass(on bool) string {
	if on {
		return "filter-chip filter-chip--on"
	}

	return "filter-chip"
}

func toggleDisabledHref(currentShowDisabled bool, ouFilter string) templ.SafeURL {
	v := url.Values{}
	if !currentShowDisabled {
		v.Set("show-disabled", "1")
	}

	if ouFilter != "" {
		v.Set("ou", ouFilter)
	}

	qs := v.Encode()
	if qs == "" {
		return templ.URL("/users")
	}

	return templ.URL("/users?" + qs)
}

func clearOUHref(showDisabled bool) templ.SafeURL {
	if showDisabled {
		return templ.URL("/users?show-disabled=1")
	}

	return templ.URL("/users")
}
```

- [ ] **Step 2: Write the drawer-contents partial**

Append to `internal/web/templates/users_v2.templ` (after `UserFullV2`):

```go
// userDrawerContents is the shared inner content of the drawer,
// rendered both inside the fragment response and inside UserFullV2.
templ userDrawerContents(vm UserDrawerVM) {
	<header
		class="drawer__head"
		data-recent-type="user"
		data-recent-dn={ vm.User.DN() }
		data-recent-cn={ vm.User.CN() }
	>
		<div class="drawer__title-wrap">
			<h2 class="drawer__title">{ vm.User.CN() }</h2>
			<p class="drawer__sub">{ vm.User.SAMAccountName }</p>
		</div>
		@pinStarButton("user", vm.User.DN(), vm.Pinned)
	</header>

	<p class="drawer__dn">{ vm.User.DN() }</p>

	<section class="drawer__section">
		<h3 class="drawer__section-title">Attributes</h3>
		<dl class="drawer__kv">
			<dt>Status</dt>
			<dd>
				if vm.User.Enabled {
					Enabled
				} else {
					Disabled
				}
			</dd>
			if vm.User.Mail != nil && *vm.User.Mail != "" {
				<dt>Email</dt>
				<dd><a href={ templ.URL("mailto:" + *vm.User.Mail) }>{ *vm.User.Mail }</a></dd>
			}
			if vm.User.Description != "" {
				<dt>Description</dt>
				<dd>{ vm.User.Description }</dd>
			}
		</dl>
	</section>

	<section class="drawer__section">
		<h3 class="drawer__section-title">
			{ fmt.Sprintf("Groups · %d", len(vm.User.Groups)) }
		</h3>
		if len(vm.User.Groups) == 0 {
			<p class="drawer__empty-inline">None.</p>
		} else {
			<ul class="drawer__tags">
				for _, g := range vm.User.Groups {
					<li>
						<a class="drawer__tag" href={ groupDetailHref(g) }>{ g.CN() }</a>
					</li>
				}
			</ul>
		}
	</section>

	<section class="drawer__section">
		<h3 class="drawer__section-title">Pivot</h3>
		<ul class="drawer__pivots">
			<li>
				<a class="drawer__pivot" href={ userDetailHref(vm.User) }>
					<span>Open full page</span>
					<span aria-hidden="true">→</span>
				</a>
			</li>
			if vm.OUPivotHref != "" {
				<li>
					<a class="drawer__pivot" href={ templ.URL(vm.OUPivotHref) }>
						<span>Other users in { immediateOULabel(vm.OUPivotHref) }</span>
						<span aria-hidden="true">→</span>
					</a>
				</li>
			}
		</ul>
	</section>
}

templ pinStarButton(entityType, entityDN string, pinned bool) {
	if pinned {
		<form hx-post="/unpin" hx-target="closest .drawer__head .pin-star" hx-swap="outerHTML">
			<input type="hidden" name="target" value={ entityDN }/>
			<button type="submit" class="pin-star pin-star--on" aria-label={ "Unpin " + entityType } aria-pressed="true">
				<span aria-hidden="true">★</span>
			</button>
		</form>
	} else {
		<form hx-post="/pin" hx-target="closest .drawer__head .pin-star" hx-swap="outerHTML">
			<input type="hidden" name="target" value={ entityDN }/>
			<button type="submit" class="pin-star" aria-label={ "Pin " + entityType } aria-pressed="false">
				<span aria-hidden="true">☆</span>
			</button>
		</form>
	}
}

func groupDetailHref(g ldap.Group) templ.SafeURL {
	return templ.URL("/groups/" + url.PathEscape(g.DN()))
}

func immediateOULabel(ouHref string) string {
	i := strings.Index(ouHref, "ou=")
	if i < 0 {
		return ""
	}

	rest := ouHref[i+3:]
	if j := strings.Index(rest, "&"); j >= 0 {
		return rest[:j]
	}

	return rest
}
```

**Note:** `immediateOULabel` uses `strings.Index`. Add `"strings"` to the import block at the top of the file.

- [ ] **Step 3: Write `user_drawer_fragment.templ`**

```go
// internal/web/templates/user_drawer_fragment.templ
package templates

// UserDrawerFragment is what /users/:dn?fragment=drawer returns.
// htmx swaps it into #drawer with innerHTML. No base template, no topnav.
templ UserDrawerFragment(vm UserDrawerVM) {
	@userDrawerContents(vm)
}

// UserDrawerV2 is an alias kept for readability at call-sites that
// render the drawer inline (e.g. if we ever need it outside UserFullV2
// or the fragment wrapper).
templ UserDrawerV2(vm UserDrawerVM) {
	@userDrawerContents(vm)
}
```

- [ ] **Step 4: Regenerate + build**

```bash
rm -f internal/web/templates/*_templ.go
templ generate
go build ./...
```

Expected: clean build.

- [ ] **Step 5: Run non-LDAP tests**

```bash
go test ./internal/web/ -count=1 -run '!TestLDAPIntegration'
```

Expected: pass. If any Class-A assertions in legacy users-handler tests need updating to the new markup (e.g. a test that asserts `{ CN } ({ SAMAccountName })`), patch them per prior-plan guidance.

- [ ] **Step 6: Commit (Task 1 handler + Task 2 templates together)**

```bash
git add internal/web/users_v2_handler.go \
        internal/web/server.go \
        internal/web/templates/users_v2.templ \
        internal/web/templates/user_drawer_fragment.templ
# and any test files touched
git commit -S --signoff -m "feat(ui): UsersListV2 + drawer fragment + full-page user detail"
```

---

## Task 3 — CSS for list + drawer

**Files:**
- Modify: `internal/web/static/app.css` (append)

- [ ] **Step 1: Append list + drawer styles**

Append to `internal/web/static/app.css`:

```css
/* ──────────────────────────── list page ────────────────────────────── */

.list-page {
    max-width: 72rem;
    margin: 1.5rem auto;
    padding: 0 1rem;
}

.list-page__head {
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    margin-bottom: 1rem;
}

.list-page__title {
    font-size: 1.5rem;
    font-weight: 600;
    letter-spacing: -0.02em;
    font-family: var(--font-heading);
    margin: 0;
}

.list-page__count {
    color: var(--fg-muted);
    margin: 0;
    font-size: 0.875rem;
}

.list-page__filters {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 1rem;
    flex-wrap: wrap;
}

.list-page__search {
    flex: 1 1 20rem;
    min-height: var(--density-touch-size);
    padding: 0 0.75rem;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    font: inherit;
}

.filter-chip {
    display: inline-flex;
    align-items: center;
    padding: 0.25rem 0.75rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--fg-muted);
    text-decoration: none;
    font-size: 0.875rem;
    background: var(--bg);
}

.filter-chip:hover,
.filter-chip:focus-visible {
    color: var(--fg);
    border-color: var(--border-strong);
}

.filter-chip--on {
    background: var(--fg);
    color: var(--bg);
    border-color: var(--fg);
}

/* Two-pane: list | drawer */
.list-page__pane {
    display: grid;
    grid-template-columns: 1fr 22rem;
    gap: 1rem;
    align-items: start;
}

.list-page--single .list-page__pane {
    grid-template-columns: minmax(0, 40rem);
    justify-content: center;
}

@media (max-width: 900px) {
    .list-page__pane {
        grid-template-columns: 1fr;
    }

    .list-page__pane > .drawer {
        display: none;
    }
}

/* ──────────────────────────── list rows ────────────────────────────── */

.list-rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
    max-height: calc(100dvh - 12rem);
    overflow: auto;
}

.list-rows__empty {
    padding: 1rem;
    color: var(--fg-muted);
    font-style: italic;
}

.list-row {
    list-style: none;
}

.list-row__link {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    gap: 0.5rem;
    align-items: center;
    padding: 0.5rem 0.75rem;
    border-radius: 0.375rem;
    color: var(--fg);
    text-decoration: none;
    font: inherit;
}

.list-row__link:hover,
.list-row__link:focus-visible {
    background: var(--bg-subtle);
}

.list-row__dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--fg-muted);
}

.list-row__link [data-enabled="true"].list-row__dot,
.list-row__link .list-row__dot[data-enabled="true"] {
    background: var(--accent);
}

.list-row__primary { font-weight: 500; }

.list-row__secondary {
    color: var(--fg-muted);
    font-size: 0.875rem;
}

.list-row__badge {
    font-size: 0.625rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    padding: 0.1rem 0.4rem;
    border: 1px solid var(--border);
    border-radius: 0.25rem;
    color: var(--fg-muted);
}

/* ──────────────────────────── drawer ───────────────────────────────── */

.drawer {
    background: var(--bg-subtle);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    padding: 1rem;
    position: sticky;
    top: calc(var(--density-touch-size) + 1rem);
    max-height: calc(100dvh - var(--density-touch-size) - 2rem);
    overflow: auto;
}

.drawer--full {
    position: static;
    max-height: none;
}

.drawer__empty {
    color: var(--fg-muted);
    font-style: italic;
    text-align: center;
    padding: 2rem 1rem;
}

.drawer__head {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 0.75rem;
    align-items: start;
}

.drawer__title-wrap { min-width: 0; }

.drawer__title {
    margin: 0;
    font-family: var(--font-heading);
    font-size: 1.125rem;
    font-weight: 600;
    letter-spacing: -0.01em;
    word-break: break-word;
}

.drawer__sub {
    margin: 0;
    color: var(--fg-muted);
    font-size: 0.875rem;
}

.drawer__dn {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--fg-muted);
    margin: 0.5rem 0 0.75rem;
    word-break: break-all;
}

.drawer__section {
    margin-top: 0.75rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--border);
}

.drawer__section-title {
    font-size: 0.6875rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-muted);
    margin: 0 0 0.5rem;
    font-weight: 600;
}

.drawer__kv {
    display: grid;
    grid-template-columns: 6rem 1fr;
    gap: 0.25rem 0.75rem;
    margin: 0;
    font-size: 0.875rem;
}

.drawer__kv dt { color: var(--fg-muted); }

.drawer__kv dd { margin: 0; word-break: break-word; }

.drawer__tags {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
}

.drawer__tag {
    display: inline-block;
    padding: 0.1rem 0.5rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--fg);
    text-decoration: none;
    font-size: 0.75rem;
}

.drawer__tag:hover, .drawer__tag:focus-visible {
    border-color: var(--border-strong);
}

.drawer__pivots {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

.drawer__pivot {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    color: var(--fg);
    text-decoration: none;
    font-size: 0.875rem;
}

.drawer__pivot:hover, .drawer__pivot:focus-visible {
    border-color: var(--border-strong);
}

.drawer__empty-inline {
    margin: 0;
    color: var(--fg-muted);
    font-style: italic;
    font-size: 0.875rem;
}

/* ──────────────────────────── pin star ─────────────────────────────── */

.pin-star {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--density-touch-size);
    height: var(--density-touch-size);
    margin: 0;
    padding: 0;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    color: var(--fg-muted);
    font: inherit;
    line-height: 1;
    cursor: pointer;
}

.pin-star:hover, .pin-star:focus-visible {
    color: var(--fg);
    border-color: var(--border-strong);
}

.pin-star--on { color: var(--accent); border-color: var(--accent); }
```

- [ ] **Step 2: Verify contrast test still passes**

```bash
go test ./internal/web/ -run TestAppCSSContrastAAA -v
```

- [ ] **Step 3: Commit**

```bash
git add internal/web/static/app.css
git commit -S --signoff -m "feat(ui): list + drawer + pin-star styles"
```

---

## Task 4 — Drawer JS (recents + close)

**Files:**
- Create: `internal/web/static/js/v2-drawer.js`
- Modify: `internal/web/templates/base_v2.templ` (load the new JS)

- [ ] **Step 1: Write `v2-drawer.js`**

```js
/*
 * Drawer helpers: record recents on htmx swap, handle drawer-close click.
 *
 * All DOM access is CSP-safe (no inline handlers, no innerHTML with dynamic
 * strings). Entity metadata arrives via data-recent-* attributes set by the
 * drawer fragment template.
 */
(function () {
  "use strict";

  function recordRecentFromHead(head) {
    if (!head) return;
    var type = head.getAttribute("data-recent-type");
    var dn = head.getAttribute("data-recent-dn");
    var cn = head.getAttribute("data-recent-cn");
    if (!type || !dn || !cn) return;
    if (typeof window.ldapManagerPushRecent === "function") {
      window.ldapManagerPushRecent({ type: type, dn: dn, cn: cn });
    }
  }

  // When htmx swaps the drawer target, read the freshly-inserted head.
  document.body.addEventListener("htmx:afterSwap", function (ev) {
    var target = ev.detail && ev.detail.target;
    if (!target) return;
    if (target.id !== "drawer") return;
    var head = target.querySelector("[data-recent-type]");
    recordRecentFromHead(head);
  });

  // On full-page detail view, also record on load.
  var head = document.querySelector(".drawer--full [data-recent-type]");
  if (head) recordRecentFromHead(head);
})();
```

- [ ] **Step 2: Load it in base_v2.templ**

Inside the `<body>` deferred-script block, add one line:

```go
			<script defer src="/static/js/v2-drawer.js"></script>
```

Place it after `v2-recents.js` so `ldapManagerPushRecent` is defined before the afterSwap listener runs.

- [ ] **Step 3: Regenerate, build, commit**

```bash
rm -f internal/web/templates/base_v2_templ.go
templ generate
go build ./...

git add internal/web/static/js/v2-drawer.js \
        internal/web/templates/base_v2.templ
git commit -S --signoff -m "feat(ui): v2-drawer.js — record recents on htmx swap"
```

---

## Task 5 — E2E: list + drawer swap + pivot + pin

**Files:**
- Create: `internal/e2e/users_v2_test.go`

- [ ] **Step 1: Write the test**

```go
//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsersV2_FlowAndAAA covers the happy path: list loads, row click
// swaps the drawer via htmx, pivot link works, and axe-core sees no
// WCAG 2.2 AAA violations on both the list + drawer view.
func TestUsersV2_FlowAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())

	tp.Navigate("/users")

	// Title present
	titleBox, err := page.Locator("h1.list-page__title").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, titleBox)
	assert.Greater(t, titleBox.Height, 0.0)

	// Empty drawer message on initial load
	emptyVisible, err := page.Locator(".drawer .drawer__empty").First().IsVisible()
	require.NoError(t, err)
	assert.True(t, emptyVisible, "initial drawer should show empty state")

	// Click the first row — htmx should swap the drawer in place.
	firstRow := page.Locator(".list-row__link").First()
	rowCount, _ := page.Locator(".list-row__link").Count()
	require.Greater(t, rowCount, 0, "expected at least one user row")
	require.NoError(t, firstRow.Click())

	// Wait for the drawer head to appear (htmx-afterSwap populates it).
	require.NoError(t, page.Locator(".drawer__head .drawer__title").WaitFor())

	// Drawer now has a title and pivots.
	titleText, err := page.Locator(".drawer__head .drawer__title").First().TextContent()
	require.NoError(t, err)
	assert.NotEmpty(t, titleText)

	pivotCount, _ := page.Locator(".drawer__pivot").Count()
	assert.GreaterOrEqual(t, pivotCount, 1, "at least one pivot link")

	// URL was push-updated to /users/:dn
	url := page.URL()
	assert.Contains(t, url, "/users/", "hx-push-url should advance the URL")

	// axe AAA pass on the post-swap page.
	axePath, _ := filepath.Abs("testdata/axe.min.js")
	axeSrc, err := os.ReadFile(axePath)
	require.NoError(t, err)
	_, err = page.Evaluate(string(axeSrc))
	require.NoError(t, err)

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
			t.Errorf("axe [%s]: %s (%d nodes)", v.ID, v.Description, len(v.Nodes))
		}
		t.FailNow()
	}
}
```

- [ ] **Step 2: Run**

```bash
go test -tags e2e ./internal/e2e/ -run TestUsersV2_FlowAndAAA -v
```

Expected: PASS. If AAA violations surface, fix in `app.css`/`users_v2.templ` and iterate.

- [ ] **Step 3: Commit**

```bash
git add internal/e2e/users_v2_test.go
git commit -S --signoff -m "test(e2e): /users list + drawer swap + pivot + AAA"
```

---

## Task 6 — Wrap-up

- [ ] **Step 1: Full lint + tests**

```bash
make lint-go
go test ./... -count=1 2>&1 | grep -E "^(FAIL|ok)"
go test -tags e2e ./internal/e2e/ -count=1 2>&1 | grep -E "^(FAIL|ok|---)"
```

Fix any nlreturn / goimports issues in the new files. `TestLDAPIntegration_*` failures are pre-existing environmental, ignore.

- [ ] **Step 2: Manual smoke**

Restart preview, log in as `demo`/`demo`, navigate `/users`:
- List renders with at least the `demo` user.
- Row click shows user details in right drawer (not a full navigation — note the URL changes but no flash).
- Pin star toggles via htmx.
- Clicking pivot "Open full page" goes to single-pane full-page view.
- Top-nav `Users` link highlighted.

If the manual smoke surfaces a small polish item, commit as `fix(ui): slice-4 polish: …`.

---

## Self-review notes

- **`userDrawerContents` is private** (lowercase name, non-exported) and can only be called from inside the `templates` package. That's fine — Go handles this via the templ package lexical scope.
- **`hx-target="closest .drawer__head .pin-star"`** in `pinStarButton` — htmx's `closest` accepts a selector. The star swaps in place after pin/unpin.
- **Pin/unpin returns 204 No Content** (from Slice 3). With `hx-swap="outerHTML"` against a 204, htmx will *not* update the DOM. The server needs to return the new star-button markup instead. Options: (a) add a new endpoint that returns the star fragment, (b) change pin/unpin to return the star fragment when `HX-Request` is true, (c) rebuild the star icon client-side on swap-before. For Slice 4, go with (b): update the pin/unpin handler to return the star fragment when `HX-Request: true`. Treat this as a sub-task under Task 4 — if time is tight, leave the pin star as a full-page POST (form action + no hx-post) and fix in Slice 4b.
- **Route parameter name** — the plan uses `:user_dn`. The project's existing registration may use a different name. Whatever the current route uses, mirror it.
- **URL-encoded DNs** contain `,` → `%2C`, `=` → `%3D`, `\` → `%5C`. `url.PathUnescape` reverses this. Tests must exercise DNs with special characters.
- **AAA risk spots for this slice**:
  - `.list-row__dot` at `var(--fg-muted)` for disabled dot — not a text token, fine.
  - `.list-row__badge` border `var(--border)` on bg `inherit` — still decorative.
  - `.drawer__sub` at `var(--fg-muted)` — meets 7.1:1 AAA (verified Slice 1).
  - `.filter-chip` at `var(--fg-muted)` — same.
  - No sub-AAA text tokens introduced.
- **htmx error paths** are out of scope; a dropped network during drawer swap leaves the old drawer contents in place, which is acceptable UX for Phase 1.
- **Mutations are NOT in this slice.** The drawer shows the user's group list as read-only tags. Slice 4b adds add/remove.
