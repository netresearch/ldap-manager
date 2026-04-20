# UI Revamp — Phase 1 Slice 8: Cleanup

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Delete the old Tailwind/TypeScript/PostCSS build chain and all legacy templates + handlers that the V2 routes replaced. The app now ships with just Pico CSS + a hand-written `app.css` + vendored htmx + `v2-*.js` files.

**Architecture:** All user-facing routes (`/login`, `/`, `/users`, `/groups`, `/computers`) now render via V2 handlers (Slices 2-6). The V1 code path is dead. This slice removes it surgically.

**Scope:** mostly deletions + small edits. Some tests that asserted on V1 markup need to be removed or retargeted.

**Out of scope:** Phase 2 / Phase 3 features.

---

## Pre-flight

```bash
cd /home/cybot/projects/ldap-manager-ui-revamp-phase-1a
git log --oneline ddea7d9..HEAD | wc -l    # ~46 commits
go test ./... 2>&1 | grep -E "^(FAIL|ok)" | grep -v TestLDAPIntegration
```

---

## Task 1 — Delete legacy templates + handlers

**Files:**
- Delete: `internal/web/templates/login.templ`
- Delete: `internal/web/templates/base.templ`
- Delete: `internal/web/templates/users.templ`
- Delete: `internal/web/templates/groups.templ`
- Delete: `internal/web/templates/computer.templ`
- Delete: `internal/web/templates/index.templ`
- Delete: `internal/web/templates/toggles.templ`
- Delete: `internal/web/templates/list.templ`
- Delete: `internal/web/templates/logged_in.templ`
- Delete: `internal/web/templates/errors.templ` (keep if V2 uses; verify first)
- Delete: `internal/web/templates/icons.templ` (keep if V2 uses; verify first)
- Keep: `internal/web/templates/flash.go` (Flash type still used)
- Delete: legacy handler functions `usersHandler`, `userHandler`, `groupsHandler`, `groupHandler`, `computersHandler`, `computerHandler`, `indexHandler`, and the `*_templ.go` auto-generated files for the deleted `.templ` sources (they're gitignored).
- Delete: `GetStylesPath`, `LoadAssetManifest`, `AssetManifest` type (all in `assets.go`), and anything else that only serves V1.

- [ ] **Step 1: Audit references before deleting**

```bash
for f in login base users groups computer index toggles list logged_in; do
  echo "=== $f.templ ==="
  grep -rn "templates.${f^}\b\|templates.${f^}WithStyles\|templates.User\b\|templates.Users\b\|templates.Computer\b\|templates.Computers\b\|templates.Group\b\|templates.Groups\b\|templates.Index\b" \
    --include="*.go" | grep -v _templ.go
done
grep -rn "GetStylesPath\|LoadAssetManifest\|AssetManifest" --include="*.go" | grep -v _templ.go
```

For each symbol that's referenced, decide:
- Called from a V1 handler that will also be deleted → fine.
- Called from a test → delete the test, or update it to use V2.
- Called from any V2 code → STOP, that's a regression, don't delete the symbol.

- [ ] **Step 2: Delete the template files**

Check `errors.templ` and `icons.templ` first — `errors.templ` likely defines `Flashes()`, `ErrorFlash()` which V2 DOES use. `icons.templ` may define SVG icons used by V2. If they're used by V2, KEEP them. Otherwise delete.

```bash
grep -n "templates\.Flashes\|templates\.ErrorFlash\|templates\.SuccessFlash" --include="*.go" -r internal/web/ | head
grep -n "@homeIcon\|@usersIcon\|@groupIcon\|@laptopIcon\|@logoutIcon\|@rightArrowIcon\|@plusIcon\|@xIcon\|@lockIcon\|@lockOpenIcon" --include="*.templ" -r internal/web/templates/ | head
```

- If `Flashes`/`ErrorFlash` are defined in `errors.templ`, move them into `flash.go` (Go code, not templ), then delete `errors.templ`. If they're in `flash.go` already (implementer should have migrated in an earlier slice), just delete `errors.templ`.
- If icon templ functions are unused by any V2 code (all V2 uses unicode characters like ◐/⇥/↗), delete `icons.templ`.

- [ ] **Step 3: Delete the legacy handler functions**

Edit `internal/web/server.go` to remove:
- `usersHandler`, `userHandler` (in `users.go` or `users_old.go`?)
- `groupsHandler`, `groupHandler`
- `computersHandler`, `computerHandler`
- `indexHandler`
- Any route registrations that still point at these (there should be none after Slices 3-6)

Check also `internal/web/users.go`, `groups.go`, `computers.go`, `health.go` — delete functions no longer referenced. Keep anything V2 still uses (e.g., `handle500`, middleware, auth).

- [ ] **Step 4: Delete the asset manifest helpers**

Delete `internal/web/assets.go` entirely (or narrow to just the embed-related code; confirm). Delete corresponding tests in `assets_test.go`.

- [ ] **Step 5: Delete gone-template auto-generated files**

```bash
# The _templ.go for deleted .templ files are gitignored; just delete them locally to make `go build` clean.
rm -f internal/web/templates/login_templ.go \
      internal/web/templates/base_templ.go \
      internal/web/templates/users_templ.go \
      internal/web/templates/groups_templ.go \
      internal/web/templates/computer_templ.go \
      internal/web/templates/index_templ.go \
      internal/web/templates/toggles_templ.go \
      internal/web/templates/list_templ.go \
      internal/web/templates/logged_in_templ.go
# Regenerate from remaining .templ sources:
templ generate
```

- [ ] **Step 6: Build, fix remaining references, commit**

```bash
go build ./...
```

Expect compile errors from tests that reference deleted symbols. For each:
- If the test is specifically testing V1 behaviour, **delete the whole test function**.
- If it's testing V2 behaviour but happens to reference a V1 symbol, replace with V2 equivalent.

Typical deletions: `login_handler_test.go` tests that assert on old markup. `modify_handlers_test.go` tests asserting on old templates. `template_cache_test.go` if it references old template symbols.

Run unit tests:

```bash
go test ./internal/web/ -count=1 2>&1 | grep -v TestLDAPIntegration | grep -E "^(FAIL|ok|---)"
```

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -S --signoff -m "chore: delete legacy Tailwind-based templates + handlers (V2 fully replaces)"
```

---

## Task 2 — Delete the frontend build chain

**Files to delete:**
- `tailwind.config.js`
- `postcss.config.mjs`
- `tsconfig.json`
- `package.json`
- `package-lock.json`
- `bun.lock`
- `internal/web/tailwind.css`
- `internal/web/static/ts/` (entire directory — TS source)
- `internal/web/static/js/app.js`, `combobox.js`, `copy-clipboard.js`, `density-init.js`, `search-filter.js`, `theme-init.js`, `toggles.js` (TS-compiled outputs — anything NOT starting with `v2-`)
- `internal/web/static/styles.css` (gitignored; Tailwind output — delete local copy)
- `internal/web/static/styles.*.css` (cache-busted variants; gitignored)
- `node_modules/` (gitignored; safe to rm)

**Files to modify:**
- `Makefile` — drop `css:*`, `js:*` targets and any frontend-specific sections
- `Dockerfile` — remove node build stage, bun layer, `bun install`/`bun run build:assets` steps
- `AGENTS.md`, `CLAUDE.md` files — update instructions

- [ ] **Step 1: Delete files**

```bash
git rm -rf tailwind.config.js postcss.config.mjs tsconfig.json package.json \
           package-lock.json bun.lock internal/web/tailwind.css \
           internal/web/static/ts/

# Local-only deletes (gitignored files):
rm -rf node_modules internal/web/static/js/app.js internal/web/static/js/combobox.js \
       internal/web/static/js/copy-clipboard.js internal/web/static/js/density-init.js \
       internal/web/static/js/search-filter.js internal/web/static/js/theme-init.js \
       internal/web/static/js/toggles.js
rm -f internal/web/static/styles.css internal/web/static/styles.*.css
```

For each file, check it's not still referenced by an `app.css`/templ. Verify with grep.

Which `js/*.js` files stay: only those starting with `v2-` (the CSP-safe ones from Slices 2-3).

- [ ] **Step 2: Update `Makefile`**

Open `Makefile`. Find frontend-related targets:
- `css:build`, `css:dev`, `css:analyze` — delete
- `js:build`, `js:dev` — delete
- `templ:build`, `templ:dev` — keep (still needed)
- `build:assets` target — rewrite to just invoke `templ generate && scripts/vendor.sh`
- `dev` target — remove `concurrently` invocation, keep `go run` + `templ generate -watch` if desired
- Any reference to `bun`, `bunx`, `npm`, `tsc`, `postcss` — delete
- `make setup` / `make setup-frontend` — delete frontend portion

Keep the `format-*`, `lint-*`, `test-*`, `check` targets pointing at Go only.

- [ ] **Step 3: Update `Dockerfile`**

Currently likely a multi-stage build: `node/bun` image → build assets → `golang` image → build Go binary → distroless runtime.

Drop the frontend stage. The Go build now produces everything needed (vendor files are committed; templ generates at build time).

Expected new shape:

```Dockerfile
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go install github.com/a-h/templ/cmd/templ@latest && templ generate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ldap-manager ./cmd/ldap-manager

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/ldap-manager /app/ldap-manager
EXPOSE 3000
USER nonroot
ENTRYPOINT ["/app/ldap-manager"]
```

Match the EXISTING Dockerfile's labels, healthcheck, arg handling as closely as possible.

- [ ] **Step 4: Update AGENTS.md / CLAUDE.md**

Replace references to:
- "Tailwind CSS v4", "PostCSS", "TypeScript" → "Pico CSS + hand-written app.css" / "plain JS" / "templ"
- `bun run build:assets` / `bun run dev` → `scripts/vendor.sh` / `make dev`
- `tailwind.config.js`, `postcss.config.mjs` → gone

Files to update:
- Top-level `AGENTS.md`
- `internal/web/AGENTS.md` and `CLAUDE.md`
- `internal/AGENTS.md` and `CLAUDE.md`
- `scripts/AGENTS.md` if it mentions JS build
- `docs/AGENTS.md` / `CLAUDE.md`

Keep the documentation short: re-state the stack accurately, remove TODOs/notes about the old build.

- [ ] **Step 5: Build, lint, test, commit**

```bash
go build ./...
make lint-go
go test ./... -count=1 2>&1 | grep -v TestLDAPIntegration | grep -E "^(FAIL|ok)"
```

Verify the binary starts and serves `/login`:

```bash
go build -o /tmp/ldap-manager-clean ./cmd/ldap-manager
setsid env PORT=3110 /tmp/ldap-manager-clean \
  --ldap-server ldap://127.0.0.1:1389 \
  --base-dn dc=preview,dc=local \
  --readonly-user cn=admin,dc=preview,dc=local \
  --readonly-password admin \
  --active-directory=false \
  --cookie-secure=false \
  --log-level info > /tmp/ldap-preview-clean.log 2>&1 < /dev/null &
sleep 4
curl -s -o /dev/null -w 'HTTP %{http_code}\n' http://localhost:3110/login
# Expect HTTP 200.
pkill -f ldap-manager-clean
```

Commit:

```bash
git add -A
git commit -S --signoff -m "chore: drop Tailwind/TypeScript/PostCSS; Go-only build"
```

---

## Task 3 — Update README + CHANGELOG

**Files:**
- Modify: `README.md` — update stack description, build instructions
- Modify: `CHANGELOG.md` — add an entry summarizing the UI revamp

- [ ] **Step 1: README**

Update any section referencing Tailwind/bun/TypeScript. The "Docker Image" section likely remains mostly the same.

Add or adjust a "Stack" bullet that matches reality: "Go + Fiber + Templ + Pico CSS + htmx + plain JS."

Remove the `bun install` / `bun run dev` instructions; replace with `make dev` / `go run ./cmd/ldap-manager`.

- [ ] **Step 2: CHANGELOG**

Append under an `## [Unreleased]` section:

```
### Changed
- **UI revamp** (Phase 1): Command-first interface with ⌘K palette, pin/unpin, recents, detail drawer. New hybrid light/dark theme (Inter sans in light, monospace in dark). WCAG 2.2 AAA conformance on all new surfaces, verified in CI via axe-core.

### Removed
- Tailwind CSS, PostCSS, TypeScript, and all associated build tooling (bun, concurrently, nodemon, tsc, postcss-*). The Go binary now builds assets itself via `templ generate` and ships Pico CSS + custom `app.css` directly.
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -S --signoff -m "docs: update README + CHANGELOG for Phase 1 UI revamp"
```

---

## Task 4 — Final Phase 1 verification

- [ ] **Step 1: Full test suite**

```bash
make lint-go
go test ./... -count=1 2>&1 | grep -E "^(FAIL|ok)"
go test -tags e2e ./internal/e2e/ -count=1 2>&1 | grep -E "^(FAIL|ok|---)"
```

All green except `TestLDAPIntegration_*` (environmental).

- [ ] **Step 2: Visual smoke**

Start preview. Click through every route:
- `/login` — form works.
- `/` — greeting + pinned + recent empty states.
- `/users` — list + drawer; row click swaps drawer.
- `/groups` — same.
- `/computers` — same.
- ⌘K from each page — palette opens, fuzzy search, navigates.
- Theme toggle — hybrid sans ↔ mono swap.
- Density toggle — target sizes change.
- Pin a user via drawer → logout → login → pinned card shows it on home.

---

## Self-review

- Legacy templates and handlers live exclusively behind old routes that no longer exist post-Slices 2-6; removing them has no user-facing effect.
- Generated `*_templ.go` files are gitignored — no diff pollution from deletion.
- Dockerfile size drops significantly (no node stage).
- `package.json` disappears; there's no longer a "frontend build" concept. `scripts/vendor.sh` is the only tool that refreshes vendor files, and it's shell-only.
- Test deletions should be scoped: delete only tests that assert on V1 markup or call V1 symbols. Don't delete tests covering V2 behaviour even if they're in `login_handler_test.go` etc.
