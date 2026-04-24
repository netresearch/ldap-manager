# LDAP Manager — Phase 3 Relationship Graph View Design

**Status:** Approved design, pre-implementation.
**Scope target:** Entity-centred relationship graph (users ↔ groups ↔ computers ↔ OUs) reached from a dedicated `/graph?entity=<dn>` route, plus a `List | Graph` mode toggle on `/users`, `/groups`, `/computers`.
**Accessibility target:** WCAG 2.2 Level AAA (comfortable density) / Level AA + near-AAA (compact density) — matches the Phase 1 baseline.
**Parent spec:** [`2026-04-20-ui-revamp-design.md`](./2026-04-20-ui-revamp-design.md) §2, §7.5. Supersedes [`2026-04-20-ui-revamp-phase-3-graph-view-deferred.md`](./2026-04-20-ui-revamp-phase-3-graph-view-deferred.md).
**Author:** Sebastian Mendel
**Date:** 2026-04-24

## 1. Goals & non-goals

### Goals

- Let operators see who/what is connected to a given user, group, computer, or OU at a glance — "bob's world" as a single picture.
- Support exploration: clicking a neighbouring node opens its drawer (pivot) or, when the node is still collapsed, expands the graph to include *its* neighbours (click-to-expand).
- Preserve Phase 1's AAA conformance on the new surface, including an always-visible flat edge table that screen-reader and keyboard-only users get by default (not behind a toggle).
- Reuse the existing cache and endpoint patterns (`/api/search-index.json`, ETag on cache-version hash) so the graph has no new LDAP traffic.
- Ship entirely within the vendored stack (Pico CSS + htmx + Alpine CSP build + plain JS). No new frontend dependencies.

### Scope posture

- No feature flag. The feature is additive; rollback means reverting the drawer pivot links and the routes. Nothing outside the graph surface depends on it.
- Single branch, atomic commits, slice-by-concern (§11). No parallel track.

### Non-goals

- No continuous force-directed layout animation. Static concentric layout only (also required by `prefers-reduced-motion`).
- No graph-side editing in v1 (no drag-to-add-to-group, no edge-remove action). Edits happen in the existing drawer forms the graph pivots into.
- No `managedBy` / `managedByUser` edge types in v1. Only `memberOf` and OU containment. These are scoped-out but the JSON shape is extensible.
- No saved graph layouts, no named graph views, no multi-focus graphs.
- No performance work beyond capping the default depth and the rendered node count. Directories with >200 neighbours at depth 2 get a visible overflow indicator rather than a hairball.

## 2. Scope

The feature ships in a single branch of atomic commits, no feature flag. Rolling back means reverting the pivot links and the routes. The graph is additive; nothing depends on it.

### 2.1 Two entry points

1. **Dedicated route** — `GET /graph?entity=<dn>&depth=<N>` renders a page dedicated to exploring the graph around one entity. Reached via a "View relationships" pivot in the drawer on every entity type (user, group, computer, OU). Supports a depth slider (values 1 / 2 / 3; default 2) and click-to-expand on collapsible nodes.
2. **List-page mode toggle** — `GET /users?view=graph&<existing filters>` (and identical shape for `/groups`, `/computers`) flips the list surface into a graph view of the currently filtered set plus each entity's direct groups. Fixed depth 1 — the entity-centred slider is not available here, the rationale in §7.

The two share the same rendering code (§5) and the same parallel-table code (§6). They differ in what populates the node/edge JSON.

### 2.2 Deferred from this spec

- Editing from within the graph (remove-from-group via edge click, drag-to-add). Rejected in §12.
- Broader edge types. v1 ships `memberOf` (user/computer → group, group → group) and `contains` (OU → user/computer). `managedBy`, `manager`, and site topology edges are scoped-out; the JSON shape leaves room.

## 3. Architecture

### 3.1 Data flow

```
Browser                                   Server (Fiber)
───────                                   ──────────────
GET /graph?entity=<dn>&depth=2      →    handleGraphV2
                                          ├─ buildGraph(dn, depth)  ──→  ldap_cache (no LDAP)
                                          ├─ assign (ring, angle) per node
                                          └─ render graph_v2.templ
                                              ├─ embeds GraphJSON (inline <script type="application/json">)
                                              ├─ renders SSR <table> fallback
                                              └─ references v2-graph.js

Browser (after paint)
───────
v2-graph.js reads the JSON, draws the SVG canvas sized to viewport,
wires pan/zoom + click handlers. Canvas and the SSR table share the
same source-of-truth JSON so they cannot drift.

Click-to-expand on node X
───────
GET /api/graph.json?entity=<X-dn>&depth=1
→ handleGraphJSON → buildGraph(X, 1) → JSON (same shape)
→ client merges new nodes/edges into its in-memory state, assigns
  angles to new nodes in the next ring, patches the SVG, appends
  rows to the SSR table via DOM manipulation.
```

### 3.2 Package layout

| Concern | Path | Notes |
| --- | --- | --- |
| Graph builder (BFS walk over cache) | `internal/ldap_cache/graph.go` | New. Pure function over cached data. No LDAP access. |
| Graph HTML handler | `internal/web/graph_v2_handler.go` | New. Validates DN, calls builder, renders template. |
| Graph JSON handler | `internal/web/graph_v2_handler.go` | Same file. Returns `application/json` for click-to-expand. |
| Layout math (Go side) | `internal/ldap_cache/graph.go` | `assignConcentric(nodes, focalDN)` — ring + angle per node. |
| Graph template | `internal/web/templates/graph_v2.templ` | New. SSR table + SVG shell + inline JSON. |
| List-page mode | `internal/web/{users,groups,computers}_v2_handler.go` | Extend existing handlers: when `view=graph`, render the graph template with the filtered set. |
| Client canvas | `internal/web/static/js/v2-graph.js` | New. Plain JS, no build step. ~200–300 LOC target. |
| CSS | `internal/web/static/app.css` | Additive section. Concentric node styling, focus ring, hover state. |
| Pivot links into graph | `internal/web/templates/{user,group,computer}_drawer_fragment.templ` | One pivot anchor per drawer: "View relationships →". |

## 4. Data model

### 4.1 JSON shape

Single canonical shape, returned by both the HTML handler (embedded in the page) and the JSON endpoint (click-to-expand):

```json
{
  "focus": "cn=bob.ops,ou=Engineering,dc=example,dc=com",
  "depth": 2,
  "nodes": [
    {
      "dn": "cn=bob.ops,...",
      "type": "user",
      "label": "bob.ops",
      "ring": 0,
      "angle": 0,
      "enabled": true,
      "expandable": false
    },
    {
      "dn": "cn=admins,ou=Groups,...",
      "type": "group",
      "label": "admins",
      "ring": 1,
      "angle": 0.523598,
      "memberCount": 7,
      "expandable": true
    },
    {
      "dn": "ou=Engineering,...",
      "type": "ou",
      "label": "ou=Engineering",
      "ring": 1,
      "angle": 2.094395,
      "expandable": true
    }
  ],
  "edges": [
    { "source": "cn=bob.ops,...", "target": "cn=admins,...", "kind": "memberOf" },
    { "source": "ou=Engineering,...", "target": "cn=bob.ops,...", "kind": "contains" }
  ],
  "overflow": {
    "truncated": false,
    "rendered": 42,
    "available": 42
  }
}
```

Fields:

- **`focus`** — the DN the graph is centred on (ring 0) for entity-centred requests. For list-page Graph mode, which has no single focal entity, `focus` is the empty string and no node has ring 0 (see §4.4 for the alternative ring convention).
- **`depth`** — integer 1 / 2 / 3. Matches the URL slider value.
- **`nodes[].ring`** — 0 for the focus, 1 for direct neighbours, 2 for second-degree, 3 for third-degree (only reached when depth=3 or via click-to-expand).
- **`nodes[].angle`** — radians in `[0, 2π)`. Server-assigned by sorting neighbours within each ring by DN and distributing them evenly: `angle[i] = i × 2π / count(ring)`. Deterministic, stable across refreshes as long as the cache contents are the same.
- **`nodes[].expandable`** — true when the node has at least one neighbour not already in the graph. Group and OU nodes are almost always expandable beyond ring 1; user/computer nodes rarely are (they mostly link "outward" to groups). Governs the ⊕ badge and the click-to-expand affordance.
- **`nodes[].memberCount`** / **`nodes[].enabled`** — type-specific display hints. Extensible.
- **`edges[].kind`** — one of `memberOf` (user/computer → group, group → group) or `contains` (OU → user/computer). Future: `manager` / `managedBy`.
- **`overflow`** — a global indicator. If the BFS walk discovered more than the cap (§4.3), nodes beyond the cap are dropped from both `nodes` and `edges`, and `truncated: true`, `available` reports the full count. The UI shows a "showing N of M" pill.

### 4.2 Endpoint contract

| Method | Path | Purpose | Response | Auth | CSRF |
| --- | --- | --- | --- | --- | --- |
| GET | `/graph?entity=<dn>&depth=<N>` | HTML page (canvas + SSR table + JSON) | `text/html` | session | n/a |
| GET | `/api/graph.json?entity=<dn>&depth=<N>` | JSON for click-to-expand | `application/json` | session | n/a |

Both endpoints:

- 400 if `entity` is missing or not a parseable DN (via `ldap.ParseDN`).
- 404 if the DN is not present in any cache (user, group, computer, or OU). OUs are cached implicitly — any DN with an `ou=` RDN counts as an OU focus; its neighbours are everything directly under it.
- Default `depth=2` when the parameter is missing. Clamp to `[1, 3]` — any out-of-range input rounds to the nearest valid value and returns 200 (not 400).
- Set `ETag` to `"` + `sha256(<marshalled JSON body>)[:16]` + `"`, mirroring the `/api/search-index.json` pattern: the hash over the response body is stable across requests as long as the builder inputs (focus, depth) and the underlying cache contents don't change. Clients respect `If-None-Match`.
- `Cache-Control: private, must-revalidate`.

The HTML endpoint also sets `Content-Security-Policy: script-src 'self'` inherited from the site CSP — the embedded `<script type="application/json">` is data, not script, so it is CSP-safe.

### 4.3 Caps

Two independent caps, both server-side:

- **Per-ring cap:** 60 nodes. Rings 1 and 2 sorted by (entity type ascending, CN ascending). Nodes beyond the cap dropped; `overflow.truncated = true`.
- **Total cap:** 200 nodes. Even if no single ring exceeds 60, the cumulative walk aborts once the total reaches 200.

Caps chosen from the same rationale as Phase 1's search-index cap: a concentric layout at 200 dots on a 900-px-wide canvas is still legible at comfortable density. Beyond 200 it becomes a hairball regardless of layout. The parallel table degrades gracefully — 200 rows is a scroll, not a render problem.

### 4.4 List-page Graph mode

`GET /users?view=graph&<filters>` builds a different graph:

- **Nodes:** the filtered users (same set the list view would show), plus each user's direct groups (deduped). No second-degree, no OUs.
- **Focus:** the filter combination itself. There is no single focal entity — the canvas uses a modified concentric layout with *two* rings: inner ring = groups (because they are the connective tissue), outer ring = users. Rings are the same geometry, but the semantic "distance from focus" no longer applies.
- **Edges:** only `memberOf` edges between filtered users and their groups. Edges between two filtered users (via a shared group) are implicit via the group node, not direct.
- **Depth:** fixed at 1. The slider is hidden.
- **Expand:** disabled. Click-to-pivot on a node opens its drawer, same as everywhere else, but expansion would defeat the bounded-set rationale of the filter.

The same JSON shape and the same template render the list-page graph — the handler just builds a different node/edge set.

## 5. Client rendering

### 5.1 No graph library

The concentric layout is closed-form — `x = cx + r · cos(angle); y = cy + r · sin(angle)`. The server delivers `(ring, angle)`; the client multiplies by a viewport-dependent radius and centre. No force simulation, no layout solver.

Rationale (superset of Phase 1's stack principles):

- The site CSP is `script-src 'self'` with no `unsafe-eval`. Cytoscape.js uses the `Function` constructor in some code paths; d3 has modules that parse CSS property strings at runtime. Either would require a CSP-compliance spike before adoption. Writing the 300 LOC ourselves sidesteps that entirely.
- Keyboard and screen-reader support for node graphs is not a library strong point — we would end up hand-rolling the AAA layer on top of either library anyway.
- Bundle budget from the Phase 1 spec was ~30 KB gzipped JS total. We are currently at ~25 KB (Alpine CSP + htmx + our own JS). Adding 100 KB gzipped for Cytoscape would 3× the budget for one feature.
- Supply-chain surface is a direct concern; see the Ofelia trivy-action incident referenced in AGENTS.md. Zero new dependencies == zero new supply-chain risk.

### 5.2 v2-graph.js — life cycle

```
DOMContentLoaded
├─ find <script id="graph-data" type="application/json">, JSON.parse it
├─ find <svg id="graph-canvas">
├─ compute layout radii from viewport size (min dim × 0.35 for outer ring)
├─ render: one <g class="graph-node"> per node, positioned via transform
├─ render: one <line class="graph-edge"> per edge, endpoints from node positions
├─ attach: pan (mousedown + mousemove on canvas, translates <g class="graph-viewport">)
├─ attach: zoom (wheel → scale the viewport <g>, clamp 0.3–3.0)
├─ attach: keyboard nav (Tab cycles nodes in ring-then-angle order)
├─ attach: click/Enter on node → pivot via htmx to drawer
└─ attach: click on expandable node badge → fetch /api/graph.json, merge, re-layout
```

Pan/zoom uses SVG's native `transform="translate(x,y) scale(s)"` on a wrapper `<g>`. No CSS transforms, no transform-matrix math required — SVG handles it.

### 5.3 Viewport responsiveness

On `resize`, the client recomputes the layout radii from the new viewport and re-applies node transforms. No re-fetch. Rings stay in their proportional positions.

On `prefers-reduced-motion: reduce`, pan/zoom transitions are `0.01ms` (existing site default). Click-to-expand still works but the incoming nodes fade in at 0ms instead of 120ms.

### 5.4 Click-to-expand state

The client maintains an in-memory `{ nodes: Map<DN, Node>, edges: Set<string> }`. When an expandable node is clicked:

1. Send `GET /api/graph.json?entity=<clicked-dn>&depth=1`.
2. Merge the returned nodes into the local Map (deduped by DN; existing nodes keep their position).
3. For each *new* node, assign an angle in the ring it claims: find the next unoccupied slot in that ring by spacing `2π / (existing + new)`; if the ring already has its cap (§4.3), push into the next ring out with a "overflow" style.
4. Patch the SVG: add `<g class="graph-node graph-node--added">` for each, `<line>` for each new edge. Transition opacity 0→1 over 120ms unless reduce-motion.
5. Patch the SSR table: append `<tr>` for each new edge, sort-order-preserving insert.
6. Announce via `aria-live` on the `#graph-announce` region: "Expanded {label}: added {N} nodes."

The clicked node's `expandable` flag is flipped to false and its ⊕ badge is removed.

## 6. AAA parallel view

The flat edge table sits **below** the SVG canvas on every graph page. Always visible. No toggle, no collapse. Shares the same source-of-truth JSON as the canvas — there is no client-side drift possible.

### 6.1 Table shape

One row per edge. Columns:

| Column | Meaning |
| --- | --- |
| Ring | 1 or 2 (the edge's farther endpoint's ring). |
| From | Source entity (DN + type + label). Linked to the entity's full page. |
| Edge | Edge kind. Human label: "member of", "contains". |
| To | Target entity (DN + type + label). Linked. |
| Type | Target type. |

Rows are sortable by every column. Sort state is URL-preserved (e.g. `?sort=ring`), and the handler re-orders rows server-side — so a shared link, a JS-disabled reload, and an SSR render all agree. Clicking a column header submits a GET with the new `?sort=` parameter via a plain `<a>` (no JS dependency). Default sort: (Ring asc, From CN asc, To CN asc).

Keyboard: `↑`/`↓` move between rows, `Enter` opens the drawer for the row's *target* entity via the same htmx pivot used by the SVG nodes, `Cmd+Enter` opens the full page.

### 6.2 ARIA scaffolding

- `<div id="graph-announce" role="status" aria-live="polite" class="sr-only">` sits outside the SVG. Receives "Focused {label}" / "Expanded {label}: added N nodes" / "Loaded details for {label}".
- The SVG itself is `role="img" aria-labelledby="graph-title" aria-describedby="graph-desc"`. `<title id="graph-title">` = "Relationship graph for {focus.label}"; `<desc id="graph-desc">` = a one-sentence summary ("Shows 12 direct neighbours and 23 second-degree connections"). Both are text-only so screen readers read them verbatim.
- Each SVG node is a focusable `<g tabindex="0" role="button" aria-label="Group admins (7 members). Member of all-staff. Press Enter to open.">` — the aria-label is generated server-side from the node's type, label, and first-two outgoing edges.
- The SSR table is fully announced by AT without any ARIA additions; `<table>` + `<th scope="col">` is sufficient.

### 6.3 Conformance claim

The graph view conforms to **WCAG 2.2 AA** (same as Phase 1's compact density). **AAA** is claimed only for the parallel table — the SVG canvas itself satisfies all testable AAA criteria except 1.4.11 "Non-text Contrast" for edge lines (3:1 against the background token is met, but the 7:1 bar that AAA implies for graph lines as "meaningful content" is subjective; we claim 3:1 non-text per AA). The README accessibility conformance statement gets a sentence added: "Graph relationship view meets AA; AAA-equivalent information is always rendered in the adjacent edge table."

## 7. Interaction model

### 7.1 Dedicated `/graph` route

- **Hover:** highlights the node and all its incident edges (other edges fade to 0.3 opacity). No content change, pure visual affordance.
- **Click on non-expandable node:** same as clicking the row's target in the drawer-pivot flow elsewhere — htmx fetch of `/users/:dn?fragment=drawer` (or group/computer/OU equivalent), `hx-target="#drawer"`. This is identical to every other pivot in the app.
- **Click on expandable node:** expand (§5.4). A small ⊕ badge in the node corner makes the affordance obvious. The same node becomes non-expandable after its ring-1 neighbours are in the graph; subsequent clicks pivot.
- **Depth slider:** labelled `<input type="range" min=1 max=3 step=1 value=2 aria-label="Graph depth">`. Changing the value navigates to `/graph?entity=<same>&depth=<new>` (full page nav, not an in-place refetch — depth is part of the URL and must be shareable).
- **Pan:** mousedown + drag on canvas background. Two-finger pan on touch. Arrow keys scroll the viewport when the canvas has focus.
- **Zoom:** mouse wheel + Ctrl on desktop; pinch on touch; `+` / `-` keys when the canvas has focus. Clamp `[0.3, 3.0]`.
- **Keyboard focus order:** depth slider → canvas → first node in ring order → subsequent nodes → table's first row → subsequent rows. Explicit `tabindex="0"` on the canvas and each node.

### 7.2 List-page Graph mode

All of the above, minus the depth slider and minus click-to-expand (expandable flag is always false). The `List | Graph` toggle is a segmented control at the top of the list page, keyboard-accessible (`aria-pressed`).

### 7.3 R3 interaction tier

This matches the "R3" choice from the brainstorm: **click-to-pivot + click-to-expand**. Editing (R4) is explicitly out of scope.

## 8. Data & endpoints

Consolidated from §4.2 and §2.1:

| Method | Path | Purpose | Auth | CSRF |
| --- | --- | --- | --- | --- |
| GET | `/graph?entity=<dn>&depth=<N>` | Entity-centred graph page | session | n/a |
| GET | `/users?view=graph&<filters>` | List-page graph mode (users) | session | n/a |
| GET | `/groups?view=graph&<filters>` | List-page graph mode (groups) | session | n/a |
| GET | `/computers?view=graph&<filters>` | List-page graph mode (computers) | session | n/a |
| GET | `/api/graph.json?entity=<dn>&depth=<N>` | JSON for click-to-expand | session | n/a |

No new storage. Everything derives from the existing `ldap_cache`.

## 9. Accessibility strategy

Ratchets the Phase 1 CI gates:

- **Contrast:** new tokens `--graph-edge`, `--graph-edge-focus`, `--graph-node-border`, `--graph-node-focus-ring` added to `app.css` with contrast ratios verified by `TestAppCSSTokensMeetAAAContrast` (existing CI).
- **axe-core:** the existing Playwright suite runs one pass per page; `/graph?entity=…` and each list-page graph mode get their own test.
- **Target size:** clickable nodes render at 44×44 CSS pixels in comfortable density, 36×36 in compact. Enforced by the existing E2E target-size assertion.
- **Tab-order snapshot:** the new surfaces are added to the committed snapshot set.
- **aria-live verification:** a new E2E test opens `/graph`, clicks an expandable node, and asserts that `#graph-announce` received the "Expanded X: added N nodes" message.
- **Reduced-motion:** E2E test in `prefers-reduced-motion: reduce` mode asserts zero transition duration on node add + zero pan/zoom easing.

The parallel edge table is the AAA claim's load-bearing element — every interaction available on the canvas has a table equivalent. An E2E test exercises the full keyboard flow (`Tab` into table, `↑/↓`, `Enter` on a row, drawer opens for the row's target) with no mouse input.

## 10. Testing strategy

### 10.1 Unit

- `internal/ldap_cache/graph.go` — `buildGraph(focus, depth)` tested with synthetic cache contents. Cases: focus=user / group / computer / OU; depth=1 / 2 / 3; truncation at cap; cycles in group-of-group membership (no infinite walk); unknown focus DN (returns nil, ErrNotFound).
- `internal/ldap_cache/graph.go` — `assignConcentric` tested for determinism (same input → same angles), even distribution (angles differ by exactly `2π / count`), and ring-0 always at `angle=0`.
- `internal/web/graph_v2_handler.go` — ETag stability (same cache-version + same focus → same ETag), 400 on bad DN, 404 on unknown DN, 200 with JSON-parseable body on happy path, depth clamping.

### 10.2 Integration

- Renders `/graph?entity=<seeded-user-DN>` against the OpenLDAP test container and asserts the embedded JSON contains the expected direct groups.
- `/api/graph.json?entity=<same>&depth=1` returns identical node/edge subset to the embedded JSON.
- List-page graph mode: `/users?view=graph&ou=Engineering` returns only users from that OU and their direct groups.

### 10.3 E2E (Playwright)

- Happy path: navigate to a user detail, click "View relationships" pivot, assert SVG is present, assert table is present, assert both reference the same focus label.
- Click-to-expand: click an expandable group node, assert new nodes appear in SVG and new rows appear in table; assert `#graph-announce` received the update.
- Keyboard-only: Tab through the depth slider, the canvas, each node, each table row; Enter on the third row; assert a drawer opens for that row's target.
- Reduced-motion: repeat the happy path with `prefers-reduced-motion: reduce`; assert no transitions fired.
- AAA table-alone: scroll past the SVG (or use CSS to hide it), assert every interaction (pivot, sort, expand via table) still works.

## 11. Rollout

Single branch, atomic commits, no feature flag. Slicing mirrors Phase 1:

1. **Graph builder + tests** — `internal/ldap_cache/graph.go` plus unit tests. No handler, no template, no client. CI stays green.
2. **JSON endpoint** — `/api/graph.json` handler, tests, wired into the router. Still no UI.
3. **Template + SSR table** — `graph_v2.templ`, handler, CSS for the table, E2E that hits `/graph?entity=<dn>` with JavaScript disabled and asserts the table is complete.
4. **SVG canvas + interaction** — `v2-graph.js`, CSS for nodes/edges, pan/zoom, click-to-pivot, click-to-expand.
5. **List-page mode** — extend `/users`, `/groups`, `/computers` handlers with `view=graph`; add the `List | Graph` segmented control.
6. **Drawer pivots + AAA tests + README conformance statement** — "View relationships" link added to each drawer; the new E2E/axe-core tests land; README gets the graph-view conformance sentence.

Each slice is independently revertable. The worst-case rollback is `git revert` of slices 4–6, leaving the JSON endpoint in place (still useful for admins with their own tooling).

## 12. Rejected alternatives

- **Cytoscape.js or d3-force.** Rejected per §5.1: CSP + bundle + supply-chain costs all on one dependency that the concentric layout does not need.
- **Graph rendered inside the detail drawer** (a "Relationships" accordion section in the existing drawer). Rejected during brainstorming: cramped at 360 px, conflates "who is this entity?" with "who does this entity know?", and makes every drawer heavier for a feature most pivots do not need.
- **Force-directed layout.** Rejected: continuously-animating layouts fail `prefers-reduced-motion`, and they re-jumble on click-to-expand, which defeats the spatial mental model ("node X is still where I left it").
- **AAA parallel view behind a toggle** (`?view=list`). Rejected: no accessibility-assistive user should have to discover a toggle. The cost of always rendering the table is trivial; the benefit is a single artefact with no discoverability gap.
- **`view=list` URL remains supported as a rewrite that scrolls to the table anchor.** Optional; not load-bearing.
- **Depth slider absent in v1.** Considered (simpler), rejected in favour of the slider because the click-to-expand choice from the brainstorm implies an adjustable default too.
- **Full editing from the graph (R4).** Rejected per §7.3: the UX is large (drag-and-drop accessibility is its own WCAG hole), the actions already exist in drawer forms, and shipping it would delay the read surface the spec is actually about.
- **JS-only positioning (client assigns rings + angles).** Rejected: the SSR table's row-order must match the canvas rendering order for the keyboard-nav equivalence to hold; Go-side angle assignment sorted by DN is deterministic and trivially testable.
- **Server-returned pre-positioned SVG.** Rejected: no viewport-responsive sizing, click-to-expand would be an HTML-fragment swap of the whole canvas, no click-target geometry for precise hit-testing.

## 13. Open assumptions (verify before implementation)

1. The OU "node" concept is lightweight — OUs are implicit in DNs, not separate cached records. `buildGraph(focus=ou-dn, depth=1)` must enumerate every cached entity whose DN has this OU as its immediate parent. Verify the performance of that scan on a 10k-entity directory; if too slow, add an OU-to-children index alongside `dnIndex`.
2. `ldap.ParseDN` (from `github.com/go-ldap/ldap/v3`, already in use) correctly handles escaped commas inside RDN values (`cn=Last\, First,ou=…`) so the focus-DN validation does not mis-split. Verify against the existing `immediateOU` helper pattern in `internal/web/search_index.go`.
3. The cache-version hash exposed via the existing search-index ETag pattern is incremented on any cache refresh, so reusing it for the graph ETag does not need a new monotonic counter. Verify in `internal/ldap_cache/manager.go`.
4. Group-of-group membership is representable in `ldap.Group.Members` as a group DN (not only user DNs). The BFS walk needs to recognise this and follow the edge as `memberOf`. Verify with the integration test suite and, if needed, extend `PopulateUsersForGroup` to resolve nested membership.
5. `prefers-reduced-motion` is already honoured globally — the new JS must NOT add a handler that overrides it. Verify by reviewing `v2-preferences-init.js` side effects.

## 14. References

- Parent design: [`2026-04-20-ui-revamp-design.md`](./2026-04-20-ui-revamp-design.md)
- Deferred stub this supersedes: [`2026-04-20-ui-revamp-phase-3-graph-view-deferred.md`](./2026-04-20-ui-revamp-phase-3-graph-view-deferred.md)
- Search-index endpoint pattern (reused): `internal/web/search_index.go`
- Cache hook pattern (for future edit-from-graph): `internal/ldap_cache/manager.go` §OnAddUserToGroup / OnDeleteUser
