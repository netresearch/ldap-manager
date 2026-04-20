# AGENTS.md — scripts/

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2026-04-20 -->

## Overview

Utility shell scripts used during development and CI. The Node.js asset-
pipeline scripts were removed together with Tailwind/PostCSS/TypeScript
in the Phase 1 UI revamp (Slice 8). The remaining scripts are shell-only.

**Scripts:**

- `vendor.sh` — Refreshes vendored third-party frontend files (Pico CSS,
  htmx, Alpine) from the versions pinned in `scripts/vendor.lock`.

## Setup & Environment

```bash
# POSIX-compatible shell (bash 5+ recommended)
bash --version

# curl and sha256sum must be on PATH
command -v curl sha256sum
```

## Running

```bash
# Refresh /internal/web/static/vendor/*
bash scripts/vendor.sh
```

`vendor.sh` downloads the pinned versions, verifies checksums against
`scripts/vendor.lock`, and writes to `internal/web/static/vendor/`.

## Code Style

- Shell scripts use `#!/usr/bin/env bash` with `set -euo pipefail`.
- Quote all variable expansions.
- Fail loudly on checksum mismatches; never silently overwrite.

## PR & Commit Checklist

- [ ] Shebang is `#!/usr/bin/env bash`
- [ ] `set -euo pipefail` near the top
- [ ] Arguments and env vars documented in comments
- [ ] `shellcheck` clean

## When stuck

1. **Checksum mismatch**: Check if the upstream file changed; update
   `scripts/vendor.lock` with the new SHA only after verifying upstream.
2. **Permission errors**: Ensure `scripts/*.sh` is executable
   (`chmod +x`).

## House Rules

- **Shell-only**: No Node.js scripts here since the JS build chain was
  removed. If a task needs Go, put it in `cmd/` or a `go:generate`
  directive instead.
- **Pinned versions**: All third-party downloads go through
  `vendor.lock`; drive-by version bumps are not allowed.
