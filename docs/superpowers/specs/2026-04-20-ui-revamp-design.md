# LDAP Manager — UI/UX Revamp Design

**Status:** Approved design, pre-implementation.
**Scope target:** Full app revamp — visual system, information architecture, interaction model, and frontend tech stack.
**Accessibility target:** WCAG 2.2 Level AAA (comfortable density) / Level AA + near-AAA (compact density).
**Author:** Sebastian Mendel
**Date:** 2026-04-20

## 1. Goals & non-goals

### Goals

- A modern, minimal, functional, and exploration-oriented UI for LDAP directory management.
- WCAG 2.2 Level AAA conformance in the default density on touch devices; AA + near-AAA on desktop compact.
- Pragmatic, small-footprint frontend stack. No heavy build chain.
- Preserve all current capabilities (auth, user/group/computer browsing, group membership add/remove).
- Enable ten exploratory features without overwhelming any single screen.

### Non-goals

- No changes to Go backend architecture beyond what new UI endpoints require (pinned-objects storage, search-index JSON, HTML fragment endpoints).
- No change to authentication model (session + bind credentials unchanged).
- No change to LDAP access patterns beyond adding a read-mostly pinning store.
- No multi-tenant, no RBAC overlay, no admin audit log (out of scope for this project).

## 2. Scope & phasing

The revamp is large. It ships in three phases, each independently deployable. Each phase gets its own implementation plan. **This spec covers all three phases, with Phase 1 defined in detail.**

### Phase 1 — New foundation (revamp people will see)

- Stack migration: Pico CSS + htmx + Alpine.js; delete Tailwind, TypeScript, PostCSS.
- New visual system in both themes (see §4).
- Command palette (⌘K) as primary navigation.
- Side-panel detail view for users/groups/computers.
- Pivot links inside drawer.
- Recents (client-side) + Pinned (server-side per user).
- Re-implementation of all existing routes in the new system.
- Dark/light theme and compact/comfortable density preserved.
- AAA conformance for all new surfaces.

After Phase 1, the app is the revamp. Phases 2 and 3 are incremental.

### Phase 2 — Power browsing

- Faceted filters on list pages (OU, enabled, last-logon window, member-of).
- Saved searches / named views.
- Optional OU tree rail (toggleable; default stays command-first).
- Inline attribute edit for users and groups.

### Phase 3 — Advanced

- Bulk multi-select + batch actions (add-to-group, remove, enable, disable).
- Relationship graph view (SVG-rendered node graph) with a mandatory parallel list/table representation for AAA.

## 3. Technology decisions

### Stack (Phase 1)

| Layer | Current | New | Rationale |
| --- | --- | --- | --- |
| Server | Go + Fiber + Templ | **Unchanged** | Fits well. |
| Base CSS | Tailwind v4 + PostCSS | **Pico CSS v2** (vendored single file) | Classless, built-in dark mode, accessible form defaults, ~10 KB. |
| Custom CSS | Tailwind `@apply` + custom properties | **One hand-written `app.css`** | Custom-property overrides on Pico. Hybrid theme tokens. |
| Client interactivity | TypeScript modules | **Alpine.js** (vendored) | Declarative open/close/focus/selection; fits Templ; tiny. |
| Server-driven partials | N/A (full navigations) | **htmx** (vendored) | Partial swap for drawer, list filter, pivots. Native fit with Templ. |
| Build | bun + tsc + postcss + concurrently + nodemon | **`scripts/vendor.sh`** | Refreshes three pinned static files. Go still builds via `go build`. |

**Shipped size target:** ~30 KB gzipped JS, ~15 KB CSS. Smaller than today's output after Tailwind purge + compiled TS.

> **Assumption (to verify before implementation):** Alpine.js is still actively maintained in 2026. htmx and Pico CSS confidence is high. The implementation plan MUST verify current version/status of all three before pinning.

### Removed in Phase 1 slice 7

- `tailwindcss`, `@tailwindcss/postcss`, `@tailwindcss/forms`
- `typescript`, `tsc`
- `postcss`, `postcss-cli`, `postcss-hash`, `postcss-reporter`, `cssnano`, `cssnano-preset-advanced`, `autoprefixer`, `purgecss`
- `bun`, `concurrently`, `nodemon`, `prettier-plugin-tailwindcss`
- Entire TypeScript source tree (`internal/web/static/ts/**`), replaced by Alpine directives and a small `internal/web/static/js/ldap-manager.js` for non-declarative pieces (client-side search index, localStorage recents).
- `tailwind.css`, `tailwind.config.js`, `postcss.config.mjs`, `tsconfig.json`, generated `styles.*.css`

## 4. Visual language

### 4.1 Direction

Hybrid theme — **light mode follows Direction III (Clean sans neutral)**; **dark mode follows Direction II (Terminal/IDE)**. The theme toggle literally swaps personality: casual users see the calm sans light theme; power users get the terminal-monospace dark theme.

### 4.2 Typography

| Theme | Family | Feature |
| --- | --- | --- |
| Light | `"Inter", ui-sans-serif, system-ui, -apple-system, sans-serif` | Mixed case throughout. Letter-spacing `-0.02em` on headings. |
| Dark | `ui-monospace, "JetBrains Mono", "SF Mono", Consolas, monospace` | Lowercase convention: top-level nav, page headings, section labels are lowercase. Body content (user names, attribute values) preserves authored case. |

Font sizing in `rem`. Scale resizable to 200% without reflow or clipping (WCAG 1.4.4).

### 4.3 Color palette (AAA-verified contrast)

Light (III — Clean sans neutral):

| Token | Value | Against BG | Ratio |
| --- | --- | --- | --- |
| `--bg` | `#ffffff` | — | — |
| `--bg-subtle` | `#fafafa` | `#ffffff` | 1.04 (surface) |
| `--fg` | `#0a0a0a` | `#ffffff` | 20.6 |
| `--fg-muted` | `#525252` | `#ffffff` | 8.3 |
| `--border` | `#e5e5e5` | `#ffffff` | non-text |
| `--border-strong` | `#a3a3a3` | `#ffffff` | non-text (focus ring, active borders) |
| `--accent` | `#0a0a0a` | `#ffffff` | 20.6 (primary actions = inverted background) |

Dark (II — Terminal/IDE):

| Token | Value | Against BG | Ratio |
| --- | --- | --- | --- |
| `--bg` | `#0d0d0d` | — | — |
| `--bg-subtle` | `#1a1a1a` | `#0d0d0d` | 1.11 (surface) |
| `--fg` | `#f5f5f5` | `#0d0d0d` | 17.8 |
| `--fg-muted` | `#a3a3a3` | `#0d0d0d` | 10.4 |
| `--border` | `#262626` | `#0d0d0d` | non-text (surface separators) |
| `--border-strong` | `#525252` | `#0d0d0d` | non-text (focus ring, active borders) |
| `--accent` | `#4ade80` | `#0d0d0d` | 11.2 |

All text tokens meet AAA normal-text contrast (≥7:1) against their specified background. Secondary text (timestamps, counts, meta) uses `--fg-muted`, which also meets AAA. There is no sub-AAA text token in the palette; if something doesn't meet 7:1, it is not text.

### 4.4 Density

Two modes:

- **compact** — desktop default. Tighter spacing, smaller controls. Target size ~36×36 (AA 2.5.8, not AAA 2.5.5).
- **comfortable** — touch / mobile / reduced-motion default. Larger spacing. Target size ≥44×44 (AAA 2.5.5).

Auto-selection (runs before first paint via `density-init`):

```js
const coarse = matchMedia('(pointer: coarse)').matches;
const narrow = matchMedia('(max-width: 600px)').matches;
const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
const auto = (coarse || narrow || reduce) ? 'comfortable' : 'compact';
```

User toggle in top nav always overrides the auto value; the manual preference is persisted in `localStorage` per origin.

### 4.5 Motion

`prefers-reduced-motion: reduce` → all transitions ≤ 0.01ms (already in place). Default transitions: 120ms ease-out for drawer slide-in, palette fade-in. No auto-playing motion anywhere.

### 4.6 Focus

Every focusable element: 2px solid outline with 2px offset, color `currentColor`-inverted (always visible against the element's own background in both themes). No focus rings ever removed without replacement. Focus indicators tested via E2E tab-order screenshot diff.

## 5. Information architecture

**Command-first.** The command palette (⌘K) is the primary navigation surface; it finds entities, opens details, and runs actions. Explicit top-nav links to `/users`, `/groups`, `/computers` remain as a minimal secondary row for users who prefer clicking.

### Routes (unchanged from today unless noted)

- `/` — signed-in home (greeting, search entry, pinned, recents)
- `/login` — login form (restyled; logo kept; labels made visible)
- `/logout` — session destroy
- `/users` — user list (with optional filter query params)
- `/users/:dn` — user detail full page. `?fragment=drawer` returns drawer-only markup for htmx swap. `?panel=1` opens the page with drawer context.
- `/groups` — group list
- `/groups/:dn` — group detail (same fragment/panel pattern)
- `/computers` — computer list
- `/computers/:dn` — computer detail
- `/pin`, `/unpin` — POST; toggle pinned state (new endpoints)
- `/api/search-index.json` — GET; returns the client-side search index (new endpoint)
- `/health`, `/ready` — unchanged

### Top navigation

Single row: Logo · Crumbs (when applicable) · *spacer* · ⌘K hint · Theme toggle · Density toggle · Logout.

Below (minimal secondary row, right-aligned, only when not on login/home): `Users · Groups · Computers`.

### Home page (signed-in)

- Greeting: "Hi {user.CN} — what are you looking for?"
- Large search entry (opens the full palette on focus or ⌘K)
- Two blocks side-by-side: **Pinned** (server) · **Recent** (client)
- Small user card at the bottom: DN (copyable), email (if any), direct groups count

### List + drawer

- Left pane: filtered list with mini-filter chips (enabled/disabled, free-text) — Phase 1 minimum
- Right pane: detail drawer, 360px fixed on ≥900px viewports; full-screen overlay below 900px
- Keyboard: ↑/↓ move between rows, Enter opens/focuses the drawer, Escape closes, Cmd+Enter navigates to the full page

### Detail drawer (side panel)

Mounted as a sibling of the list in list-page templates. htmx swaps its inner markup on row click without full navigation. URL gets `?panel=1` via `hx-push-url`. Drawer contains:

1. Entity name (h2) + canonical DN (monospace, copyable)
2. **Attributes** section (key-value table of LDAP attributes the user cares about)
3. **Groups** section (group tags for users; members list for groups)
4. **Pivot** section (anchors to adjacent filtered views — see §6.3)
5. **Mutations** — group add/remove forms via htmx (for users); form placement matches current app

### Command palette

A modal overlay centered at ~48px from viewport top, max 560px wide, max ~300px tall. Key interactions:

- Opens via ⌘K / Ctrl+K / ⌘/ / Ctrl+/ from anywhere; a `/` key shortcut on focus-free body also opens.
- `Esc` closes and restores focus to the opener.
- Input is a combobox (`role="combobox"` + `aria-expanded` + `aria-controls`).
- Results list is a `role="listbox"` with option groups for Users · Groups · Computers · Actions.
- Empty query state shows: Pinned, then Recent.
- `Enter` opens the detail drawer (on list pages) or navigates to full page (elsewhere).
- `Cmd+Enter` always navigates to full page.
- `Tab` from a focused option reveals that option's actions (Phase 1: just "open full page"; Phase 2+: more).

## 6. Feature design (Phase 1)

### 6.1 Command palette — search index

- On first open per session, the client fetches `/api/search-index.json` and caches in `sessionStorage`.
- Index shape:

  ```json
  [
    {"type": "user",     "dn": "cn=...", "cn": "bob.ops", "sam": "bob", "ou": "ou=Eng,...", "alias": ["Bob Operator"], "enabled": true},
    {"type": "group",    "dn": "cn=...", "cn": "admins",  "ou": "ou=Groups,...", "memberCount": 7},
    {"type": "computer", "dn": "cn=...", "cn": "laptop-42", "ou": "ou=Computers,..."}
  ]
  ```

- Server response is derived from the existing `ldap_cache` (no new LDAP calls per request). Cache invalidation: the endpoint sets `ETag` = cache-version hash; client respects `If-None-Match`.
- Client-side matching: substring + initials + score-by-recency + score-by-type-weight. Implementation target: < 100 lines of plain JS. No fuzzy-search library.
- Actions (e.g. "Add alice to group…") are synthesized client-side based on focus context.
- For directories > 10k entries the index endpoint may exceed 1 MB. Phase 1 accepts this. Later phases can add server-side search (`/api/search?q=…` returning scored results) as a transparent drop-in.

### 6.2 Detail drawer — URL-driven htmx swap

- Row markup: `<a href="/users/:dn?panel=1" hx-get="/users/:dn?fragment=drawer" hx-target="#drawer" hx-push-url="true" hx-swap="innerHTML">…</a>`
- The `href` is the full-page URL so middle-click / Cmd-click still opens a new tab with the full page.
- `<div id="drawer" aria-live="polite" role="complementary" aria-labelledby="drawer-title">` — initial content: "Select an item to see details" empty state.
- Server templates provide two renderings per entity: full-page (`/users/:dn`) and fragment (`/users/:dn?fragment=drawer`). Both share the same inner content templ component to avoid drift.
- Close control: visible close button in drawer header (`aria-label="Close detail panel"`) + Escape + click-on-backdrop (below 900px only).
- On close, browser URL drops `panel=1` via `history.replaceState`.

### 6.3 Pivot links

Every detail view includes a "Pivot" section with plain `<a>` links to filtered list views. Clicking a pivot in drawer mode replaces the list via htmx; clicking while on a full-page detail view is a normal navigation.

Minimum pivots, Phase 1:

- User drawer: "Open full page", "Other members of {group}" for each of user's groups (max 3 shown; more via palette), "Other users in {ou}"
- Group drawer: "Members of this group", "Open full page"
- Computer drawer: "Open full page", "Other computers in {ou}"

### 6.4 Recents

- Client-side only. Stored in `localStorage` under key `ldap-manager:recents:<session-user-dn>`.
- Capped at 10 entries; FIFO eviction.
- Each entry: `{type, dn, cn, lastSeenAt}`. Entry pushed on any detail view (drawer open or full page).
- Rendered on home and in empty-query palette.
- Cleared when the user logs out (hook into logout form submit).

### 6.5 Pinned

- Per-user server-side bookmarks, stored in a new bbolt bucket `pinned` keyed `{session-user-dn}/{target-dn}`.
- Endpoints:
  - `POST /pin` with `target=<dn>` form field (CSRF-protected) → creates bucket entry; returns 204
  - `POST /unpin` with `target=<dn>` → removes bucket entry; returns 204
- UI: star icon in drawer header (`aria-pressed="true"` when pinned). Click toggles via htmx.
- Rendered on home and above recents in empty-query palette.

> **Assumption:** bbolt persistence is acceptable because sessions already use bbolt (`session.bbolt`). Alternative (LDAP attribute storage) is available but out of scope for Phase 1.

### 6.6 Home page behavior

- When `/` is requested without session, redirect to `/login`.
- Greeting uses `user.CN()` fallback to `user.SAMAccountName` if CN is empty.
- Search entry is visually prominent but functionally identical to the palette — focusing and typing opens the full palette overlay on first keystroke.
- Pinned block shows up to 8 entries with an overflow "more" link that opens the palette filtered to `pinned:`.
- Recent block shows up to 8 entries.
- Both blocks are `<nav aria-label="Pinned">` and `<nav aria-label="Recent">`.

### 6.7 Login page

- Keep logo (`/static/logo.webp`).
- Change: replace `sr-only` labels with visible labels for AAA.
- Keep CSRF, keep autocomplete attributes, keep flashes.
- Visual: card centered, field widths match the command palette's input width for consistency.
- No htmx/Alpine; plain form POST.

### 6.8 Top nav

- Sticky to viewport top, full width, 44px tall (touch) / 36px (compact mouse).
- Logo is a link to `/`.
- Crumbs appear after logo on detail pages: `Users › bob.ops`.
- Right-aligned cluster: ⌘K hint · theme toggle · density toggle · logout icon.
- Secondary row (hidden on mobile): three text links `Users · Groups · Computers`. Active link uses the current active styling (inverted background).

## 7. Accessibility strategy

### 7.1 Automated verification in CI

- **Contrast** — Go script parses CSS custom properties and computes WCAG contrast ratios for every documented text-on-surface pair. Any pair < 7:1 fails the build.
- **axe-core** — runs as part of the existing Playwright E2E suite, one pass per page, AAA ruleset enabled. Any violation fails the build.
- **Tab-order snapshot** — new E2E test tabs through every page and records focus outline + visibility per stop. Deviation from the committed snapshot fails the build (accept intentional diffs by updating snapshot in the same PR).
- **Target size** — E2E asserts every interactive element in comfortable density is ≥44×44 CSS pixels.

### 7.2 ARIA patterns

- Command palette: `role="combobox"` + `aria-expanded` + `aria-controls` + `aria-activedescendant`. Results `role="listbox"` + grouped with `role="group"` + `aria-labelledby`. Each result `role="option"` with `aria-selected` for focused item.
- Detail drawer: `role="complementary"` + `aria-labelledby="drawer-title"` + `aria-live="polite"`. Announces "Loaded details for {name}" on swap.
- Pinned/Recents: `<nav aria-label="…">`.
- Flash messages: `role="status"` + `aria-live="polite"`.

### 7.3 Keyboard

- Skip link to main (first tabbable on every page).
- ⌘K / Ctrl+K opens palette from any page; Esc closes and restores focus.
- Drawer traps focus while open in mobile overlay mode; in desktop split mode, focus returns to opener on Escape.
- No keyboard traps on the list, drawer, or palette per WCAG 2.1.3 AAA.
- Global shortcuts documented in a `Help` overlay (`?` key).

### 7.4 Conformance statement

To be added to `README.md` and `docs/operations/security-configuration.md`:

> "LDAP Manager conforms to WCAG 2.2 Level AAA in *comfortable* density (the default on touch devices, mobile viewports, and under `prefers-reduced-motion`). In *compact* density (the default on desktop), the application conforms to Level AA, meeting all Level AAA success criteria except 2.5.5 Target Size (Enhanced)."

### 7.5 Deferred AAA items (documented as known limitations)

- Phase 3 graph view. AAA for node-graph visualisations is its own research. Ship a parallel list/table representation.
- Compact density's 36×36 target size. Accepted as a density-preference tradeoff.

## 8. Data & endpoints

### 8.1 New endpoints (Phase 1)

| Method | Path | Purpose | Auth | CSRF |
| --- | --- | --- | --- | --- |
| GET  | `/api/search-index.json` | Client-side search index | session | n/a (GET) |
| POST | `/pin`                   | Pin an entity | session | yes |
| POST | `/unpin`                 | Unpin | session | yes |
| GET  | `/users/:dn?fragment=drawer` | Drawer fragment for user | session | n/a |
| GET  | `/groups/:dn?fragment=drawer` | Drawer fragment for group | session | n/a |
| GET  | `/computers/:dn?fragment=drawer` | Drawer fragment for computer | session | n/a |

### 8.2 New storage

- `pinned` bucket in the existing bbolt DB. Key: `{sessionUserDN}/{targetDN}` UTF-8 bytes. Value: `{"createdAt": RFC3339}` JSON.
- No migration required; empty bucket on first run.

### 8.3 Search-index generation

The `ldap_cache` package already holds users, groups, computers in memory. A new function `BuildSearchIndex() []SearchEntry` materialises them into the JSON shape. Called on every request (cheap; no LDAP round-trip). ETag = hash of cache version counters.

## 9. Migration plan

Slices, each independently shippable:

1. **Pre-work** — vendor files (pico, htmx, alpine), new `app.css`, base templ supports feature flag. No user-visible change.
2. **Login** — restyle, visible labels. Verifies theme/density/contrast/focus on simplest page.
3. **Home + shell** — new top nav, home page, pinned backend, recents client-side, minimal ⌘K palette navigating to old-style routes.
4. **Users list + drawer** — htmx-driven row swap, fragment endpoint, pivots, membership forms.
5. **Groups list + drawer** — same pattern.
6. **Computers list + drawer** — read-only, simplest.
7. **Command palette v2** — Actions group, pinned/recents in empty state, fuzzy scoring.
8. **Cleanup** — delete Tailwind/TypeScript/PostCSS/bun tooling, update Dockerfile/Makefile/AGENTS.md/package.json (or remove).

Between slices, a feature flag can toggle any single route back to the old shell for emergency rollback during rollout.

Per-slice CI additions (introduced in pre-work, ratcheted per slice):

- axe-core AAA pass
- contrast unit test (CSS-custom-props → ratio table)
- Playwright visual snapshot baseline

Phase 2 and Phase 3 each get their own design spec and implementation plan; they are not implemented in this project.

## 10. Open assumptions (verify before lock-in)

1. Alpine.js is actively maintained as of 2026-04 and suitable for a production app. Verify during implementation-plan phase.
2. Pico CSS v2 remains the preferred classless framework in 2026-04. Verify latest stable version and any AAA-relevant CSS default changes.
3. htmx v2 is the current stable line (v1 supported but not default). Verify.
4. The existing `ldap_cache` can be queried synchronously on every `/api/search-index.json` request without measurable latency for directories ≤ 10k entries. Verify with existing benchmarks or add one.
5. The existing session bbolt store can host an additional bucket without backwards-incompatibility with current deployments. Verify by reading current bbolt initialization.

## 11. Rejected alternatives

- **Keep Tailwind, add htmx/Alpine on top.** Rejected: the goal explicitly includes reducing tooling footprint. Tailwind + PostCSS + TypeScript survive as build-chain weight even without class-utility changes.
- **Full SPA with a framework (React/Vue/Svelte).** Rejected: breaks the Go + Templ server rendering model; adds an API-contract burden; offers nothing over htmx for this app's interactivity shape.
- **Tree-first or dense 3-pane explorer as primary IA.** Rejected in favour of command-first after comparison (see the visual mockups under `.superpowers/brainstorm/`). Tree-first considered for Phase 2 as an optional toggle.
- **Single density only.** Rejected: user preference for preserving compact as desktop default with near-AAA tradeoff is deliberate and acceptable.
- **`prefers-reduced-motion` decoupled from density auto-select.** Considered and rejected in favour of the current bundled semantic ("comfortable = more-accessible bundle"), overrideable by user toggle.
