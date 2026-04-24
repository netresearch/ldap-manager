# Phase 3 Graph View — Deferred (SUPERSEDED)

**Status:** Superseded 2026-04-24 by [`2026-04-24-phase-3-graph-view-design.md`](./2026-04-24-phase-3-graph-view-design.md). Kept for historical context on why the work was originally deferred.
**Parent spec:** [`2026-04-20-ui-revamp-design.md`](./2026-04-20-ui-revamp-design.md) §2

The relationship graph view (users ↔ groups ↔ computers ↔ OUs) was in scope
for Phase 3 per the design spec §2.

It is deferred from the consolidated Phase 2+3 implementation because the
work does not fit inside a single autonomous session:

1. **AAA research required.** WCAG 2.2 AAA requires a keyboard-navigable,
   screen-reader-usable representation of the graph. `role="graphics-*"`
   ARIA patterns have uneven browser + AT support; the established pattern
   is a parallel data-table representation that mirrors the visual graph.
   Designing that parallel view is a design-plus-QA effort beyond
   single-session scope.

2. **Library evaluation.** Force-directed layouts (d3-force, cytoscape-web,
   native SVG) each have CSP and bundle-size tradeoffs that need
   evaluation against the rest of the vendored stack. The current CSP is
   `script-src 'self'` with no `unsafe-eval`, which rules out any library
   that relies on runtime source compilation.

3. **Interaction design.** Pan/zoom on touch, focus-ring visibility on
   nodes, edge highlighting on hover — all need user testing before we
   commit to a pattern. This session can't run that.

## Rough shape for future work

- `GET /graph?entity=<dn>` renders the graph centred on the entity.
- Query returns JSON:

  ```json
  {
    "nodes": [{ "id": "...", "type": "user|group|computer|ou", "label": "..." }],
    "edges": [{ "source": "...", "target": "...", "type": "memberOf|contains" }]
  }
  ```

  derived from cache membership relationships (`FullLDAPUser.Groups`,
  `FullLDAPGroup.Members`, `FullLDAPComputer.Groups`).

- Client renders via a vendored graph library (AAA-friendly layout only;
  avoid force layouts that animate continuously — keep a static
  `breadthfirst` or `concentric` layout to respect
  `prefers-reduced-motion`). ARIA live-region announces focus changes.

- `GET /graph?entity=<dn>&view=list` — AAA-compliant parallel list
  representation (entity + direct relations + second-degree relations).
  This is the WCAG-mandated text alternative and must reach feature-parity
  with the visual graph.

## Estimated effort

- 3-5 engineering days (backend JSON endpoint + frontend glue + parallel
  list view).
- 2 days of a11y testing with real screen-readers (NVDA + VoiceOver).
- 1 day of user-testing on the interaction model before sign-off.

## Dependencies / blockers

- A vendored graph-rendering library added to `internal/web/static/vendor/`
  (CSP-clean, no runtime source compilation).
- A Phase 2.5 data-endpoint refactor if the JSON shape conflicts with the
  current search-index response (`/api/search-index.json`).
