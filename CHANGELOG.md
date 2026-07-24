# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

---

## [v1.5.0] - 2026-07-24

### Added

- **Password-expiry roster** ([#626](https://github.com/netresearch/ldap-manager/pull/626), closes [ldap-selfservice-password-changer#628](https://github.com/netresearch/ldap-selfservice-password-changer/issues/628)). A new admin-only `/password-expiry` page lists accounts whose LDAP password is expiring, resolved live via [simple-ldap-go](https://github.com/netresearch/simple-ldap-go) v1.13.0. It shows four states — expiring, must-change, never-expires, unknown — as status badges, defaults to accounts due within a window (`?days=`, default 30) with a **Show all accounts** toggle, and is reachable only by admins.
  - New `LDAP_ADMIN_GROUP` (`--admin-group`): an admin is a member of this group **or** carries Active Directory's `adminCount=1`. On OpenLDAP, which has no `adminCount`, the group is the only way to grant access. Group membership is read from `memberOf` (Active Directory populates it automatically; OpenLDAP needs the `memberof` overlay).
  - The roster is resolved live from the directory, not the background cache, which cannot compute expiry; on OpenLDAP expiry needs the `ppolicy` overlay.

### Fixed

- **DOM XSS in the command palette** ([#625](https://github.com/netresearch/ldap-manager/pull/625)). The palette now validates the navigation target before following it.
- **End-to-end tests** migrate to the renamed `playwright-go` module ([#621](https://github.com/netresearch/ldap-manager/pull/621)).

### Dependencies

- Routine Renovate/Dependabot updates across Go modules, Docker base images (Alpine 3.24.1, `docker/dockerfile` v1.25) and CI actions.

---

## [v1.4.1] - 2026-04-27

### Fixed

- **Release pipeline:** Switch to `release-go-app.yml` atomic-release orchestrator. Previous pipeline created an immutable GitHub Release before the binaries job could attach assets, causing v1.3.0 and v1.4.0 to ship without binaries or container images (HTTP 422 "Cannot upload assets to an immutable release"). The orchestrator publishes atomically at the end after binaries + container builds succeed.
- **README:** Repair broken CI/Container badges — workflow files were renamed (`quality.yml` → `ci.yml`, `docker.yml` → `container.yml`) without updating the README.

---

## [v1.4.0] - 2026-04-25

### Added

- **Phase 3 graph view (Slices 1–6):** Server-rendered relationship graph with JSON endpoint, interactive JS canvas, list-page Graph mode (toggle + persistent selection), drawer pivots, weighted layout (degree-scaled disc + parent-anchored angles), and axe-core a11y ratchet. Includes "View relationships" drawer pivot, dark-mode + zoom anchoring, scroll-anchor fixes.
- **Table view:** New per-page Table mode for `/users`, `/groups`, `/computers` with persistent List/Table/Graph selection. Server-rendered sortable column headers + client-side filter widget.

### Fixed

- `FindUsers` cache excludes AD computer accounts.
- Subheader layout, page width, dark-mode and console-mode polish across list/detail pages.

---

## [v1.3.0] - 2026-04-24

### Added

- **UI revamp Phase 1:** Command-first interface with ⌘K palette, pin/unpin, recents, detail drawer. New hybrid light/dark theme (Inter sans in light, monospace in dark). WCAG 2.2 AAA conformance on all new surfaces, verified in CI via axe-core.
- **UI revamp Phase 2:** Inline-edit for user email + description in the drawer via htmx; last-logon filter chips on `/users` (last 24h / 7d / 30d / never); toggleable OU tree rail on `/users`, `/groups`, `/computers` populated from distinct immediate-OU values in the cache.
- **UI revamp Phase 3:** Bulk add-to-group, bulk + single disable (AD-gated, adminCount-based Privileged), and bulk delete for groups + computers with session flash. Per-row checkboxes feed a floating bulk-bar that POSTs `target_dn[]` + `group_dn` to `/users/bulk?action=add-to-group`. CSP-safe (external `v2-bulk.js`, `createElement` / `textContent` only).
- **Phase 3 graph view deferral note:** `docs/superpowers/specs/2026-04-20-ui-revamp-phase-3-graph-view-deferred.md` — captures the rough shape and dependencies for the deferred relationship graph view.

### Changed

- Pinned-store hardening: hashed buckets, nil-safe, configurable path.
- Use `ldap.ParseDN` for OU extraction with deterministic entry sort.

### Removed

- Tailwind CSS, PostCSS, TypeScript, Bun, and all associated build tooling (`package.json`, `bun.lock`, `tsconfig.json`, `tailwind.config.js`, `postcss.config.mjs`, concurrently, nodemon, tsc, postcss-\*). The Go binary now builds assets itself via `templ generate` and ships Pico CSS + a hand-written `app.css` + vendored htmx directly.

---

## [v1.2.0] - 2026-04-16

### Fixed

- Upgrade `simple-ldap-go` v1.9.0 → v1.10.0 — fixes password change bug and adds consistent `ValidateSAMAccountName` input validation across all entrypoints

### Changed

- **Migrate from pnpm to Bun** — faster installs, resolves broken `pnpm audit` (npm retired audit API endpoint)
- All CI workflows, Dockerfile, Makefile updated for Bun
- Remove duplicate `pnpm audit` from quality.yml (covered by reusable `node-audit.yml`)

### Added

- `--version` flag — prints version, commit hash, and build timestamp ([#462](https://github.com/netresearch/ldap-manager/pull/462) by @liberodark)
- CONTRIBUTING.md with contribution guidelines
- CHANGELOG.md
- Release labeler workflow for automatic PR/issue tagging

### Dependencies

- `simple-ldap-go` v1.9.0 → v1.10.0
- `testcontainers-go` v0.40.0 → v0.42.0
- `valyala/fasthttp` v1.69.0 → v1.70.0
- `go.opentelemetry.io/otel` v1.41.0 → v1.43.0
- All Node dependencies upgraded to latest (Tailwind 4.2.2, PostCSS 8.5.10, TypeScript 6.0.2)

---

## [v1.1.1] - 2026-01-12

### Fixed

- Restored Docker HEALTHCHECK with built-in `--health-check` flag ([#385](https://github.com/netresearch/ldap-manager/pull/385))
  - Added `--health-check` CLI flag that performs HTTP health check against `/health/live`
  - Works with distroless images (no shell/curl required)

### Changed

- Replaced go-mutesting with gremlins for mutation testing ([#379](https://github.com/netresearch/ldap-manager/pull/379))

### Testing

- Added mutation-killing tests for retry package ([#380](https://github.com/netresearch/ldap-manager/pull/380))
- Added mutation-killing tests for Parse function ([#382](https://github.com/netresearch/ldap-manager/pull/382))

---

## [v1.1.0] - 2025-12-29

### Added

- **Enhanced Detail Views**: Email, description, copy-to-clipboard for users/groups/computers ([#373](https://github.com/netresearch/ldap-manager/pull/373))
- **GUI Rework**: Theme switching (light/dark/system), density controls, accessibility improvements ([#370](https://github.com/netresearch/ldap-manager/pull/370))
- **Client-side Search**: Real-time search filter for users, groups, and computers lists
- **Searchable Combobox**: Filterable dropdown for user/group selection
- **Rate Limiting**: Rate limiting for authentication endpoints
- **Graceful Shutdown**: Proper signal handling with context propagation
- **Retry Logic**: Exponential backoff for LDAP operations
- **TLS Skip Verify**: Support for self-signed certificates
- **WCAG Compliance**: Title attributes and accessibility improvements

### Fixed

- LDAP DN handling for special characters ([#371](https://github.com/netresearch/ldap-manager/pull/371))
- Invalid UTF-8 handling in URL parsing ([#369](https://github.com/netresearch/ldap-manager/pull/369))
- Data race in template cache Get method
- CSS cache busting for old versions

### Changed

- Updated Go to 1.25.x
- Updated simple-ldap-go to v1.6.0
- Updated Tailwind CSS to v4.1.x

---

## [v1.0.8] - 2025-02-14

### Fixed

- CSS build process

---

## [v1.0.7] - 2025-01-31

### Changed

- Dependency updates (pnpm, tailwindcss, prettier)

---

## [v1.0.6] - 2024-11-13

### Changed

- Updated Go to 1.23
- Various dependency updates

---

## Earlier Releases

For releases prior to v1.0.6, see the [GitHub Releases](https://github.com/netresearch/ldap-manager/releases) page.
