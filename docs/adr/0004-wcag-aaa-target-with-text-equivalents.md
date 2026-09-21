# 4. WCAG 2.2 AAA target, with a text equivalent for every graphical view

Date: 2026-04-24

## Status

Accepted. Enforced in CI by `internal/web/contrast_test.go` and the axe-core pass in
`internal/e2e/axe_test.go`.

## Context

The UI revamp set WCAG 2.2 Level AAA as the target rather than the more common AA. AAA raises
the contrast floor and, for anything drawn rather than written, requires a representation that
a keyboard and a screen reader can actually use. The relationship graph made this concrete:
`role="graphics-*"` ARIA patterns have uneven support across browsers and assistive technology,
so the graph was deferred until a workable approach existed. The approach that shipped is a
parallel text representation.

## Decision

AAA is the target. A graphical view ships together with a text representation that carries the
same information, and conformance is a CI gate rather than a review habit.

## Consequences

- The relationship graph ships with a server-rendered edge table beside the SVG canvas. The
  table is the accessible representation, not a fallback for browsers without JavaScript.
- Colour choices are constrained by the AAA contrast floor in both themes, which is why the
  palette is fixed in `app.css` custom properties and checked by a unit test.
- The axe gate covers five views at AAA — `/login`, `/`, `/users`, `/groups` and `/computers` —
  and `/graph` with the list-page graph modes at AA. A regression on one of those fails CI. The
  entity detail pages and `/password-expiry` have no axe pass yet, so the target holds there by
  review rather than by gate, and a new route joins the gate when it is added to the e2e suite.
- One exception is recorded and deliberate: in compact density the login page meets AA rather
  than AAA, because success criterion 2.5.5 Target Size (Enhanced) conflicts with the density
  preference. Comfortable density, the default on touch devices and narrow viewports, meets AAA.

## Source

`docs/superpowers/specs/2026-04-24-phase-3-graph-view-design.md` and
`docs/superpowers/specs/2026-04-20-ui-revamp-phase-3-graph-view-deferred.md`, as of commit
`da77c81`. The working specs and plans were removed from the tree once the work shipped.
