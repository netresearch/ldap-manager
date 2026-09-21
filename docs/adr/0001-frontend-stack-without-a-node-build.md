# 1. Frontend stack without a Node build step

Date: 2026-04-20

## Status

Accepted. Implemented in the UI revamp, Phase 1 slices 1–8.

## Context

The frontend was built with Tailwind CSS v4, TypeScript and a bun toolchain: `bun` plus
`tsc`, `postcss`, `postcss-cli`, `postcss-hash`, `cssnano`, `autoprefixer`, `purgecss`,
`concurrently` and `nodemon`. A Go binary that serves LDAP pages therefore needed a Node
toolchain to build, and contributors needed it installed before they could change a single
class name.

## Decision

The frontend ships as vendored files that need no build:

- Pico CSS v2 as the base stylesheet, classless and dark-mode aware.
- One hand-written `internal/web/static/app.css` that overrides Pico through custom properties.
- htmx for server-driven partial swaps, which fits Templ's server rendering.
- Alpine.js in its CSP build for local interactive state, introduced in slice 3.
- `scripts/vendor.sh` refreshes the pinned third-party files and verifies SHA-256 checksums
  against `scripts/vendor.lock`.

The whole Node toolchain, the TypeScript source tree under `internal/web/static/ts/`, and the
Tailwind and PostCSS configuration were removed in slice 7.

## Consequences

- `go build` is the only build. No Node version, no lockfile, no `node_modules`.
- Third-party updates are a deliberate act: edit `scripts/vendor.lock`, run `scripts/vendor.sh`,
  commit the vendored file. There is no transitive dependency tree to audit.
- There is no CSS purge step, so `app.css` is written by hand and stays small by discipline
  rather than by tooling.
- No TypeScript. Client code is plain JavaScript in `internal/web/static/js/v2-*.js` and is
  reviewed as such.

## Source

`docs/superpowers/specs/2026-04-20-ui-revamp-design.md` §3, as of commit `da77c81`. The working
specs and plans were removed from the tree once the work shipped.
