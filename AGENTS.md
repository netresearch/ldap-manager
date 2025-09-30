# AGENTS.md (root)

<!-- Managed by agent: keep sections and order; edit content, not structure. Last updated: 2025-09-30 -->

This file explains repo-wide conventions and where to find scoped rules.

**Precedence:** the *closest* `AGENTS.md` to your changes wins. Root holds global defaults only.

## Project Overview

LDAP Manager is a Go-based web application for managing LDAP/Active Directory users through a web interface. Built with:

- **Backend**: Go 1.25+ with Fiber v2 web framework
- **Frontend**: Templ templates + TailwindCSS v4
- **Build**: pnpm for assets, Go for binaries
- **Quality**: golangci-lint, pre-commit hooks, 80%+ test coverage

## Global Rules

- Keep diffs small; add tests for new code paths
- Ask first before: heavy deps, full e2e runs, or repo-wide rewrites
- Follow Conventional Commits format for all commits
- All code must pass lint, format, and test checks before commit

## Minimal Pre-commit Checks

Commands must be run from repository root:

```bash
# Go type checking & building
go build ./...

# Linting & formatting
make lint         # Runs golangci-lint, security checks, format validation
make fix          # Auto-fix formatting issues

# Testing
make test         # Full test suite with coverage (must be ≥80%)
make test-quick   # Quick test run without coverage

# Frontend assets
pnpm build:assets # Build CSS and templates

# Full quality check
make check        # Runs lint + test
```

## House Rules (Defaults)

### Commits & Branching

- Atomic commits with Conventional Commits format: `feat(scope):`, `fix:`, `chore:`, etc.
- Keep PRs small; split if >300 net LOC excluding locks/generated files
- Feature branches only, never commit directly to main

### Type Safety & Design

- Strict Go standards: no `panic()` in production code, handle all errors
- Follow SOLID, KISS, DRY, YAGNI principles
- Prefer composition over inheritance
- All exported functions must have godoc comments

### Dependency Hygiene

- New/updated deps must be latest stable & compatible
- Use `go get -u` and `pnpm update` for updates
- Run `go mod tidy` after dep changes
- Check license compatibility (MIT/Apache-2.0/BSD preferred)

### API & Versioning

- HTTP handlers follow RESTful conventions
- Breaking API changes require discussion and migration plan
- Semantic versioning for releases

### Security & Compliance

- No secrets in VCS (enforced by detect-secrets hook)
- All user input must be validated and sanitized
- LDAP queries use parameterized binds (no string concatenation)
- Run `make lint-security` (govulncheck) before commits

### Observability

- Use zerolog for structured logging
- Log levels: debug (dev), info (default), warn (recoverable issues), error (needs attention)
- Include context in logs: user ID, request ID where applicable

### Testing Standards

- Unit tests for all business logic
- Integration tests for LDAP operations (use Docker compose test profile)
- Minimum 80% code coverage (enforced in CI)
- Test file naming: `*_test.go` adjacent to source
- Use testify/assert for assertions

## Index of Scoped AGENTS.md

- [`./cmd/AGENTS.md`](cmd/AGENTS.md) — CLI entry point and main package
- [`./internal/AGENTS.md`](internal/AGENTS.md) — Core application logic
- [`./internal/web/AGENTS.md`](internal/web/AGENTS.md) — Web handlers, templates, and assets

## When Instructions Conflict

- The nearest `AGENTS.md` wins. Explicit user prompts override files.
- Security rules (secrets, validation) always take precedence.
- Quality gates (lint, test, coverage) are non-negotiable unless explicitly approved.

## Development Workflow

1. **Setup**: `make setup` (installs all dependencies and tools)
2. **Development**: `make dev` (starts hot-reload server)
3. **Before commit**: `make check` (lint + test)
4. **Pre-commit hooks**: Auto-run on `git commit` (install with `make setup-hooks`)

## Access Policy

**IMPORTANT**: Never use `localhost` to access the application.

- All services use Traefik for routing
- Access via configured domain in `compose.yml` labels
- Check `compose.yml` for the proper domain configuration

## Quick Reference

- **Full help**: `make help`
- **Documentation**: `docs/` directory (user guides, dev docs, operations)
- **Build info**: `make info`
- **Clean workspace**: `make clean`

## Decision Log

- **Makefile not modified**: Existing Makefile already has comprehensive help target and follows best practices
- **.envrc enhanced**: Added welcome message per requirements
- **No TypeScript**: Project uses Go templates (Templ), not TypeScript/JSX
- **Quality threshold**: 80% coverage enforced (from `.testcoverage.yml`)