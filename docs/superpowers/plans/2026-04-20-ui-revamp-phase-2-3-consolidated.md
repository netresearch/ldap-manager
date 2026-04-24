# UI Revamp — Phases 2 & 3 Consolidated Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Deliver the highest-value Phase 2 and Phase 3 features as one combined slice. Skip low-value items (OU tree rail, saved views) and defer the genuinely complex graph view to a dedicated future effort.

**Scope:**
- **Phase 2:**
  - Inline attribute edit for users (email + description only; not DN/CN).
  - Last-logon filter on `/users` list.
  - OU tree rail on `/users`, `/groups`, `/computers` — toggleable secondary filter (populated from distinct immediate-OU values in the cache).
- **Phase 3:**
  - Bulk actions — multi-select user rows + batch add-to-group (single action for Phase 3 MVP).
- **Deferred (documented):** graph view, saved named views. Reasons: graph view needs its own AAA research; saved views are low priority post-palette.

**Architecture:** All features respect CSP (external JS, no inline scripts). htmx drives the mutations; small v2-*.js helpers wire up multi-select + OU rail toggle.

---

## Task 1 — Inline edit for user email + description

**Files:**
- Modify: `internal/web/users_v2_handler.go` — add `handleUserV2Patch` (PATCH or POST with `?edit=1`)
- Modify: `internal/web/templates/users_v2.templ` — drawer `Attributes` section: each editable field becomes a `<form>` with `hx-patch="/users/<dn>?field=email"`
- New: `internal/web/static/js/v2-inline-edit.js` — optional tiny helper to promote `<dd>` → editable `<input>` on double-click (graceful fallback: a pencil ↗ link opens an inline form)
- New CSS in `app.css` for `.drawer__kv-editable`

- [ ] **Step 1: Server handler for single-field edit**

Add to `users_v2_handler.go`:

```go
// handleUserV2Edit updates a single whitelisted user attribute.
// Form fields:
//   field=email   value=<new>
//   field=description  value=<new>
// Response: drawer fragment (HX-Request: true) or full page redirect.
func (a *App) handleUserV2Edit(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	userDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	field := c.FormValue("field")
	value := c.FormValue("value")

	switch field {
	case "email", "description":
		// whitelisted
	default:
		return c.Status(fiber.StatusBadRequest).SendString("field not editable")
	}

	client, err := a.userLDAPClient(c)
	if err != nil {
		return handle500(c, err)
	}
	defer client.Close()

	// The simple-ldap-go library exposes ModifyUser(dn, modifications).
	// If it doesn't expose per-field helpers, issue a raw modify.
	if err := modifyUserField(client, userDN, field, value); err != nil {
		log.Error().Err(err).Str("user", userDN).Str("field", field).Msg("inline edit failed")
		return handle500(c, err)
	}

	// Invalidate cache so the drawer re-renders with the new value.
	a.ldapCache.InvalidateUser(userDN)

	// Re-render drawer fragment.
	vm, ok := a.buildUserDrawerVM(userDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("user not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return templates.UserDrawerFragment(vm).
		Render(c.UserContext(), c.Response().BodyWriter())
}
```

Helpers (adjust to simple-ldap-go actual API — grep existing modify patterns in the codebase first):

```go
func modifyUserField(client UserLDAPClient, dn, field, value string) error {
	attr := ldapAttrForField(field)
	if attr == "" {
		return fmt.Errorf("no LDAP attribute for field %q", field)
	}
	req := ldap.NewModifyRequest(dn, nil)
	if value == "" {
		req.Delete(attr, []string{})
	} else {
		req.Replace(attr, []string{value})
	}
	return client.Modify(req)
}

func ldapAttrForField(field string) string {
	switch field {
	case "email":
		return "mail"
	case "description":
		return "description"
	}
	return ""
}
```

Register:

```go
protected.Post("/users/*/edit", a.handleUserV2Edit)
```

- [ ] **Step 2: Templ — editable KV row**

Replace static `<dd>` for email/description with a form:

```go
<dt>Email</dt>
<dd>
    <form class="kv-edit"
          hx-post={ userEditHref(vm.User.User) }
          hx-target="#drawer"
          hx-swap="innerHTML">
        <input type="hidden" name="field" value="email"/>
        <input class="kv-edit__input" name="value"
               type="email"
               value={ derefString(vm.User.Mail) }
               aria-label="Email"
               placeholder="(no email)"/>
        <button class="kv-edit__save" type="submit" aria-label="Save email">✓</button>
    </form>
</dd>
```

Add helper functions in `users_v2.templ`:

```go
func userEditHref(u ldap.User) string {
	return "/users/" + url.PathEscape(u.DN()) + "/edit"
}

func derefString(p *string) string {
	if p == nil { return "" }
	return *p
}
```

- [ ] **Step 3: CSS**

Append to `app.css`:

```css
.kv-edit { display: flex; gap: 0.25rem; align-items: center; margin: 0; }
.kv-edit__input {
    flex: 1; min-width: 0;
    padding: 0.25rem 0.5rem;
    background: var(--bg);
    border: 1px solid transparent;
    border-radius: 0.25rem;
    color: var(--fg);
    font: inherit;
}
.kv-edit__input:hover { border-color: var(--border); }
.kv-edit__input:focus-visible { border-color: var(--border-strong); outline: none; }
.kv-edit__save {
    display: inline-flex; align-items: center; justify-content: center;
    width: 1.75rem; height: 1.75rem; padding: 0;
    border: 1px solid var(--border); background: var(--bg-subtle); color: var(--fg-muted);
    border-radius: 0.25rem; cursor: pointer; line-height: 1;
    opacity: 0; transition: opacity 120ms;
}
.kv-edit:hover .kv-edit__save,
.kv-edit:focus-within .kv-edit__save { opacity: 1; }
.kv-edit__save:hover, .kv-edit__save:focus-visible {
    border-color: var(--border-strong); color: var(--fg);
}
```

- [ ] **Step 4: Build + test + commit**

```bash
templ generate && go build ./... && make lint-go
git add -A
git commit -S --signoff -m "feat(ui): inline edit for user email + description via htmx"
```

If simple-ldap-go doesn't expose a raw `Modify` method, use whatever the library exposes to update `mail` / `description`. Grep for existing `userModifyHandler` flow if it survived — that's a reference for how writes were done previously.

---

## Task 2 — Last-logon filter on /users

**Files:**
- Modify: `internal/web/users_v2_handler.go` — accept `?last-logon=24h|7d|30d|never` query param
- Modify: `internal/web/templates/users_v2.templ` — add filter chips

- [ ] **Step 1: Handler filter**

Add to `handleUsersV2`:

```go
lastLogonFilter := c.Query("last-logon") // "", "24h", "7d", "30d", "never"
users = filterUsersByLastLogon(users, lastLogonFilter)
```

Helper:

```go
func filterUsersByLastLogon(users []ldap.User, window string) []ldap.User {
	if window == "" {
		return users
	}
	var cutoff time.Time
	switch window {
	case "24h":
		cutoff = time.Now().Add(-24 * time.Hour)
	case "7d":
		cutoff = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		cutoff = time.Now().Add(-30 * 24 * time.Hour)
	case "never":
		out := users[:0:0]
		for _, u := range users {
			if u.LastLogon == 0 {
				out = append(out, u)
			}
		}
		return out
	default:
		return users
	}
	out := users[:0:0]
	for _, u := range users {
		if u.LastLogon == 0 { continue }
		if ldapFileTimeToGoTime(u.LastLogon).After(cutoff) {
			out = append(out, u)
		}
	}
	return out
}
```

Find the existing LDAP filetime converter in the codebase (there's likely one used by old `user.templ`); reuse instead of duplicating.

- [ ] **Step 2: Filter chips**

Add in the filter row after the existing chips:

```go
@lastLogonChip("24h", "last 24h", c.Query("last-logon"))
@lastLogonChip("7d", "last 7d", c.Query("last-logon"))
@lastLogonChip("30d", "last 30d", c.Query("last-logon"))
@lastLogonChip("never", "never logged in", c.Query("last-logon"))
```

`c.Query` is a handler-side thing; pass the active value into `UsersListV2(...)` as a new param `lastLogonFilter string`. Then render chips:

```go
templ lastLogonChip(value, label, active string) {
	<a class={ filterChipClass(value == active) } href={ lastLogonChipHref(value) }>
		{ label }
		if value == active { × }
	</a>
}

func lastLogonChipHref(value string) templ.SafeURL {
	v := url.Values{}
	if value != "" { v.Set("last-logon", value) }
	return templ.URL("/users?" + v.Encode())
}
```

- [ ] **Step 3: Commit**

```bash
templ generate && go build ./... && make lint-go
git add -A
git commit -S --signoff -m "feat(ui): add last-logon filter chips to /users"
```

---

## Task 3 — OU tree rail (toggleable)

**Files:**
- Modify: `internal/web/templates/users_v2.templ`, `groups_v2.templ`, `computers_v2.templ` — add optional left rail
- Modify: handlers to compute distinct OU list from cache
- Modify: `app.css` for rail styles
- New: `internal/web/static/js/v2-rail.js` for toggle state persistence

Rail shows a list of distinct OUs extracted from the cached entity DNs, each an `<a href="/users?ou=...">`. Click to filter; click the header to collapse.

- [ ] **Step 1: Helper — distinct OUs**

```go
// in users_v2_handler.go (or a shared helpers file)
func distinctImmediateOUs(dns []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, dn := range dns {
		ou := immediateOU(dn)
		if ou == "" { continue }
		if _, ok := seen[ou]; ok { continue }
		seen[ou] = struct{}{}
		out = append(out, ou)
	}
	sort.Strings(out)
	return out
}
```

Call with DNs from `a.ldapCache.FindUsers(true)`, `FindGroups()`, `FindComputers(true)` respectively, and pass to the templates.

- [ ] **Step 2: Template — list-page__rail**

In each list page, wrap the `.list-page__pane` with a 3-column grid:

```css
.list-page__pane--rail {
    grid-template-columns: 14rem 1fr 22rem;
}
@media (max-width: 900px) {
    .list-page__pane--rail {
        grid-template-columns: 1fr;
    }
    .list-page__rail { display: none; }
}
```

Template:

```go
<nav class="list-page__rail" aria-label="OUs">
    <h2 class="list-page__rail-title">OUs</h2>
    <ul>
        for _, ou := range ous {
            <li>
                <a class={ filterChipClass(ou == activeOU) } href={ templ.URL("/users?ou=" + url.QueryEscape(ou)) }>
                    { ou }
                </a>
            </li>
        }
    </ul>
</nav>
```

- [ ] **Step 3: CSS**

```css
.list-page__rail {
    padding: 0.5rem; background: var(--bg-subtle);
    border: 1px solid var(--border); border-radius: 0.5rem;
    align-self: start; position: sticky;
    top: calc(var(--density-touch-size) + 1rem);
}
.list-page__rail-title { font-size: 0.75rem; letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-muted); margin: 0 0 0.5rem; font-weight: 600; }
.list-page__rail ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.125rem; }
.list-page__rail li .filter-chip { width: 100%; justify-content: flex-start; border-radius: 0.25rem; }
```

- [ ] **Step 4: Commit**

```bash
templ generate && go build ./... && make lint-go
git add -A
git commit -S --signoff -m "feat(ui): OU tree rail on /users, /groups, /computers"
```

---

## Task 4 — Bulk actions (Phase 3)

**Files:**
- Modify: `internal/web/templates/users_v2.templ` — per-row checkbox + floating bar
- New: `internal/web/static/js/v2-bulk.js` — multi-select state + floating-bar rendering
- New: `internal/web/bulk_handlers.go` — `POST /users/bulk?action=add-to-group` that takes `target_dn[]` + `group_dn`
- New: CSS for `.bulk-bar`

- [ ] **Step 1: Row checkboxes**

Add to `userRowV2`:

```go
<input type="checkbox" class="list-row__check" data-bulk value={ u.DN() } aria-label={ "Select " + u.CN() }/>
```

- [ ] **Step 2: Floating bar + bulk JS**

```js
/* internal/web/static/js/v2-bulk.js */
(function () {
  "use strict";

  var bar = null;
  var selected = new Set();

  function ensureBar() {
    if (bar) return bar;
    bar = document.createElement("div");
    bar.className = "bulk-bar";
    bar.setAttribute("role", "region");
    bar.setAttribute("aria-label", "Bulk actions");
    bar.hidden = true;

    var count = document.createElement("span");
    count.className = "bulk-bar__count";
    bar.appendChild(count);

    var addBtn = document.createElement("button");
    addBtn.type = "button";
    addBtn.className = "bulk-bar__action";
    addBtn.textContent = "Add to group…";
    addBtn.addEventListener("click", openAddToGroup);
    bar.appendChild(addBtn);

    var cancelBtn = document.createElement("button");
    cancelBtn.type = "button";
    cancelBtn.className = "bulk-bar__cancel";
    cancelBtn.textContent = "Cancel";
    cancelBtn.addEventListener("click", clearSelection);
    bar.appendChild(cancelBtn);

    document.body.appendChild(bar);
    return bar;
  }

  function updateBar() {
    ensureBar();
    var n = selected.size;
    bar.hidden = n === 0;
    bar.querySelector(".bulk-bar__count").textContent = n + " selected";
  }

  function clearSelection() {
    selected.clear();
    document.querySelectorAll("[data-bulk]:checked").forEach(function (cb) { cb.checked = false; });
    updateBar();
  }

  function openAddToGroup() {
    var g = window.prompt("Group DN to add selected users to:");
    if (!g) return;
    var form = document.createElement("form");
    form.method = "post";
    form.action = "/users/bulk?action=add-to-group";
    form.style.display = "none";
    var groupInput = document.createElement("input");
    groupInput.name = "group_dn";
    groupInput.value = g;
    form.appendChild(groupInput);
    selected.forEach(function (dn) {
      var i = document.createElement("input");
      i.name = "target_dn";
      i.value = dn;
      form.appendChild(i);
    });
    document.body.appendChild(form);
    form.submit();
  }

  document.addEventListener("change", function (ev) {
    var t = ev.target;
    if (!t || !t.hasAttribute("data-bulk")) return;
    if (t.checked) selected.add(t.value);
    else selected.delete(t.value);
    updateBar();
  });
})();
```

(A group-picker UI is nicer, but a prompt is CSP-safe and viable for MVP.)

- [ ] **Step 3: Handler**

```go
// internal/web/bulk_handlers.go
package web

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

func (a *App) handleBulkUsers(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}
	_ = viewerDN

	action := c.Query("action")
	if action != "add-to-group" {
		return c.Status(fiber.StatusBadRequest).SendString("unknown action")
	}

	// Fiber's FormValue("target_dn") only returns the first; use the underlying request body.
	form, err := c.MultipartForm()
	if err != nil {
		// Also try standard form parsing:
		if raw := c.Request().PostArgs().PeekMulti("target_dn"); len(raw) > 0 {
			targets := make([]string, 0, len(raw))
			for _, v := range raw { targets = append(targets, string(v)) }
			groupDN := c.FormValue("group_dn")
			return a.bulkAddToGroup(c, targets, groupDN)
		}
		return c.Status(fiber.StatusBadRequest).SendString("invalid form")
	}

	targets := form.Value["target_dn"]
	groupDN := c.FormValue("group_dn")
	return a.bulkAddToGroup(c, targets, groupDN)
}

func (a *App) bulkAddToGroup(c *fiber.Ctx, userDNs []string, groupDN string) error {
	if groupDN == "" { return c.Status(fiber.StatusBadRequest).SendString("missing group_dn") }
	if len(userDNs) == 0 { return c.Redirect("/users", fiber.StatusSeeOther) }

	client, err := a.userLDAPClient(c)
	if err != nil { return handle500(c, err) }
	defer client.Close()

	for _, userDN := range userDNs {
		if err := addUserToGroup(client, groupDN, userDN); err != nil {
			log.Error().Err(err).Str("user", userDN).Str("group", groupDN).Msg("bulk add-to-group failed")
		} else {
			a.ldapCache.InvalidateUser(userDN)
			a.ldapCache.InvalidateGroup(groupDN)
		}
	}
	return c.Redirect("/users", fiber.StatusSeeOther)
}

// addUserToGroup adds member to the group's member attribute.
// Adjust signature to match simple-ldap-go's modify API.
func addUserToGroup(client UserLDAPClient, groupDN, userDN string) error {
	req := ldap.NewModifyRequest(groupDN, nil)
	req.Add("member", []string{userDN})
	return client.Modify(req)
}
```

Register:

```go
protected.Post("/users/bulk", a.handleBulkUsers)
```

- [ ] **Step 4: CSS**

```css
.bulk-bar {
    position: fixed;
    bottom: 1rem;
    left: 50%;
    transform: translateX(-50%);
    display: flex; align-items: center; gap: 0.75rem;
    padding: 0.625rem 1rem;
    background: var(--fg); color: var(--bg);
    border-radius: 0.5rem;
    box-shadow: 0 8px 24px rgb(0 0 0 / 0.3);
    z-index: 80;
}
.bulk-bar__count { font-weight: 600; }
.bulk-bar__action,
.bulk-bar__cancel {
    padding: 0.25rem 0.75rem;
    background: var(--bg); color: var(--fg);
    border: 1px solid var(--bg); border-radius: 0.375rem;
    font: inherit; cursor: pointer;
}
```

- [ ] **Step 5: Load bulk JS in base_v2**

```go
<script defer src="/static/js/v2-bulk.js"></script>
```

- [ ] **Step 6: Commit**

```bash
templ generate && go build ./... && make lint-go
git add -A
git commit -S --signoff -m "feat(ui): bulk add-to-group on /users (Phase 3)"
```

---

## Task 5 — Phase 3 graph view — deferred

**Files:**
- Create: `docs/superpowers/specs/2026-04-20-ui-revamp-phase-3-graph-view-deferred.md`

Document the deferral: graph view needs AAA-compliance research (ARIA graph patterns, keyboard nav, parallel list representation) that exceeds the scope of the single autonomous session. Include a rough shape for the future implementation.

- [ ] **Step 1: Write the deferral note**

```markdown
# Phase 3 Graph View — Deferred

The relationship graph view (users ↔ groups ↔ computers ↔ OUs) was in scope
for Phase 3 per the design spec (`2026-04-20-ui-revamp-design.md` §2).

It is deferred from this autonomous implementation because:

1. WCAG 2.2 AAA requires a keyboard-navigable, screen-reader-usable
   representation of the graph. `role="graphics-*"` ARIA patterns have
   varying browser + AT support; the established pattern is a parallel
   data-table representation that mirrors the visual graph. This is a
   design + QA effort beyond single-session scope.
2. Force-directed layouts (d3-force, cytoscape-web, native SVG) each have
   CSP and bundle-size tradeoffs that need evaluation against the rest of
   the vendored stack.
3. Interaction design (pan/zoom on touch; focus ring visibility on nodes;
   edge highlighting on hover) requires user testing, which this session
   can't run.

## Rough shape for future work

- `/graph?entity=<dn>` — renders the graph centered on the entity.
- Query returns JSON `{nodes: [{id,type,label}], edges: [{source,target,type}]}`
  derived from cache membership relationships.
- Client renders via vendored `cytoscape.min.js` (AAA-friendly layout only)
  with ARIA live-region announcements on focus changes.
- `/graph?entity=<dn>&view=list` — AAA-compliant parallel list representation
  (entity + direct relations + second-degree relations).

Estimated effort: 3-5 days engineering + 2 days a11y testing.
```

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-04-20-ui-revamp-phase-3-graph-view-deferred.md
git commit -S --signoff -m "docs: defer Phase 3 graph view with implementation notes"
```

---

## Task 6 — Final verification

- [ ] **Step 1: Full test suite**

```bash
make lint-go
go test ./... -count=1 2>&1 | grep -E "^(FAIL|ok)"
go test -tags e2e ./internal/e2e/ -count=1 -run "Axe|V2" -v 2>&1 | tail -30
```

All green (except pre-existing LDAP-integration env failures).

- [ ] **Step 2: CHANGELOG**

Append to Unreleased:

```
### Added
- Phase 2: inline edit for user email + description; last-logon filter chips on /users; OU tree rail on list pages.
- Phase 3: bulk add-to-group for users.
- Phase 3 graph view: deferred with design notes (see docs/superpowers/specs/...).
```

```bash
git add CHANGELOG.md
git commit -S --signoff -m "docs: CHANGELOG for Phase 2+3"
```
