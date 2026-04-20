# UI Revamp — Phase 1 Slice 7: Palette v2 polish

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** On empty-query palette open, render the user's Pinned (server-rendered hidden list) and Recents (localStorage) so the palette is useful even before typing. Minor polish: close-palette-on-navigate for screen reader continuity, aria-activedescendant wiring.

**Scope:** Pure client-side improvements to `v2-palette.js`; tiny templ change to pass pinned JSON into the palette; small CSS polish if needed.

**Out of scope:** Server-side "Actions" result category ("Add alice to group…") — Phase 2.

**Spec reference:** §6.1 palette — "Empty query state shows: Pinned, then Recent."

---

## Task 1 — Empty-state pinned + recents

**Files:**
- Modify: `internal/web/templates/palette_v2.templ` (render the user's pinned list as a hidden JSON script tag for the palette to read)
- Modify: `internal/web/static/js/v2-palette.js` (render pinned + recents on empty query)
- Modify: `internal/web/home_handler.go` (pass pinned entries into `paletteV2`)
- Possibly: `internal/web/templates/base_v2.templ` if a shared palette-context needs to render across routes

- [ ] **Step 1: Palette pinned injection via Templ context**

Rather than threading pinned through every handler, render a `<script type="application/json">` tag inside the palette that the palette JS reads on open. The base template doesn't know the user, so we add a separate templ that pages can invoke.

Edit `internal/web/templates/palette_v2.templ`:

```go
// internal/web/templates/palette_v2.templ
package templates

// paletteV2 is the ⌘K command palette shell. Body is a native <dialog>.
// Empty-query state is rendered by the JS client-side, seeded from the
// data-pinned JSON attribute on the dialog element (set by routes that
// know the user — routes that don't set it see only recents).
templ paletteV2() {
	@paletteV2WithPinned(nil)
}

templ paletteV2WithPinned(pinned []PinnedEntry) {
	<dialog
		id="cmd-palette"
		class="palette"
		aria-label="Command palette"
		data-pinned={ pinnedJSON(pinned) }
	>
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

Add the helper at the end of `palette_v2.templ` in a `<script>` block? No — Go helper functions live outside templ but in the same `templates` package. Define in a new file `internal/web/templates/palette_helpers.go`:

```go
// internal/web/templates/palette_helpers.go
package templates

import "encoding/json"

// pinnedJSON returns a JSON array suitable for a data-pinned attribute
// on the palette dialog. Empty-safe.
func pinnedJSON(pinned []PinnedEntry) string {
	if len(pinned) == 0 {
		return "[]"
	}

	out := make([]map[string]string, 0, len(pinned))
	for _, p := range pinned {
		out = append(out, map[string]string{
			"type": p.Type,
			"dn":   p.DN,
			"cn":   p.CN,
		})
	}

	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}

	return string(b)
}
```

- [ ] **Step 2: Home + list handlers pass pinned through**

In handlers that render `HomeV2`, call `paletteV2WithPinned(pinned)` instead of `paletteV2()` inside the `HomeV2` templ. Since `HomeV2` already receives `pinned`, update its internal `@paletteV2()` call to `@paletteV2WithPinned(pinned)`.

Edit `internal/web/templates/home_v2.templ`:

```go
		@paletteV2WithPinned(pinned)
```

For list pages (users/groups/computers), the handlers don't currently compute pinned. Add a helper to the App:

```go
// internal/web/palette_context.go
package web

import "github.com/netresearch/ldap-manager/internal/web/templates"

// paletteContextFor returns the pinned slice a page's palette should
// seed from. Safe on nil pinnedStore / empty viewerDN.
func (a *App) paletteContextFor(viewerDN string) []templates.PinnedEntry {
	if viewerDN == "" {
		return nil
	}

	entries, err := a.pinnedEntriesFor(viewerDN)
	if err != nil {
		return nil
	}

	return entries
}
```

Update `UsersListV2`, `UserFullV2`, `GroupsListV2`, `GroupFullV2`, `ComputersListV2`, `ComputerFullV2` templates to accept a `palettePinned []PinnedEntry` parameter and forward to `@paletteV2WithPinned(palettePinned)`. Handlers pass `a.paletteContextFor(viewerDN)`.

(If this plumbing feels heavy, an alternative is to fetch `/api/pinned.json` from the client. For now, the through-passing version is simpler and only touches the render path.)

- [ ] **Step 3: Update v2-palette.js — render empty state**

Replace the current `renderEmptyState` usage on `openPalette` with `renderEmptyContent(read pinned+recents)`. The existing `renderQuery` handles non-empty queries unchanged.

Append/modify inside `internal/web/static/js/v2-palette.js`:

```js
  function readPinned() {
    try {
      var raw = dialog.getAttribute("data-pinned");
      if (!raw) return [];
      var arr = JSON.parse(raw);
      return Array.isArray(arr) ? arr : [];
    } catch (_e) { return []; }
  }

  function readRecents() {
    try {
      var raw = localStorage.getItem("ldap-manager:recents:v1");
      if (!raw) return [];
      var arr = JSON.parse(raw);
      return Array.isArray(arr) ? arr : [];
    } catch (_e) { return []; }
  }

  function renderEmptyContent() {
    clearResults();

    var pinned = readPinned();
    var recents = readRecents();

    if (pinned.length === 0 && recents.length === 0) {
      var li = document.createElement("li");
      li.className = "palette__empty";
      li.textContent = "Type to search.";
      results.appendChild(li);
      focused = -1;
      return;
    }

    if (pinned.length > 0) {
      var header = document.createElement("li");
      header.className = "palette__group-header";
      header.textContent = "Pinned";
      results.appendChild(header);
      for (var i = 0; i < pinned.length; i++) {
        results.appendChild(buildItem(pinned[i], i === 0));
      }
    }

    if (recents.length > 0) {
      var rh = document.createElement("li");
      rh.className = "palette__group-header";
      rh.textContent = "Recent";
      results.appendChild(rh);
      for (var j = 0; j < recents.length; j++) {
        results.appendChild(buildItem(recents[j], pinned.length === 0 && j === 0));
      }
    }

    focused = 0;
  }
```

Replace the `input.value === ""` branch in the existing renderQuery invocation / openPalette flow so the empty input shows this content. Specifically, update `openPalette` to call `renderEmptyContent()` instead of `renderEmptyState(...)`, and update `renderQuery(q)` to call `renderEmptyContent()` when `q === ""`.

- [ ] **Step 4: Tiny CSS — `.palette__group-header`**

Append to `internal/web/static/app.css`:

```css
.palette__group-header {
    padding: 0.625rem 0.75rem 0.25rem;
    font-size: 0.6875rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--fg-muted);
    pointer-events: none;
}
```

Pico's `ul > li` styles may affect this — verify no extra margin/bullet.

- [ ] **Step 5: Build, test, E2E**

```bash
rm -f internal/web/templates/*_templ.go
templ generate
go build ./...
go test ./internal/web/ -count=1 | grep -v TestLDAPIntegration | grep -E "^(---|FAIL|ok)"

# E2E palette test — does it still pass with empty-state content?
go test -tags e2e ./internal/e2e/ -run TestAxeAAA_LoginPage -v
go test -tags e2e ./internal/e2e/ -run TestHomeV2 -v
```

- [ ] **Step 6: Commit**

```bash
git add internal/web/templates/palette_v2.templ \
        internal/web/templates/palette_helpers.go \
        internal/web/templates/home_v2.templ \
        internal/web/templates/users_v2.templ \
        internal/web/templates/groups_v2.templ \
        internal/web/templates/computers_v2.templ \
        internal/web/palette_context.go \
        internal/web/home_handler.go \
        internal/web/users_v2_handler.go \
        internal/web/groups_v2_handler.go \
        internal/web/computers_v2_handler.go \
        internal/web/static/js/v2-palette.js \
        internal/web/static/app.css
git commit -S --signoff -m "feat(ui): palette empty-state shows Pinned + Recent"
```

## Self-review notes

- **Pinned-through-templates plumbing** is the heavier option but keeps the JSON render server-side, no extra endpoint needed.
- **Alternative considered** but rejected for this slice: a `/api/pinned.json` endpoint with a client-side fetch — adds a round trip on palette open. Simpler to embed.
- **keyboard focus after render**: the palette keeps its existing ↑/↓/Enter handlers. Empty-state items are real options with `data-href`, so Enter works.
- **Pinned tops the list** when both pinned + recents exist, matching spec §6.6.
