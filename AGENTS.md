# AGENTS.md (root)

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2026-02-22 -->

This file explains repo-wide conventions and where to find scoped rules.
**Precedence:** the **closest `AGENTS.md`** to the files you're changing wins. Root holds global defaults only.

## Global rules

- Keep diffs small; add tests for new code paths
- Go: run `make format-go` + `make lint` before commit
- Frontend: run `make format-js` for JS/CSS/JSON
- All exported functions require godoc comments
- No `panic()` in production code

## Boundaries

### Always Do

- Run pre-commit checks before committing (`make check`)
- Add tests for new code paths (≥80% coverage required)
- Use conventional commit format

### Ask First

- Adding new dependencies
- Modifying CI/CD configuration
- Changing public API signatures
- Repo-wide refactoring

### Never Do

- Commit secrets, credentials, or sensitive data
- Use `panic()` in production code
- Push directly to main branch
- Log passwords, tokens, or session IDs
- Concatenate user input in LDAP filters (use `ldap.EscapeFilter()`)

## Minimal pre-commit checks

```bash
make check          # Full quality check (lint + test)
make format-all     # Format all code
make lint-security  # Security vulnerability checks
```

## Index of scoped AGENTS.md

| Scope   | Path                                                 | Purpose                                     |
| ------- | ---------------------------------------------------- | ------------------------------------------- |
| CLI     | [`cmd/AGENTS.md`](./cmd/AGENTS.md)                   | CLI entry point and main package            |
| Core    | [`internal/AGENTS.md`](./internal/AGENTS.md)         | Core application logic, LDAP client, config |
| Web     | [`internal/web/AGENTS.md`](./internal/web/AGENTS.md) | HTTP handlers, templates, middleware        |
| Scripts | [`scripts/AGENTS.md`](./scripts/AGENTS.md)           | Build scripts and tooling                   |
| Docs    | [`docs/AGENTS.md`](./docs/AGENTS.md)                 | Documentation standards                     |

## When instructions conflict

- The nearest `AGENTS.md` wins
- Explicit user prompts override files
- See scoped files for detailed patterns and examples
