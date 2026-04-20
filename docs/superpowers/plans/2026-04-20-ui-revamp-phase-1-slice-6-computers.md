# UI Revamp — Phase 1 Slice 6: Computers List + Drawer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Rewrite `/computers` and `/computers/:dn` on the new stack. Read-only list + drawer. Mirrors Slice 5 Groups but without members/pivots beyond OU.

**Tech Stack:** Unchanged.

**Spec reference:** §5, §6.2.

---

## Task 1 — Handler + templates + E2E

**Files:**
- Create: `internal/web/computers_v2_handler.go`
- Create: `internal/web/templates/computers_v2.templ`
- Create: `internal/web/templates/computer_drawer_fragment.templ`
- Create: `internal/e2e/computers_v2_test.go`
- Modify: `internal/web/server.go`

- [ ] **Step 1: Handler**

```go
// internal/web/computers_v2_handler.go
package web

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// buildComputerDrawerVM hydrates the computer drawer view-model.
func (a *App) buildComputerDrawerVM(computerDN, viewerDN string) (templates.ComputerDrawerVM, bool) {
	computer, ok := a.lookupComputerByDN(computerDN)
	if !ok {
		return templates.ComputerDrawerVM{}, false
	}

	pinned := false
	if a.pinnedStore != nil && viewerDN != "" {
		pinned, _ = a.pinnedStore.IsPinned(viewerDN, computerDN)
	}

	ouName := immediateOU(computerDN)

	return templates.ComputerDrawerVM{
		Computer:    computer,
		Pinned:      pinned,
		OUName:      ouName,
		OUPivotHref: buildComputerOUPivotHref(ouName),
	}, true
}

func buildComputerOUPivotHref(ou string) string {
	if ou == "" {
		return ""
	}

	v := url.Values{}
	v.Set("ou", ou)

	return "/computers?" + v.Encode()
}

func (a *App) handleComputersV2(c *fiber.Ctx) error {
	_, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	ouFilter := c.Query("ou")

	var computers []ldap.Computer
	if a.ldapCache != nil {
		all := a.ldapCache.FindComputers(true)
		computers = filterComputersByOU(all, ouFilter)
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.ComputersListV2(computers, ouFilter, templates.Flashes()).
		Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) handleComputerV2(c *fiber.Ctx) error {
	viewerDN, handled, res := a.resolveViewerDN(c)
	if handled {
		return res
	}

	computerDN, err := url.PathUnescape(c.Params("*"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid dn")
	}

	vm, ok := a.buildComputerDrawerVM(computerDN, viewerDN)
	if !ok {
		return c.Status(fiber.StatusNotFound).SendString("computer not found")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	if c.Query("fragment") == "drawer" {
		return templates.ComputerDrawerFragment(vm).
			Render(c.UserContext(), c.Response().BodyWriter())
	}

	return templates.ComputerFullV2(vm).
		Render(c.UserContext(), c.Response().BodyWriter())
}

func filterComputersByOU(computers []ldap.Computer, ou string) []ldap.Computer {
	if ou == "" {
		return computers
	}

	out := make([]ldap.Computer, 0, len(computers))
	for _, cp := range computers {
		if immediateOU(cp.DN()) == ou {
			out = append(out, cp)
		}
	}

	return out
}
```

**Note on `FindComputers` signature:** previous slices used `a.ldapCache.FindComputers(true)`. If the signature is zero-arg, adjust.

- [ ] **Step 2: Templates**

```go
// internal/web/templates/computers_v2.templ
package templates

import (
	"fmt"
	"net/url"

	ldap "github.com/netresearch/simple-ldap-go"
)

type ComputerDrawerVM struct {
	Computer    ldap.Computer
	Pinned      bool
	OUName      string
	OUPivotHref string
}

templ ComputersListV2(computers []ldap.Computer, ouFilter string, flashes []Flash) {
	@baseV2("Computers") {
		@topnavV2("/computers")

		<main class="list-page">
			<header class="list-page__head">
				<h1 class="list-page__title">Computers</h1>
				<p class="list-page__count">{ fmt.Sprintf("%d", len(computers)) } computers</p>
			</header>

			<div class="list-page__filters" data-search-filter>
				<input
					type="search"
					class="list-page__search"
					placeholder="Filter computers…"
					aria-label="Filter computers"
					data-search-input
				/>
				if ouFilter != "" {
					<a class="filter-chip filter-chip--on" href={ clearComputersOUHref() }>
						ou={ ouFilter } ×
					</a>
				}
			</div>

			<div class="list-page__pane">
				<ul class="list-rows" data-search-list aria-label="Computers">
					for _, cp := range computers {
						@computerRowV2(cp)
					}
					if len(computers) == 0 {
						<li class="list-rows__empty">No computers match.</li>
					}
				</ul>

				<aside class="drawer" id="drawer" aria-live="polite" aria-label="Detail">
					<div class="drawer__empty">Select a computer to see details.</div>
				</aside>
			</div>
		</main>

		@paletteV2()
	}
}

templ computerRowV2(cp ldap.Computer) {
	<li class="list-row" data-search-item>
		<a
			class="list-row__link"
			href={ computerDetailHref(cp) }
			hx-get={ computerDrawerFragmentHref(cp) }
			hx-target="#drawer"
			hx-swap="innerHTML"
			hx-push-url="true"
			title={ "View " + cp.CN() }
		>
			<span class="list-row__dot" data-enabled={ computerEnabledAttr(cp) }></span>
			<span class="list-row__primary">{ cp.CN() }</span>
			<span class="list-row__secondary">{ cp.SAMAccountName }</span>
			if !cp.Enabled {
				<span class="list-row__badge">disabled</span>
			}
		</a>
	</li>
}

templ ComputerFullV2(vm ComputerDrawerVM) {
	@baseV2(vm.Computer.CN()) {
		@topnavV2("/computers")

		<main class="list-page list-page--single">
			<div class="list-page__pane">
				<aside class="drawer drawer--full" aria-label="Computer detail">
					@computerDrawerContents(vm)
				</aside>
			</div>
		</main>

		@paletteV2()
	}
}

templ computerDrawerContents(vm ComputerDrawerVM) {
	<header
		class="drawer__head"
		data-recent-type="computer"
		data-recent-dn={ vm.Computer.DN() }
		data-recent-cn={ vm.Computer.CN() }
	>
		<div class="drawer__title-wrap">
			<h2 class="drawer__title">{ vm.Computer.CN() }</h2>
			<p class="drawer__sub">{ vm.Computer.SAMAccountName }</p>
		</div>
		@pinStarButton("computer", vm.Computer.DN(), vm.Pinned)
	</header>

	<p class="drawer__dn">{ vm.Computer.DN() }</p>

	<section class="drawer__section">
		<h3 class="drawer__section-title">Attributes</h3>
		<dl class="drawer__kv">
			<dt>Status</dt>
			<dd>
				if vm.Computer.Enabled {
					Enabled
				} else {
					Disabled
				}
			</dd>
			if vm.Computer.Description != "" {
				<dt>Description</dt>
				<dd>{ vm.Computer.Description }</dd>
			}
			if vm.Computer.SAMAccountName != "" {
				<dt>sAMAccountName</dt>
				<dd>{ vm.Computer.SAMAccountName }</dd>
			}
		</dl>
	</section>

	<section class="drawer__section">
		<h3 class="drawer__section-title">Pivot</h3>
		<ul class="drawer__pivots">
			<li>
				<a class="drawer__pivot" href={ computerDetailHref(vm.Computer) }>
					<span>Open full page</span>
					<span aria-hidden="true">→</span>
				</a>
			</li>
			if vm.OUPivotHref != "" {
				<li>
					<a class="drawer__pivot" href={ templ.URL(vm.OUPivotHref) }>
						<span>Other computers in { vm.OUName }</span>
						<span aria-hidden="true">→</span>
					</a>
				</li>
			}
		</ul>
	</section>
}

func computerDetailHref(cp ldap.Computer) templ.SafeURL {
	return templ.URL("/computers/" + url.PathEscape(cp.DN()))
}

func computerDrawerFragmentHref(cp ldap.Computer) string {
	return "/computers/" + url.PathEscape(cp.DN()) + "?fragment=drawer"
}

func clearComputersOUHref() templ.SafeURL {
	return templ.URL("/computers")
}

func computerEnabledAttr(cp ldap.Computer) string {
	if cp.Enabled {
		return "true"
	}
	return "false"
}
```

- [ ] **Step 3: Fragment templ**

```go
// internal/web/templates/computer_drawer_fragment.templ
package templates

templ ComputerDrawerFragment(vm ComputerDrawerVM) {
	@computerDrawerContents(vm)
}
```

- [ ] **Step 4: Route registrations**

In `internal/web/server.go`:

```go
protected.Get("/computers", a.handleComputersV2)
protected.Get("/computers/*", a.handleComputerV2)
```

Keep old handlers.

- [ ] **Step 5: E2E test**

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

func TestComputersV2_FlowAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())
	tp.Navigate("/computers")

	titleBox, err := page.Locator("h1.list-page__title").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, titleBox)
	assert.Greater(t, titleBox.Height, 0.0)

	// Axe AAA pass (list with possibly zero rows still needs to be clean).
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

- [ ] **Step 6: Build, run, commit**

```bash
rm -f internal/web/templates/*_templ.go
templ generate
go build ./...
go test ./internal/web/ -count=1 | grep -v TestLDAPIntegration | grep -E "^(---|FAIL|ok)"
go test -tags e2e ./internal/e2e/ -run TestComputersV2_FlowAndAAA -v
make lint-go

git add internal/web/computers_v2_handler.go \
        internal/web/templates/computers_v2.templ \
        internal/web/templates/computer_drawer_fragment.templ \
        internal/web/server.go \
        internal/e2e/computers_v2_test.go
# plus any test file updates from Class-A assertion changes
git commit -S --signoff -m "feat(ui): ComputersListV2 + drawer fragment + full-page computer detail + E2E"
```

## Self-review notes

- CSS fully reused from Slice 4.
- `ldap.Computer` shape: `.CN()`, `.DN()`, `.SAMAccountName`, `.Description`, `.Enabled` — verify at top of handler work; if any differ adjust.
- `FindComputers` may or may not take a bool — check existing handler or search_index.go.
- Computer drawer has no "members" or "groups" sections — it's the simplest of the three.
