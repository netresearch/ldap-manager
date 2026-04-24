# UI Revamp — Phase 1 Slice 5: Groups List + Drawer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`).

**Goal:** Rewrite `/groups` and `/groups/:dn` on the new stack. Mirrors Slice 4 Users: list + drawer with htmx row swap, pinned, pivots, recents. Drawer shows group members (as tag chips) instead of groups.

**Architecture:** Same pattern as Slice 4. `GroupsListV2`, `GroupFullV2`, `GroupDrawerFragment`. Row click → drawer fragment → htmx innerHTML swap. Direct nav `/groups/:dn` → full page. Members render via resolved `ldap_cache.FullLDAPGroup` (mirrors `FullLDAPUser`).

**Tech Stack:** Unchanged from Slice 4.

**Out of scope:** In-drawer add/remove member (Slice 5b). In-drawer group attribute edit (Phase 2).

**Spec reference:** `docs/superpowers/specs/2026-04-20-ui-revamp-design.md` §6.2 drawer, §6.3 pivots.

---

## Pre-flight

```bash
cd /home/cybot/projects/ldap-manager-ui-revamp-phase-1a
git log --oneline ddea7d9..HEAD | head -3
go test ./internal/web/ -count=1 -run TestAppCSSContrastAAA
```

---

## Task 1 — Handler + templates

**Files:**
- Create: `internal/web/groups_v2_handler.go`
- Create: `internal/web/templates/groups_v2.templ`
- Create: `internal/web/templates/group_drawer_fragment.templ`
- Modify: `internal/web/server.go`

- [ ] **Step 1: Handler**

```go
// internal/web/groups_v2_handler.go
package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// buildGroupDrawerVM hydrates the group drawer view-model.
func (a *App) buildGroupDrawerVM(groupDN, viewerDN string) (templates.GroupDrawerVM, bool) {
	group, ok := a.lookupGroupByDN(groupDN)
	if !ok {
		return templates.GroupDrawerVM{}, false
	}

	var pinned bool
	if a.pinnedStore != nil && viewerDN != "" {
		pinned, _ = a.pinnedStore.IsPinned(viewerDN, groupDN)
	}

	fullGroup := a.populateMembersForGroup(&group)
	ouName := immediateOU(groupDN)

	return templates.GroupDrawerVM{
		Group:       fullGroup,
		Pinned:      pinned,
		OUName:      ouName,
		OUPivotHref: buildGroupOUPivotHref(ouName),
	}, true
}

// populateMembersForGroup resolves a group's member DN list into
// []ldap.User via the ldap_cache. Missing members are silently skipped.
func (a *App) populateMembersForGroup(group *ldap.Group) *ldap_cache.FullLDAPGroup {
	var users []ldap.User
	if a.ldapCache != nil {
		users = a.ldapCache.FindUsers(true)
	}

	return ldap_cache.PopulateMembersForGroupFromData(group, users)
}

func buildGroupOUPivotHref(ou string) string {
	if ou == "" {
		return ""
	}

	v := url.Values{}
	v.Set("ou", ou)

	return "/groups?" + v.Encode()
}

func (a *App) handleGroupsV2(c *fiber.Ctx) error {
	viewerDN := GetUserDN(c)
	if viewerDN == "" {
		sess, err := a.sessionStore.Get(c)
		if err != nil {
			return handle500(c, err)
		}

		viewerDN, _ = sess.Get("dn").(string)
		if viewerDN == "" {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}
	}

	ouFilter := c.Query("ou")

	var groups []ldap.Group
	if a.ldapCache != nil {
		all := a.ldapCache.FindGroups()
		groups = filterGroupsByOU(all, ouFilter)
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.GroupsListV2(groups, ouFilter, templates.Flashes()).
		Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) handleGroupV2(c *fiber.Ctx) error {
	viewerDN := GetUserDN(c)
	if viewerDN == "" {
		sess, err := a.sessionStore.Get(c)
		if err != nil {
			return handle500(c, err)
		}

		viewerDN, _ = sess.Get("dn").(string)
		if viewerDN == "" {
			return c.Redirect("/login", fiber.StatusSeeOther)
		}
	}

	encodedDN := c.Params("*")
	groupDN, err := url.PathUnescape(encodedDN)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	vm, ok := a.buildGroupDrawerVM(groupDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("group not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" {
		return templates.GroupDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.GroupFullV2(vm).
		Render(c.UserContext(), c.Response().BodyWriter())
}

func filterGroupsByOU(groups []ldap.Group, ou string) []ldap.Group {
	if ou == "" {
		return groups
	}

	out := groups[:0:0]
	for _, g := range groups {
		if immediateOU(g.DN()) == ou {
			out = append(out, g)
		}
	}

	return out
}
```

**Note:** `ldap_cache.PopulateMembersForGroupFromData` may not exist yet — mirror whatever helper `PopulateGroupsForUserFromData` uses. If the cache package doesn't expose it, either add a thin helper in the ldap_cache package **or** do the member resolution inline in `populateMembersForGroup` by iterating `group.Members` and looking up each user DN. Choose based on existing patterns.

- [ ] **Step 2: Templates — `groups_v2.templ`**

```go
// internal/web/templates/groups_v2.templ
package templates

import (
	"fmt"
	"net/url"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

type GroupDrawerVM struct {
	Group       *ldap_cache.FullLDAPGroup
	Pinned      bool
	OUName      string
	OUPivotHref string
}

templ GroupsListV2(groups []ldap.Group, ouFilter string, flashes []Flash) {
	@baseV2("Groups") {
		@topnavV2("/groups")

		<main class="list-page">
			<header class="list-page__head">
				<h1 class="list-page__title">Groups</h1>
				<p class="list-page__count">{ fmt.Sprintf("%d", len(groups)) } groups</p>
			</header>

			<div class="list-page__filters" data-search-filter>
				<input
					type="search"
					class="list-page__search"
					placeholder="Filter groups…"
					aria-label="Filter groups"
					data-search-input
				/>
				if ouFilter != "" {
					<a class="filter-chip filter-chip--on" href={ clearGroupsOUHref() }>
						ou={ ouFilter } ×
					</a>
				}
			</div>

			<div class="list-page__pane">
				<ul class="list-rows" data-search-list aria-label="Groups">
					for _, g := range groups {
						@groupRowV2(g)
					}
					if len(groups) == 0 {
						<li class="list-rows__empty">No groups match.</li>
					}
				</ul>

				<aside class="drawer" id="drawer" aria-live="polite" aria-label="Detail">
					<div class="drawer__empty">Select a group to see details.</div>
				</aside>
			</div>
		</main>

		@paletteV2()
	}
}

templ groupRowV2(g ldap.Group) {
	<li class="list-row" data-search-item>
		<a
			class="list-row__link"
			href={ groupDetailHref(g) }
			hx-get={ groupDrawerFragmentHref(g) }
			hx-target="#drawer"
			hx-swap="innerHTML"
			hx-push-url="true"
			title={ "View " + g.CN() }
		>
			<span class="list-row__dot" data-enabled="true"></span>
			<span class="list-row__primary">{ g.CN() }</span>
			<span class="list-row__secondary">{ fmt.Sprintf("%d members", len(g.Members)) }</span>
		</a>
	</li>
}

templ GroupFullV2(vm GroupDrawerVM) {
	@baseV2(vm.Group.CN()) {
		@topnavV2("/groups")

		<main class="list-page list-page--single">
			<div class="list-page__pane">
				<aside class="drawer drawer--full" aria-label="Group detail">
					@groupDrawerContents(vm)
				</aside>
			</div>
		</main>

		@paletteV2()
	}
}

templ groupDrawerContents(vm GroupDrawerVM) {
	<header
		class="drawer__head"
		data-recent-type="group"
		data-recent-dn={ vm.Group.DN() }
		data-recent-cn={ vm.Group.CN() }
	>
		<div class="drawer__title-wrap">
			<h2 class="drawer__title">{ vm.Group.CN() }</h2>
			<p class="drawer__sub">{ fmt.Sprintf("%d members", len(vm.Group.Members)) }</p>
		</div>
		@pinStarButton("group", vm.Group.DN(), vm.Pinned)
	</header>

	<p class="drawer__dn">{ vm.Group.DN() }</p>

	if vm.Group.Description != "" {
		<section class="drawer__section">
			<h3 class="drawer__section-title">Description</h3>
			<p>{ vm.Group.Description }</p>
		</section>
	}

	<section class="drawer__section">
		<h3 class="drawer__section-title">
			{ fmt.Sprintf("Members · %d", len(vm.Group.Members)) }
		</h3>
		if len(vm.Group.Members) == 0 {
			<p class="drawer__empty-inline">No members.</p>
		} else {
			<ul class="drawer__tags">
				for _, u := range vm.Group.Members {
					<li>
						<a class="drawer__tag" href={ userDetailHref(u) }>{ u.CN() }</a>
					</li>
				}
			</ul>
		}
	</section>

	<section class="drawer__section">
		<h3 class="drawer__section-title">Pivot</h3>
		<ul class="drawer__pivots">
			<li>
				<a class="drawer__pivot" href={ groupDetailHref(asLdapGroup(vm.Group)) }>
					<span>Open full page</span>
					<span aria-hidden="true">→</span>
				</a>
			</li>
			if vm.OUPivotHref != "" {
				<li>
					<a class="drawer__pivot" href={ templ.URL(vm.OUPivotHref) }>
						<span>Other groups in { vm.OUName }</span>
						<span aria-hidden="true">→</span>
					</a>
				</li>
			}
		</ul>
	</section>
}

func groupDrawerFragmentHref(g ldap.Group) string {
	return "/groups/" + url.PathEscape(g.DN()) + "?fragment=drawer"
}

func clearGroupsOUHref() templ.SafeURL {
	return templ.URL("/groups")
}

// asLdapGroup returns the embedded ldap.Group from a FullLDAPGroup so
// existing helpers that take ldap.Group (like groupDetailHref) can be reused.
func asLdapGroup(g *ldap_cache.FullLDAPGroup) ldap.Group {
	if g == nil {
		return ldap.Group{}
	}
	return g.Group
}
```

- [ ] **Step 3: Fragment templ**

```go
// internal/web/templates/group_drawer_fragment.templ
package templates

templ GroupDrawerFragment(vm GroupDrawerVM) {
	@groupDrawerContents(vm)
}
```

- [ ] **Step 4: Route registrations**

In `internal/web/server.go`, swap the groups routes inside the authenticated block:

```go
protected.Get("/groups", a.handleGroupsV2)
protected.Get("/groups/*", a.handleGroupV2)
```

Keep the old `groupsHandler`/`groupHandler` functions (legacy tests may reference them).

- [ ] **Step 5: Build + test**

```bash
rm -f internal/web/templates/*_templ.go
templ generate
go build ./...
go test ./internal/web/ -count=1 2>&1 | grep -v TestLDAPIntegration | grep -E "^(---|ok|FAIL)" | tail -10
```

Fix any Class-A test assertions if groups-related tests hit new markup.

- [ ] **Step 6: Commit**

```bash
git add internal/web/groups_v2_handler.go \
        internal/web/templates/groups_v2.templ \
        internal/web/templates/group_drawer_fragment.templ \
        internal/web/server.go
# Plus any test updates.
git commit -S --signoff -m "feat(ui): GroupsListV2 + drawer fragment + full-page group detail"
```

---

## Task 2 — ldap_cache helper (if missing)

**Files:**
- Possibly modify: `internal/ldap_cache/*.go` to add `PopulateMembersForGroupFromData`

- [ ] **Step 1: Check existing helper**

```bash
grep -n "PopulateMembersForGroupFromData\|FullLDAPGroup" internal/ldap_cache/*.go
```

If `PopulateMembersForGroupFromData` + `FullLDAPGroup` already exist, skip this task.

If `FullLDAPGroup` exists but the helper doesn't, add the helper adjacent to `PopulateGroupsForUserFromData`. Mirror its pattern. If neither exists:

- Define a simple type in the handler file instead:

```go
type localFullGroup struct {
	ldap.Group
	Members     []ldap.User
	Description string
}
```

and adjust the template to use the local type. Prefer upstream helpers if they fit; don't sprawl.

- [ ] **Step 2 (if needed): Add helper**

Following the existing `PopulateGroupsForUserFromData` pattern, add to the ldap_cache package (or adjacent):

```go
// PopulateMembersForGroupFromData resolves a group's Members DN list
// (from ldap.Group) against a user cache, returning a FullLDAPGroup
// with []ldap.User. Missing members are silently skipped.
func PopulateMembersForGroupFromData(group *ldap.Group, users []ldap.User) *FullLDAPGroup {
	if group == nil {
		return nil
	}

	resolved := make([]ldap.User, 0, len(group.Members))
	for _, memberDN := range group.Members {
		for _, u := range users {
			if u.DN() == memberDN {
				resolved = append(resolved, u)
				break
			}
		}
	}

	return &FullLDAPGroup{
		Group:   *group,
		Members: resolved,
	}
}
```

- [ ] **Step 3: Commit (only if new file/symbol)**

```bash
git add internal/ldap_cache/
git commit -S --signoff -m "feat(ldap_cache): PopulateMembersForGroupFromData helper for group drawer VM"
```

If no changes were needed, skip the commit.

---

## Task 3 — E2E + wrap-up

**Files:**
- Create: `internal/e2e/groups_v2_test.go`

- [ ] **Step 1: Write the test (mirrors users_v2_test.go)**

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

func TestGroupsV2_FlowAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())
	tp.Navigate("/groups")

	titleBox, err := page.Locator("h1.list-page__title").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, titleBox)
	assert.Greater(t, titleBox.Height, 0.0)

	rowCount, _ := page.Locator(".list-row__link").Count()
	if rowCount == 0 {
		t.Skip("no seeded groups in e2e fixture — skipping drawer swap path")
	}

	require.NoError(t, page.Locator(".list-row__link").First().Click())
	require.NoError(t, page.Locator(".drawer__head .drawer__title").WaitFor())

	titleText, err := page.Locator(".drawer__head .drawer__title").First().TextContent()
	require.NoError(t, err)
	assert.NotEmpty(t, titleText)

	axePath, _ := filepath.Abs("internal/e2e/testdata/axe.min.js")
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
go test -tags e2e ./internal/e2e/ -run TestGroupsV2_FlowAndAAA -v
```

Fix any AAA violations in templates/CSS until clean.

- [ ] **Step 3: Final lint + tests**

```bash
make lint-go
go test ./... -count=1 2>&1 | grep -E "^(FAIL|ok)"
```

- [ ] **Step 4: Commit**

```bash
git add internal/e2e/groups_v2_test.go
git commit -S --signoff -m "test(e2e): /groups list + drawer + AAA"
```

---

## Self-review notes

- **CSS is already in place** from Slice 4 — all list-page, list-row, drawer, filter-chip classes are reused verbatim. No new CSS tasks this slice.
- **Type reuse**: `pinStarButton` from Slice 4 is shared — it already accepts `(entityType, entityDN, pinned)` so both user and group drawer use it.
- **`hx-post` star button still a no-op** (Slice 3 documented limitation) — ok for this slice.
- **E2E seeded data**: `main_test.go` seedLDIF may not create a group today. Slice 5 E2E gracefully skips if no groups present.
- **`asLdapGroup` helper** exists so `groupDetailHref(ldap.Group)` can still be called from template code that holds a `*FullLDAPGroup`. Consider moving to a shared location if Slice 6 needs similar.
