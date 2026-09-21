# 3. Strict CSP without unsafe-inline or unsafe-eval

Date: 2026-04-20

## Status

Accepted. Enforced by the helmet middleware in `internal/web/server.go`.

## Context

The site sends `default-src 'self'; style-src 'self'; script-src 'self'; …` with neither
`unsafe-inline` nor `unsafe-eval`. This was discovered to constrain the frontend during the
slice 2 bring-up: Alpine.js in its default build evaluates `x-data` and `x-on:*` expression
strings at runtime, which needs `unsafe-eval`.

## Decision

The policy stays strict. The frontend adapts to it rather than the reverse.

## Consequences

- Alpine is loaded as `@alpinejs/csp`, and component data is declared with `Alpine.data(name, fn)`
  in an external file. Inline `x-data="{…}"` does not work.
- No `<script>` blocks in templates. Even pre-paint preference initialisation lives in an
  external file loaded synchronously from `<head>`.
- htmx's `hx-*` attributes are fine, but `hx-on` handlers evaluate expressions and are avoided
  in favour of out-of-band event listeners.
- Any third-party library that compiles source at runtime is excluded. This ruled out several
  graph-layout libraries when the relationship graph was designed; the shipped graph renders
  SVG from server-provided JSON instead.
- A new dependency is checked against the policy before it is vendored, not after.

## Source

`docs/superpowers/specs/2026-04-20-ui-revamp-design.md` §3 and
`docs/superpowers/specs/2026-04-20-ui-revamp-phase-3-graph-view-deferred.md`, as of commit
`da77c81`. The working specs and plans were removed from the tree once the work shipped.
