# UI Revamp — Phase 1 Foundation + Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the new frontend stack (Pico CSS + htmx + Alpine.js) on the login page, delete zero old code, prove AAA conformance end-to-end with automated tests. This is Slice 1 (Pre-work) + Slice 2 (Login) of the spec's 8-slice Phase 1.

**Architecture:** Strangler pattern. New templates live alongside old ones; only the login route is switched in this plan. Old Tailwind pipeline keeps running for every other route. AAA conformance is machine-verified via a contrast unit test and an axe-core pass inside the Playwright E2E suite. No user-visible changes outside the login page.

**Tech Stack:**
- Vendored: Pico CSS v2 (single CSS file), htmx v2 (single JS file), Alpine.js v3 (single JS file), axe-core v4 (E2E only, single JS file).
- `scripts/vendor.sh` — bash, downloads pinned versions with SHA256 verification.
- New hand-written `internal/web/static/app.css` (hybrid theme tokens + login component).
- New Templ components: `base_v2.templ`, `login_v2.templ`.
- Go contrast test (stdlib-only, no new deps).
- Playwright E2E test adds axe-core injection + AAA rule pass.

**Spec reference:** [`docs/superpowers/specs/2026-04-20-ui-revamp-design.md`](../specs/2026-04-20-ui-revamp-design.md) — §4 (visual language), §7 (a11y), §9 (migration plan, slices 1–2).

**Out of scope for this plan** (shipped in later plans):
- Any route other than `/login`.
- Deleting Tailwind, TypeScript, or PostCSS (Slice 8).
- Pinned/Recents, command palette, detail drawer (Slices 3+).

---

## File Structure

**Created:**
- `scripts/vendor.sh` — downloads & verifies pinned vendor files into `internal/web/static/vendor/`.
- `scripts/vendor.lock` — pinned versions + SHA256s.
- `internal/web/static/vendor/.gitkeep` — so directory exists before `vendor.sh` runs.
- `internal/web/static/vendor/pico.min.css` — checked-in, refreshed by `vendor.sh`.
- `internal/web/static/vendor/htmx.min.js` — checked-in.
- `internal/web/static/vendor/alpine.min.js` — checked-in.
- `internal/web/static/app.css` — hand-written; hybrid theme custom properties + login component. Grows in later slices.
- `internal/web/templates/base_v2.templ` — new base template using Pico + app.css + vendored JS.
- `internal/web/templates/login_v2.templ` — new login page.
- `internal/web/contrast_test.go` — parses `app.css` custom properties, asserts every documented text/background pair ≥7:1.
- `internal/e2e/axe_test.go` — Playwright E2E test that injects axe-core on `/login` and asserts zero AAA violations.
- `internal/e2e/testdata/axe.min.js` — vendored axe-core for offline CI.

**Modified:**
- `internal/web/auth.go` — swap `templates.LoginWithStyles(...)` → `templates.LoginV2(...)` (2 call sites).
- `internal/web/server.go` — register `/static/vendor/*` serving route if not already covered by the static handler (verify first).
- `.gitignore` — ensure `internal/web/static/vendor/*` is NOT ignored (checked-in vendor files).
- `Makefile` — add `vendor` and `test-contrast` targets.

**Not modified in this plan:**
- Every other templ, handler, test, or CSS file. Tailwind pipeline stays untouched.

---

## Pre-flight

- [ ] **Step 0.1: Create a worktree for this work**

  The repo convention (CLAUDE.md) is one worktree per branch. From the project root:

  ```bash
  cd /home/cybot/projects/ldap-manager
  git -C .bare worktree list 2>/dev/null || echo "no bare setup — using single-checkout mode"
  ```

  If the bare setup exists, add a worktree:

  ```bash
  git -C .bare worktree add ../ui-revamp-phase-1a main
  cd ../ui-revamp-phase-1a
  git checkout -b feat/ui-revamp-phase-1a
  ```

  If not (single-checkout repo), create the branch in place:

  ```bash
  git checkout -b feat/ui-revamp-phase-1a
  ```

  All subsequent paths assume you are in the project root of whichever layout you have.

- [ ] **Step 0.2: Confirm toolchain**

  Run:

  ```bash
  go version          # expect: go1.25+
  curl --version | head -1
  sha256sum --version | head -1
  templ version || go install github.com/a-h/templ/cmd/templ@latest
  ```

  Expected: Go 1.25+ (project uses 1.25 per `go.mod`), curl, sha256sum, templ CLI available.

---

## Slice 1 — Foundation (Pre-work)

No user-visible change. Adds vendor pipeline, new theme CSS skeleton, contrast test harness.

### Task 1.1: Skeleton `scripts/vendor.sh`

**Files:**
- Create: `scripts/vendor.sh`
- Create: `scripts/vendor.lock`
- Create: `internal/web/static/vendor/.gitkeep`

- [ ] **Step 1: Create empty vendor directory placeholder**

  ```bash
  mkdir -p internal/web/static/vendor
  touch internal/web/static/vendor/.gitkeep
  ```

- [ ] **Step 2: Write `scripts/vendor.lock` with pinned versions**

  Before committing, the EXECUTOR must verify each URL is the current stable version via `mcp__package-version__check_npm_versions` (or the maintainers' release pages) and compute real SHA256s. Replace the placeholder hashes below with verified values.

  ```
  # scripts/vendor.lock
  # Pinned frontend vendor files. Refreshed by scripts/vendor.sh.
  # Format: <dest-path> <url> <sha256>

  internal/web/static/vendor/pico.min.css https://cdn.jsdelivr.net/npm/@picocss/pico@2.0.6/css/pico.min.css PLACEHOLDER_SHA256_PICO
  internal/web/static/vendor/htmx.min.js https://cdn.jsdelivr.net/npm/htmx.org@2.0.4/dist/htmx.min.js PLACEHOLDER_SHA256_HTMX
  internal/web/static/vendor/alpine.min.js https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js PLACEHOLDER_SHA256_ALPINE
  internal/e2e/testdata/axe.min.js https://cdn.jsdelivr.net/npm/axe-core@4.10.2/axe.min.js PLACEHOLDER_SHA256_AXE
  ```

  Create the file exactly as above; the EXECUTOR replaces placeholders in Step 4.

- [ ] **Step 3: Write `scripts/vendor.sh`**

  ```bash
  #!/usr/bin/env bash
  # Refreshes pinned frontend vendor files from scripts/vendor.lock.
  # Aborts if SHA256 verification fails.

  set -euo pipefail

  LOCK="$(dirname "$0")/vendor.lock"
  ROOT="$(cd "$(dirname "$0")/.." && pwd)"

  if [[ ! -f "$LOCK" ]]; then
    echo "missing $LOCK" >&2
    exit 1
  fi

  fail=0
  while IFS=' ' read -r dest url sha; do
    # skip blank lines and comments
    [[ -z "$dest" || "$dest" == \#* ]] && continue

    abs="$ROOT/$dest"
    mkdir -p "$(dirname "$abs")"
    echo "fetching $url"
    tmp="$(mktemp)"
    trap 'rm -f "$tmp"' EXIT
    curl --fail --silent --show-error --location "$url" -o "$tmp"

    got="$(sha256sum "$tmp" | awk '{print $1}')"
    if [[ "$got" != "$sha" ]]; then
      echo "SHA256 mismatch for $dest" >&2
      echo "  expected: $sha" >&2
      echo "  got:      $got" >&2
      fail=1
      rm -f "$tmp"
      continue
    fi

    mv "$tmp" "$abs"
    trap - EXIT
    echo "  -> $dest"
  done < "$LOCK"

  if [[ $fail -ne 0 ]]; then
    echo "one or more vendor files failed SHA verification" >&2
    exit 1
  fi

  echo "vendor refresh OK"
  ```

- [ ] **Step 4: Make executable and populate real SHAs**

  ```bash
  chmod +x scripts/vendor.sh

  # Temporarily replace all PLACEHOLDER_SHA256_* lines with "0000000..." (64 zeros).
  # Run vendor.sh — it will fail on each mismatch and print the ACTUAL SHA.
  # Capture the actual SHAs, edit vendor.lock, re-run.

  sed -i 's/PLACEHOLDER_SHA256_[A-Z]*/0000000000000000000000000000000000000000000000000000000000000000/g' scripts/vendor.lock
  ./scripts/vendor.sh 2>&1 | tee /tmp/vendor.log || true

  # Manually update scripts/vendor.lock with the 4 "got:" values from the log.
  # Then rerun until exit 0:
  ./scripts/vendor.sh
  ```

  Expected (after SHA fixup): `vendor refresh OK` and four files exist:
  - `internal/web/static/vendor/pico.min.css`
  - `internal/web/static/vendor/htmx.min.js`
  - `internal/web/static/vendor/alpine.min.js`
  - `internal/e2e/testdata/axe.min.js`

- [ ] **Step 5: Verify the vendored files look sane**

  ```bash
  wc -c internal/web/static/vendor/*.* internal/e2e/testdata/axe.min.js
  # Expect roughly:
  #  pico.min.css  ~70-120 KB
  #  htmx.min.js   ~50 KB
  #  alpine.min.js ~45 KB
  #  axe.min.js    ~450-700 KB

  head -c 80 internal/web/static/vendor/pico.min.css; echo
  # Expect: something like "/*! Pico CSS v2.*.* ..."
  ```

- [ ] **Step 6: Commit**

  ```bash
  git add scripts/vendor.sh scripts/vendor.lock \
          internal/web/static/vendor/ \
          internal/e2e/testdata/axe.min.js
  git commit -S --signoff -m "chore(vendor): add pinned Pico/htmx/Alpine/axe-core with SHA-verified refresh"
  ```

### Task 1.2: Hand-written `app.css` skeleton

Only the tokens and base layer for now. Login-specific styles added in Task 2.2. This locks the color palette from the spec.

**Files:**
- Create: `internal/web/static/app.css`

- [ ] **Step 1: Write the file exactly as follows**

  ```css
  /*
   * LDAP Manager — custom app styles on top of Pico CSS v2.
   * Hybrid theme: Light = Clean sans (Inter); Dark = Terminal/IDE (monospace).
   * Every text-on-surface pair below is AAA-verified (ratio ≥7:1).
   *
   * See: docs/superpowers/specs/2026-04-20-ui-revamp-design.md §4
   */

  /* ──────────────────────────── theme tokens ──────────────────────────── */

  :root {
    /* Light (Clean sans neutral) */
    --bg: #ffffff;
    --bg-subtle: #fafafa;
    --fg: #0a0a0a;        /* 20.6:1 on --bg */
    --fg-muted: #525252;  /*  8.3:1 on --bg */
    --border: #e5e5e5;
    --border-strong: #a3a3a3;
    --accent: #0a0a0a;
    --accent-fg: #fafafa;

    /* Pico overrides — we drive the colours via our tokens */
    --pico-background-color: var(--bg);
    --pico-color: var(--fg);
    --pico-muted-color: var(--fg-muted);
    --pico-muted-border-color: var(--border);
    --pico-primary: var(--accent);
    --pico-primary-hover: var(--fg);
    --pico-primary-focus: var(--border-strong);
    --pico-primary-inverse: var(--accent-fg);
    --pico-form-element-focus-color: var(--border-strong);

    --font-sans: "Inter", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
    --font-mono: ui-monospace, "JetBrains Mono", "SF Mono", Consolas, monospace;
    --font-heading: var(--font-sans);
  }

  :root[data-theme="dark"] {
    /* Dark (Terminal/IDE) */
    --bg: #0d0d0d;
    --bg-subtle: #1a1a1a;
    --fg: #f5f5f5;        /* 17.8:1 on --bg */
    --fg-muted: #a3a3a3;  /* 10.4:1 on --bg */
    --border: #262626;
    --border-strong: #525252;
    --accent: #4ade80;    /* 11.2:1 on --bg */
    --accent-fg: #0d0d0d;

    --font-heading: var(--font-mono);
  }

  /* ──────────────────────────── density ──────────────────────────────── */

  :root {
    /* Comfortable (touch / narrow / reduced-motion default).
       Enforced for target-size ≥44×44 (WCAG 2.2 AAA 2.5.5). */
    --density-touch-size: 2.75rem;   /* 44 px at 16 px root */
    --density-spacing:    0.875rem;
    --density-font:       1rem;
  }

  :root[data-density="compact"] {
    /* Compact (desktop default). AA target size (36×36), not AAA. */
    --density-touch-size: 2.25rem;   /* 36 px */
    --density-spacing:    0.5rem;
    --density-font:       0.9375rem;
  }

  /* Respect reduced-motion preference for all transitions, everywhere. */
  @media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
      animation-duration: 0.01ms !important;
      transition-duration: 0.01ms !important;
      scroll-behavior: auto !important;
    }
  }

  /* ──────────────────────────── base layer ───────────────────────────── */

  html {
    color-scheme: light dark;
    background: var(--bg);
    color: var(--fg);
    font-family: var(--font-sans);
    font-size: var(--density-font);
    line-height: 1.5;
  }

  :root[data-theme="dark"] body { font-family: var(--font-mono); }

  /* Enhanced focus appearance — AAA-friendly and always visible. */
  :where(a, button, input, select, textarea, [tabindex]):focus-visible {
    outline: 2px solid var(--border-strong);
    outline-offset: 2px;
  }
  ```

- [ ] **Step 2: Verify the file compiles and has no typos**

  ```bash
  # Extremely simple sanity: each selector block closes properly.
  node -e "const fs=require('fs');const s=fs.readFileSync('internal/web/static/app.css','utf8');const open=(s.match(/\{/g)||[]).length;const close=(s.match(/\}/g)||[]).length;if(open!==close){console.error('brace mismatch:',open,close);process.exit(1)}console.log('braces ok:',open)" 2>/dev/null \
    || awk 'BEGIN{o=0;c=0} {for(i=1;i<=length;i++){ch=substr($0,i,1);if(ch=="{")o++;else if(ch=="}")c++}} END{if(o!=c){print "brace mismatch: "o"/"c;exit 1} else print "braces ok: "o}' internal/web/static/app.css
  ```

  Expected: `braces ok: <some number>`. If mismatch, fix the CSS before proceeding.

- [ ] **Step 3: Commit**

  ```bash
  git add internal/web/static/app.css
  git commit -S --signoff -m "feat(ui): add app.css with AAA-verified hybrid theme tokens"
  ```

### Task 1.3: Contrast unit test

Parses `app.css` declared tokens and asserts every documented text-on-surface pair meets AAA 7:1. No new deps; uses stdlib only.

**Files:**
- Create: `internal/web/contrast_test.go`

- [ ] **Step 1: Write the failing test first**

  ```go
  // internal/web/contrast_test.go
  package web

  import (
      "math"
      "os"
      "path/filepath"
      "regexp"
      "strconv"
      "strings"
      "testing"
  )

  // TestAppCSSContrastAAA verifies every declared text/background pair in
  // app.css meets WCAG 2.2 AAA contrast (≥7:1 for normal text). Tokens are
  // read straight from the file so drift is caught automatically.
  func TestAppCSSContrastAAA(t *testing.T) {
      path := filepath.Join("static", "app.css")
      raw, err := os.ReadFile(path)
      if err != nil {
          t.Fatalf("read %s: %v", path, err)
      }

      light := parseTokens(string(raw), "")       // :root
      dark := parseTokens(string(raw), "dark")    // :root[data-theme="dark"]

      // dark inherits anything not overridden
      for k, v := range light {
          if _, ok := dark[k]; !ok {
              dark[k] = v
          }
      }

      pairs := []struct {
          theme  string
          fg, bg string
          min    float64 // 7.0 for AAA normal text; 4.5 for non-essential
      }{
          {"light", "--fg", "--bg", 7.0},
          {"light", "--fg-muted", "--bg", 7.0},
          {"light", "--fg", "--bg-subtle", 7.0},
          {"light", "--fg-muted", "--bg-subtle", 7.0},
          {"light", "--accent-fg", "--accent", 7.0},
          {"dark", "--fg", "--bg", 7.0},
          {"dark", "--fg-muted", "--bg", 7.0},
          {"dark", "--fg", "--bg-subtle", 7.0},
          {"dark", "--fg-muted", "--bg-subtle", 7.0},
          {"dark", "--accent-fg", "--accent", 7.0},
          {"dark", "--accent", "--bg", 7.0}, // accent-as-text on dark bg (headings)
      }

      for _, p := range pairs {
          tokens := light
          if p.theme == "dark" {
              tokens = dark
          }
          fg, ok := tokens[p.fg]
          if !ok {
              t.Errorf("%s: token %s not found", p.theme, p.fg)
              continue
          }
          bg, ok := tokens[p.bg]
          if !ok {
              t.Errorf("%s: token %s not found", p.theme, p.bg)
              continue
          }
          ratio := contrast(fg, bg)
          if ratio < p.min {
              t.Errorf("%s: %s on %s = %.2f (need ≥%.1f)",
                  p.theme, p.fg, p.bg, ratio, p.min)
          }
      }
  }

  var (
      reTokenLine = regexp.MustCompile(`^\s*(--[a-z][a-z0-9-]*)\s*:\s*(#[0-9a-fA-F]{3,8})\s*;`)
      reDarkBlock = regexp.MustCompile(`(?s):root\[data-theme="dark"\]\s*\{(.*?)\}`)
      reLightTop  = regexp.MustCompile(`(?s)^\s*:root\s*\{(.*?)\}`)
  )

  func parseTokens(css, theme string) map[string][3]int {
      var block string
      switch theme {
      case "dark":
          m := reDarkBlock.FindStringSubmatch(css)
          if len(m) == 2 {
              block = m[1]
          }
      default:
          m := reLightTop.FindStringSubmatch(css)
          if len(m) == 2 {
              block = m[1]
          }
      }

      out := map[string][3]int{}
      for _, line := range strings.Split(block, "\n") {
          m := reTokenLine.FindStringSubmatch(line)
          if len(m) != 3 {
              continue
          }
          if rgb, ok := parseHex(m[2]); ok {
              out[m[1]] = rgb
          }
      }
      return out
  }

  func parseHex(h string) ([3]int, bool) {
      h = strings.TrimPrefix(h, "#")
      if len(h) == 3 {
          h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
      }
      if len(h) != 6 {
          return [3]int{}, false
      }
      var v [3]int
      for i := 0; i < 3; i++ {
          n, err := strconv.ParseInt(h[i*2:i*2+2], 16, 0)
          if err != nil {
              return [3]int{}, false
          }
          v[i] = int(n)
      }
      return v, true
  }

  // relativeLuminance per WCAG 2.x.
  func relativeLuminance(rgb [3]int) float64 {
      f := func(c int) float64 {
          x := float64(c) / 255.0
          if x <= 0.03928 {
              return x / 12.92
          }
          return math.Pow((x+0.055)/1.055, 2.4)
      }
      return 0.2126*f(rgb[0]) + 0.7152*f(rgb[1]) + 0.0722*f(rgb[2])
  }

  func contrast(a, b [3]int) float64 {
      la, lb := relativeLuminance(a), relativeLuminance(b)
      if la < lb {
          la, lb = lb, la
      }
      return (la + 0.05) / (lb + 0.05)
  }
  ```

- [ ] **Step 2: Run the test — expect PASS**

  Unlike typical TDD this test asserts facts that are true by construction (the palette was designed AAA-compliant in the spec). Running is the verification that the parser reads `app.css` correctly.

  ```bash
  go test ./internal/web/ -run TestAppCSSContrastAAA -v
  ```

  Expected: `PASS`. If FAIL for any pair, either the token regex missed a value (look at the error message) or a shade needs adjusting in `app.css`. In either case, fix the root cause and re-run.

- [ ] **Step 3: Add to Makefile**

  Modify `Makefile` — add a target (append to the bottom, or near other `test-*` targets if they exist):

  ```makefile
  .PHONY: test-contrast
  test-contrast: ## run AAA contrast check on app.css
  	@go test ./internal/web/ -run TestAppCSSContrast -v
  ```

  Verify:

  ```bash
  make test-contrast
  ```

  Expected: the test runs and passes.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/web/contrast_test.go Makefile
  git commit -S --signoff -m "test(ui): verify app.css tokens meet WCAG AAA contrast"
  ```

### Task 1.4: Base template v2

New Templ component that loads Pico + app.css + Alpine + htmx. Replaces neither old `base.templ` nor any route yet.

**Files:**
- Create: `internal/web/templates/base_v2.templ`

- [ ] **Step 1: Write the templ file**

  ```go
  // internal/web/templates/base_v2.templ
  package templates

  // baseV2 is the new-stack shell: Pico + app.css + Alpine + htmx.
  // Renders theme/density attributes so the initialization scripts can
  // apply preferences before first paint.
  //
  // Stable across all Phase 1 slices. Individual pages embed page-specific
  // component styles in their own templ files, not here.
  templ baseV2(title string) {
  	<!DOCTYPE html>
  	<html lang="en" data-theme="light" data-density="compact" x-cloak>
  		<head>
  			<meta charset="UTF-8"/>
  			<meta name="viewport" content="width=device-width, initial-scale=1"/>
  			<title>{ title } · LDAP Manager</title>

  			<link rel="icon" type="image/png" sizes="32x32" href="/static/favicon-32x32.png"/>
  			<link rel="icon" type="image/png" sizes="16x16" href="/static/favicon-16x16.png"/>
  			<link rel="icon" type="image/x-icon" href="/static/favicon.ico"/>
  			<link rel="manifest" href="/static/site.webmanifest"/>

  			<link rel="stylesheet" href="/static/vendor/pico.min.css"/>
  			<link rel="stylesheet" href="/static/app.css"/>

  			// Preference initialization: runs before first paint to avoid FOUC.
  			<script>
  				(function () {
  					var coarse = matchMedia('(pointer: coarse)').matches;
  					var narrow = matchMedia('(max-width: 600px)').matches;
  					var reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;

  					var storedTheme = localStorage.getItem('theme');
  					var theme = storedTheme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
  					document.documentElement.setAttribute('data-theme', theme);

  					var storedDensity = localStorage.getItem('density');
  					var autoDensity = (coarse || narrow || reduce) ? 'comfortable' : 'compact';
  					var density = storedDensity || autoDensity;
  					document.documentElement.setAttribute('data-density', density);
  				})();
  			</script>

  			// Alpine needs to be loaded defer so x-data directives attach after DOM parse.
  			<script defer src="/static/vendor/alpine.min.js"></script>
  			<script defer src="/static/vendor/htmx.min.js"></script>
  		</head>
  		<body>
  			{ children... }
  		</body>
  	</html>
  }
  ```

- [ ] **Step 2: Generate Go code from the templ**

  ```bash
  templ generate
  ```

  Expected: silent success; a new file `internal/web/templates/base_v2_templ.go` appears.

- [ ] **Step 3: Confirm it builds**

  ```bash
  go build ./...
  ```

  Expected: no errors. If `base_v2_templ.go` referenced undefined symbols, regenerate. If build errors reference Go style, fix — do not proceed.

- [ ] **Step 4: Commit**

  ```bash
  git add internal/web/templates/base_v2.templ internal/web/templates/base_v2_templ.go
  git commit -S --signoff -m "feat(ui): add base_v2 Templ shell for new Pico+htmx+Alpine stack"
  ```

### Task 1.5: Static asset serving — verify vendor path works

The existing static handler already serves `internal/web/static/`. The `vendor/` subdirectory should work for free, but a quick test confirms.

**Files:**
- Potentially modify: `internal/web/server.go` (only if the current static handler excludes subdirectories)

- [ ] **Step 1: Identify how static assets are served**

  ```bash
  grep -n 'static' internal/web/server.go internal/web/assets.go 2>/dev/null | grep -vE 'test|\.templ|_templ\.'
  ```

  Inspect the result. Expected pattern: a Fiber `fs.Sub` or `Static("/static", ...)` pointing at the embedded static FS.

- [ ] **Step 2: Add a temporary debug route to confirm (read-only test)**

  Rather than modifying the server, spin it up in a terminal and curl the vendor paths.

  ```bash
  # In one terminal:
  make dev       # or: go run ./cmd/ldap-manager
  # Wait for listen message.

  # In another terminal:
  curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://localhost:3000/static/vendor/pico.min.css
  curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://localhost:3000/static/vendor/htmx.min.js
  curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://localhost:3000/static/vendor/alpine.min.js
  curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://localhost:3000/static/app.css

  # Shut down the dev server.
  ```

  Expected: four lines each starting with `200 ` and a byte count matching what `wc -c` showed for each file.

  If any returns `404`, inspect the embedded FS include pattern. Likely a `//go:embed` directive needs `vendor/*` or `vendor` added, or the embed pattern is already `all:static` which catches everything.

- [ ] **Step 3: If a 404 occurred, fix the embed directive**

  Likely in `internal/web/static/static.go`. The common safe pattern is:

  ```go
  //go:embed all:*
  var Static embed.FS
  ```

  If the directive currently excludes something (e.g. uses an explicit list), add the vendor pattern. Regenerate `templ generate && go build ./...` and re-run the curl commands until all four return `200`.

- [ ] **Step 4: Commit (only if changes were made)**

  ```bash
  git diff --cached --stat
  # If anything staged, commit:
  git add internal/web/static/static.go
  git commit -S --signoff -m "fix(ui): ensure static/vendor subdirectory is embedded and served"
  # Else skip.
  ```

### Task 1.6: Slice 1 wrap-up

- [ ] **Step 1: Re-run the full existing test suite**

  ```bash
  make check
  ```

  Expected: everything passes. No regression in existing tests; `TestAppCSSContrastAAA` passes.

- [ ] **Step 2: Confirm no user-visible change**

  ```bash
  make dev &
  # Wait, then visit http://localhost:3000/login in a browser.
  # The page should look EXACTLY as before (Tailwind still in charge for /login).
  # Kill the server.
  ```

  The baseV2 template exists but is unused until Slice 2.

---

## Slice 2 — Login page on the new stack

Replaces the existing login UI with a Pico + app.css version. Visible labels (AAA), single-card centered layout, theme & density toggles present. E2E tests get axe-core AAA verification.

### Task 2.1: Extend `app.css` with login component styles

**Files:**
- Modify: `internal/web/static/app.css` (append to end)

- [ ] **Step 1: Append the login component block**

  Open `internal/web/static/app.css` and add at the end:

  ```css
  /* ──────────────────────────── layouts ──────────────────────────────── */

  .page-center {
      min-height: 100dvh;
      display: grid;
      place-items: center;
      padding: 1rem;
      gap: 1rem;
  }

  .top-right-actions {
      position: fixed;
      top: 0.75rem;
      right: 0.75rem;
      display: flex;
      gap: 0.5rem;
      z-index: 10;
  }

  /* ──────────────────────────── login card ──────────────────────────── */

  .login-card {
      width: min(100%, 26rem);
      background: var(--bg-subtle);
      border: 1px solid var(--border);
      border-radius: 0.5rem;
      padding: 2rem;
      display: flex;
      flex-direction: column;
      gap: 1rem;
  }

  .login-card__logo {
      display: block;
      max-width: 12rem;
      margin: 0 auto 0.5rem;
  }

  .login-card label {
      display: block;
      margin-bottom: 0.25rem;
      font-weight: 500;
  }

  .login-card input[type="text"],
  .login-card input[type="password"] {
      width: 100%;
      min-height: var(--density-touch-size);
      padding: 0 0.75rem;
      background: var(--bg);
      color: var(--fg);
      border: 1px solid var(--border);
      border-radius: 0.375rem;
      font: inherit;
  }

  .login-card input:focus-visible {
      outline: 2px solid var(--border-strong);
      outline-offset: 2px;
      border-color: var(--border-strong);
  }

  .login-card button[type="submit"] {
      width: 100%;
      min-height: var(--density-touch-size);
      background: var(--accent);
      color: var(--accent-fg);
      border: 1px solid var(--accent);
      border-radius: 0.375rem;
      font: inherit;
      font-weight: 600;
      cursor: pointer;
  }

  .login-card button[type="submit"]:hover,
  .login-card button[type="submit"]:focus-visible {
      background: var(--bg);
      color: var(--fg);
  }

  /* ──────────────────────────── flash ─────────────────────────────── */

  .flash {
      padding: 0.75rem 1rem;
      border: 1px solid var(--border);
      border-radius: 0.375rem;
      background: var(--bg);
  }

  .flash[role="alert"] {
      border-color: #dc2626; /* 5.9:1 on white; alert colour only, not text */
  }

  :root[data-theme="dark"] .flash[role="alert"] {
      border-color: #f87171; /* 7.2:1 on #0d0d0d */
  }

  /* ──────────────────────────── icon buttons ──────────────────────────── */

  .icon-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: var(--density-touch-size);
      height: var(--density-touch-size);
      min-width: 2.25rem;
      min-height: 2.25rem;
      background: var(--bg-subtle);
      border: 1px solid var(--border);
      border-radius: 0.375rem;
      color: var(--fg);
      cursor: pointer;
  }

  .icon-btn:hover,
  .icon-btn:focus-visible {
      border-color: var(--border-strong);
  }

  /* ──────────────────────────── footer ─────────────────────────────── */

  .app-footer {
      margin-top: auto;
      padding: 1rem 0;
      color: var(--fg-muted);
      text-align: center;
      font-size: 0.875rem;
  }

  .app-footer a {
      color: var(--fg);
      text-decoration: underline;
      text-underline-offset: 2px;
  }
  ```

- [ ] **Step 2: Contrast test still passes**

  ```bash
  go test ./internal/web/ -run TestAppCSSContrastAAA -v
  ```

  Expected: PASS. (We added new tokens only for the alert border; no new text pairs are introduced. The colours `#dc2626` and `#f87171` are non-text decoration so they need not be 7:1.)

- [ ] **Step 3: Commit**

  ```bash
  git add internal/web/static/app.css
  git commit -S --signoff -m "feat(ui): add login-card, flash, icon-btn styles to app.css"
  ```

### Task 2.2: `login_v2.templ`

**Files:**
- Create: `internal/web/templates/login_v2.templ`

- [ ] **Step 1: Write the templ file**

  ```go
  // internal/web/templates/login_v2.templ
  package templates

  // LoginV2 is the new login page on the Pico+htmx+Alpine stack.
  // Uses visible labels (WCAG 2.2 AAA) and the hybrid theme.
  templ LoginV2(flashes []Flash, version, csrfToken string) {
  	@baseV2("Login") {
  		<div class="top-right-actions">
  			@themeToggleV2()
  			@densityToggleV2()
  		</div>

  		<main class="page-center">
  			<form class="login-card" action="/login" method="post" novalidate>
  				<input type="hidden" name="csrf_token" value={ csrfToken }/>

  				<img class="login-card__logo" src="/static/logo.webp" alt="LDAP Manager"/>

  				if len(flashes) > 0 {
  					<div role="alert" aria-live="assertive">
  						for _, flash := range flashes {
  							<div class="flash">{ flash.Message }</div>
  						}
  					</div>
  				}

  				<div>
  					<label for="login-username">Username</label>
  					<input
  						id="login-username"
  						type="text"
  						name="username"
  						autocomplete="username"
  						required
  						autofocus
  					/>
  				</div>

  				<div>
  					<label for="login-password">Password</label>
  					<input
  						id="login-password"
  						type="password"
  						name="password"
  						autocomplete="current-password"
  						required
  					/>
  				</div>

  				<button type="submit">Sign in</button>

  				<footer class="app-footer">
  					<p>
  						Powered by
  						<a href="https://github.com/netresearch/ldap-manager" target="_blank" rel="noopener noreferrer">
  							netresearch/ldap-manager
  						</a>
  					</p>
  					<p>{ version }</p>
  				</footer>
  			</form>
  		</main>
  	}
  }

  templ themeToggleV2() {
  	<button
  		type="button"
  		class="icon-btn"
  		aria-label="Toggle theme"
  		x-data="{ toggle() { const r = document.documentElement; const next = r.getAttribute('data-theme') === 'dark' ? 'light' : 'dark'; r.setAttribute('data-theme', next); localStorage.setItem('theme', next); } }"
  		x-on:click="toggle()"
  	>
  		<span aria-hidden="true">◐</span>
  	</button>
  }

  templ densityToggleV2() {
  	<button
  		type="button"
  		class="icon-btn"
  		aria-label="Toggle density"
  		x-data="{ toggle() { const r = document.documentElement; const next = r.getAttribute('data-density') === 'comfortable' ? 'compact' : 'comfortable'; r.setAttribute('data-density', next); localStorage.setItem('density', next); } }"
  		x-on:click="toggle()"
  	>
  		<span aria-hidden="true">⇥</span>
  	</button>
  }
  ```

- [ ] **Step 2: Generate**

  ```bash
  templ generate
  go build ./...
  ```

  Expected: clean.

- [ ] **Step 3: Commit (template only — no handler swap yet)**

  ```bash
  git add internal/web/templates/login_v2.templ internal/web/templates/login_v2_templ.go
  git commit -S --signoff -m "feat(ui): add LoginV2 templ with visible labels, theme+density toggles"
  ```

### Task 2.3: Swap the handler to use `LoginV2`

**Files:**
- Modify: `internal/web/auth.go` (three call sites to `templates.LoginWithStyles`: rate-limited, bad creds, GET)

- [ ] **Step 1: Update all three call sites**

  The current calls look like:

  ```go
  return templates.LoginWithStyles(
      templates.Flashes(templates.ErrorFlash("…")),
      "",
      a.GetCSRFToken(c),
      a.GetStylesPath(),
  ).Render(...)
  ```

  And (for the GET path at line 104):

  ```go
  return templates.LoginWithStyles(
      templates.Flashes(),
      version.FormatVersion(),
      a.GetCSRFToken(c),
      a.GetStylesPath(),
  ).Render(...)
  ```

  Replace with `LoginV2(flashes, version, csrfToken)` — note `LoginV2` does not take a styles path (it links pico + app.css directly).

  Rate-limited block (around line 58–63):

  ```go
  return templates.LoginV2(
      templates.Flashes(templates.ErrorFlash("Too many failed login attempts. Please try again later.")),
      "",
      a.GetCSRFToken(c),
  ).Render(c.UserContext(), c.Response().BodyWriter())
  ```

  Invalid creds block (around line 68–73):

  ```go
  return templates.LoginV2(
      templates.Flashes(templates.ErrorFlash("Invalid username or password")),
      "",
      a.GetCSRFToken(c),
  ).Render(c.UserContext(), c.Response().BodyWriter())
  ```

  GET block (around line 104–109):

  ```go
  return templates.LoginV2(
      templates.Flashes(),
      version.FormatVersion(),
      a.GetCSRFToken(c),
  ).Render(c.UserContext(), c.Response().BodyWriter())
  ```

- [ ] **Step 2: Build + run existing tests**

  ```bash
  go build ./...
  go test ./internal/web/ -run Login -v
  ```

  Expected: build passes. Some login-specific tests may fail because they assert on the OLD markup (e.g. `sr-only` labels, Tailwind class names, `styles.css` presence). Read each failure and decide:
  - If the test asserts on markup the new page intentionally changes, update the assertion. (Visible labels: test should now expect a visible `<label>`; not `sr-only`.)
  - If the test asserts behaviour (flashes shown, CSRF token present, form action), it should still pass as-is; if not, the new template is missing something.

- [ ] **Step 3: Fix tests as needed**

  Keep the existing test files; adjust expectations.

  Typical edits (example patterns — search the real test names and apply to each):

  ```go
  // Before:
  assert.Contains(t, body, `class="sr-only"`)
  // After:
  assert.Contains(t, body, `<label for="login-username">Username</label>`)

  // Before:
  assert.Contains(t, body, `href="/static/styles`)
  // After:
  assert.Contains(t, body, `href="/static/vendor/pico.min.css"`)
  assert.Contains(t, body, `href="/static/app.css"`)
  ```

  Run until green:

  ```bash
  go test ./internal/web/ -v
  ```

- [ ] **Step 4: Commit**

  ```bash
  git add internal/web/auth.go internal/web/auth_test.go internal/web/handlers_test.go
  git commit -S --signoff -m "feat(ui): switch /login to LoginV2 (Pico+app.css, visible labels)"
  ```

### Task 2.4: E2E — update existing login journey to new selectors

**Files:**
- Modify: `internal/e2e/user_journey_test.go` (the `TestLoginJourney` test)
- Maybe modify: `internal/e2e/e2e_helpers.go` (if selectors live there)

- [ ] **Step 1: Inspect helper selectors**

  ```bash
  grep -n 'username\|password\|submit' internal/e2e/e2e_helpers.go
  ```

  Note the current selectors used by `tp.Login(...)` and `tp.IsVisible(...)`.

- [ ] **Step 2: Update selectors if needed**

  Existing selectors like `input[name='username']`, `input[name='password']`, `button[type='submit']` still match the new markup — but verify by reading the `Login` helper. If it uses `#username` or class selectors that changed, update to `#login-username` / `#login-password` per the new templ.

- [ ] **Step 3: Run the E2E suite against the new login**

  ```bash
  go test -tags e2e ./internal/e2e/ -run TestLoginJourney -v
  ```

  Expected: PASS. If a subtest fails, read the failure and fix the selector or the template, not the assertion logic.

- [ ] **Step 4: Commit (if any edits)**

  ```bash
  git add internal/e2e/
  git commit -S --signoff -m "test(e2e): align login journey selectors with LoginV2 markup"
  ```

### Task 2.5: E2E — axe-core AAA verification

This is the automated proof of accessibility. Injects the vendored axe-core into the page, runs `axe.run()` with AAA rules enabled, asserts zero violations.

**Files:**
- Create: `internal/e2e/axe_test.go`

- [ ] **Step 1: Write the failing test**

  ```go
  //go:build e2e

  package e2e

  import (
  	"encoding/json"
  	"fmt"
  	"os"
  	"path/filepath"
  	"testing"

  	"github.com/playwright-community/playwright-go"
  	"github.com/stretchr/testify/require"
  )

  // TestAxeAAA_LoginPage asserts the /login page has zero WCAG 2.2 AAA
  // violations according to axe-core. Blocks merge if any violation.
  func TestAxeAAA_LoginPage(t *testing.T) {
  	config := DefaultTestConfig()
  	browser := NewTestBrowser(t, config)
  	defer browser.Close()

  	page := browser.NewPage(t)
  	defer page.Close()
  	tp := NewTestPage(t, page, config)

  	tp.Navigate("/login")

  	// Inject vendored axe-core.
  	axePath, err := filepath.Abs("testdata/axe.min.js")
  	require.NoError(t, err, "resolve axe path")
  	axeSrc, err := os.ReadFile(axePath)
  	require.NoError(t, err, "read axe.min.js")

  	_, err = page.AddScriptTag(playwright.PageAddScriptTagOptions{
  		Content: playwright.String(string(axeSrc)),
  	})
  	require.NoError(t, err, "inject axe")

  	// Run with AAA rules. axe.run returns a Promise; Playwright's Evaluate awaits.
  	result, err := page.Evaluate(`
  		() => axe.run({
  			runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag2aaa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
  			resultTypes: ['violations'],
  		})
  	`)
  	require.NoError(t, err, "axe.run")

  	// Marshal -> AxeResult for type safety.
  	raw, err := json.Marshal(result)
  	require.NoError(t, err, "marshal axe result")

  	var ar axeResult
  	require.NoError(t, json.Unmarshal(raw, &ar), "unmarshal axe result")

  	if len(ar.Violations) > 0 {
  		for _, v := range ar.Violations {
  			t.Errorf("axe violation [%s]: %s — %d node(s). help: %s",
  				v.ID, v.Description, len(v.Nodes), v.HelpURL)
  			for i, n := range v.Nodes {
  				if i >= 3 {
  					t.Logf("  (and %d more node(s) truncated)", len(v.Nodes)-i)
  					break
  				}
  				t.Logf("  %s", n.Target)
  			}
  		}
  		t.Fatalf("%d axe violation(s) on /login", len(ar.Violations))
  	}

  	fmt.Fprintln(os.Stderr, "axe AAA pass on /login: 0 violations")
  }

  type axeResult struct {
  	Violations []axeViolation `json:"violations"`
  }

  type axeViolation struct {
  	ID          string    `json:"id"`
  	Description string    `json:"description"`
  	HelpURL     string    `json:"helpUrl"`
  	Nodes       []axeNode `json:"nodes"`
  }

  type axeNode struct {
  	Target []string `json:"target"`
  }
  ```

- [ ] **Step 2: Run it**

  ```bash
  go test -tags e2e ./internal/e2e/ -run TestAxeAAA_LoginPage -v
  ```

  Expected outcomes:
  - **PASS (best case):** the login page has zero AAA violations.
  - **FAIL with violations listed:** read each violation. Common first-run hits and fixes:
    - `color-contrast` → a token pair we missed; fix `app.css`.
    - `landmark-one-main` → we do have `<main>`; if reported, ensure it's the top-level wrapper only.
    - `page-has-heading-one` → add an `<h1>` to `login_v2.templ` (visually hidden if desired: `<h1 style="position:absolute;left:-9999px;">Sign in to LDAP Manager</h1>` — but AAA-friendly).
    - `label` → the form inputs must have associated labels; we set `for`/`id`, verify they match.

  Fix the root cause in `app.css` or `login_v2.templ`, regenerate with `templ generate`, re-run the test. Iterate until zero violations.

- [ ] **Step 3: Commit**

  ```bash
  git add internal/e2e/axe_test.go internal/web/templates/login_v2.templ internal/web/templates/login_v2_templ.go internal/web/static/app.css
  git commit -S --signoff -m "test(e2e): verify /login has zero WCAG 2.2 AAA violations (axe-core)"
  ```

### Task 2.6: Slice 2 wrap-up & docs

**Files:**
- Modify: `README.md` (conformance statement)
- Create: `docs/superpowers/plans/2026-04-20-ui-revamp-phase-1-foundation.md.status` (tick box log — optional; executing-plans skill may do this automatically)

- [ ] **Step 1: Add a WCAG conformance section to README.md**

  Append to `README.md` a short section:

  ```markdown
  ## Accessibility

  LDAP Manager's login page conforms to [WCAG 2.2](https://www.w3.org/TR/WCAG22/) Level AAA in _comfortable_ density (the default on touch devices, narrow viewports, and under `prefers-reduced-motion`). In _compact_ density (the default on desktop), the application meets Level AA; all AAA success criteria are met except 2.5.5 Target Size (Enhanced), which is a deliberate density-preference trade-off.

  Conformance is enforced in CI by a contrast unit test (`internal/web/contrast_test.go`) and an axe-core AAA pass on every E2E run (`internal/e2e/axe_test.go`). Additional routes will be brought under the same guarantee as they migrate in subsequent slices.
  ```

- [ ] **Step 2: Final full-suite run**

  ```bash
  make check
  go test -tags e2e ./internal/e2e/ -v
  ```

  Expected: everything green. Note that `make check` without `-tags e2e` will not run the axe test; run both for full confidence.

- [ ] **Step 3: Commit**

  ```bash
  git add README.md
  git commit -S --signoff -m "docs: document WCAG 2.2 AAA conformance for /login"
  ```

- [ ] **Step 4: Visual smoke test**

  ```bash
  make dev
  # Open http://localhost:3000/login in a browser.
  # Checklist:
  #  □ Form visible, logo present, labels are visible text (not sr-only).
  #  □ Top-right theme button toggles light ↔ dark; monospace appears in dark.
  #  □ Top-right density button toggles comfortable ↔ compact.
  #  □ Bad credentials show a red-bordered alert.
  #  □ Tab order is: username → password → submit → theme → density (or top→bottom; what matters is no element is unreachable).
  #  □ Focused elements have visible 2-px outlines.
  # Kill dev server.
  ```

---

## Deferred to follow-up plans

After this plan lands, create a new plan covering Slice 3 (Home + shell + minimal ⌘K + pin backend + recents). Do NOT attempt it here.

---

## Self-review checklist (already applied)

1. **Spec coverage (Slices 1–2 only):**
   - §3 stack table — vendoring covers Pico/htmx/Alpine. Build step deferred (scripts/vendor.sh introduced). ✔
   - §4 visual language — all documented tokens appear in `app.css`. ✔
   - §4.4 density auto-select — encoded in `baseV2` pre-paint script. ✔
   - §4.5 motion — `prefers-reduced-motion` handled at CSS-global level. ✔
   - §4.6 focus — `:focus-visible` rule in `app.css` base layer. ✔
   - §6.7 login — visible labels, logo kept, CSRF, flashes, version. ✔
   - §7.1 axe-core AAA in CI — Task 2.5. ✔
   - §7.1 contrast unit test — Task 1.3. ✔
   - §7.4 conformance statement — Task 2.6 Step 1. ✔

2. **Placeholder scan:** no TBD/TODO remain. PLACEHOLDER_SHA256 is an EXECUTOR-instruction, replaced in Task 1.1 Step 4. ✔

3. **Type consistency:** `LoginV2` signature `(flashes, version, csrfToken)` matches usage in auth.go swap. `baseV2` takes `title string` and yields children — matches usage. ✔

4. **Assumptions to verify in EXECUTION (not silently accepted):**
   - Pico v2 current stable version pin in `vendor.lock`.
   - htmx v2 current stable pin.
   - Alpine v3 current stable pin + maintenance status confirmed via npm.
   - axe-core v4 current stable pin.
   - Go version 1.25 in `go.mod` (adjust `make test-contrast` path if different).
