# AGENTS.md (root)

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2025-10-02 -->

This file explains repo-wide conventions and where to find scoped rules.

**Precedence:** the _closest_ `AGENTS.md` to your changes wins. Root holds global defaults only.

## Project Overview

LDAP Manager is a Go-based web application for managing LDAP/Active Directory users through a web interface. Built with:

- **Backend**: Go 1.25+ with Fiber v2 web framework
- **Frontend**: Templ templates + TailwindCSS v4
- **Build**: pnpm for assets, Go for binaries
- **Quality**: golangci-lint, pre-commit hooks, 80%+ test coverage

## Setup & Environment

### Prerequisites
- Go 1.25+
- pnpm 10.17.1+ (via packageManager field)
- Docker & Docker Compose v2
- Make

### Initial Setup
```bash
make setup         # Install all dependencies and tools
make setup-hooks   # Install pre-commit hooks
```

### Environment Variables
- `.envrc` - direnv configuration (committed with help text)
- `.env` - secrets and local overrides (gitignored)
- See `.envrc` for required variables and setup help

## Build & Tests

### Local Development
```bash
make dev           # Start hot-reload dev server (pnpm + concurrently)
make up            # Start Docker services (LDAP + app)
make watch         # Watch and rebuild assets on change
make logs-app      # View application logs
```

### Building
```bash
make build         # Build application binary (requires assets)
pnpm build:assets  # Build CSS + templates (production)
make build-release # Build optimized binaries (Linux, macOS, Windows)
```

### Testing
```bash
make test          # Full test suite with coverage (≥80% required)
make test-quick    # Quick test without coverage
make test-race     # Race detection tests
make benchmark     # Performance benchmarks
go test ./...      # Native Go test runner
```

### Quality Checks
```bash
make check         # Full quality check (lint + test)
make lint          # All linting and static analysis
make lint-security # Security vulnerability checks (govulncheck)
make format-all    # Format all code (Go + JS/CSS)
```

## Code Style

### Go Code
- Run `make format-go` (gofumpt + goimports) before commit
- All exported functions require godoc comments
- No `panic()` in production code - handle all errors explicitly
- Use `zerolog` for structured logging with appropriate levels
- Follow SOLID, KISS, DRY, YAGNI principles
- Configured in `.golangci.yml` (see file for linter settings)

### Frontend Code
- Run `make format-js` (prettier) for JS/JSON/CSS
- Templates use Templ syntax (`.templ` files)
- TailwindCSS v4 for styling - no custom CSS unless necessary
- Build assets with `pnpm build:assets` before testing frontend changes

### File Organization
- Tests: `*_test.go` adjacent to source files
- Mocks: `internal/mocks/` directory
- Templates: `internal/web/templates/`
- Static assets: `internal/web/static/`
- **Claude Code Files**: ALL investigation, analysis, and temporary reports go in `claudedocs/` (gitignored)
  - Bug reports, pool analysis, agent changelogs → `claudedocs/`
  - Use descriptive names with dates: `claudedocs/pool-investigation-2025-10-04.md`
  - NEVER create investigation files in project root

## Security

### Critical Rules
- **No secrets in VCS** - enforced by detect-secrets hook
- **Input validation** - all user input must be validated and sanitized
- **LDAP queries** - use parameterized binds, never string concatenation
- **Run security checks** - `make lint-security` (govulncheck) before commits
- **Cookie security** - configure `COOKIE_SECURE=true` for HTTPS environments

### Dependency Security
- New/updated deps must be latest stable & license-compatible (MIT/Apache-2.0/BSD preferred)
- Run `go mod tidy` after dependency changes
- Check for vulnerabilities with `make lint-security`

### Access Control
- **NEVER use localhost** to access the application
- All services use Traefik for routing (see `compose.yml` labels)
- Access via configured domains only

## PR & Commit Checklist

Before opening a PR:
- [ ] `make format-all` - all code formatted
- [ ] `make lint` - passes all linters
- [ ] `make test` - tests pass with ≥80% coverage
- [ ] `make lint-security` - no security vulnerabilities
- [ ] Conventional Commit format used for all commits
- [ ] PRs kept small (<300 net LOC excluding locks/generated files)
- [ ] Feature branch used (never commit to main)
- [ ] All exported functions have godoc comments

## Examples: Good vs Bad

### ✅ Good
```go
// GetUser retrieves a user by DN from LDAP
func (s *Service) GetUser(ctx context.Context, dn string) (*User, error) {
    if dn == "" {
        return nil, ErrInvalidDN
    }
    // ... implementation
}
```

### ❌ Bad
```go
// No godoc, panic on error, no validation
func (s *Service) GetUser(dn string) *User {
    user := s.ldap.Search(dn)
    if user == nil {
        panic("user not found")
    }
    return user
}
```

### ✅ Good Commit
```
feat(auth): add LDAP group-based authorization

Implements role checking based on LDAP group membership.
Adds middleware to validate user groups against required roles.

Closes #123
```

### ❌ Bad Commit
```
fixed stuff
```

## When You're Stuck

1. **Check documentation**: `docs/` directory has user guides, dev docs, operations manuals
2. **Run diagnostics**: `make health` (service health), `make stats` (resource usage)
3. **View logs**: `make logs-app` (application), `make logs-ldap` (LDAP server)
4. **Debug mode**: `make debug` (starts app in debug mode)
5. **Clean start**: `make fresh` (clean everything and restart)
6. **Full help**: `make help` (all available commands)

## House Rules

### Type Safety & Design
- Strict Go standards - no `panic()` in production, handle all errors
- Prefer composition over inheritance
- Follow SOLID, KISS, DRY, YAGNI principles

### API & Versioning
- HTTP handlers follow RESTful conventions
- Breaking changes require discussion and migration plan
- Semantic versioning for releases

### Observability
- Use `zerolog` for structured logging
- Log levels: debug (dev), info (default), warn (recoverable), error (attention needed)
- Include context in logs: user ID, request ID where applicable

### Testing Standards
- Unit tests for all business logic
- Integration tests for LDAP operations (Docker compose test profile)
- Minimum 80% code coverage (enforced by `.testcoverage.yml`)
- Test file naming: `*_test.go` adjacent to source
- Use `testify/assert` for assertions

## Index of Scoped AGENTS.md

- [`./cmd/AGENTS.md`](cmd/AGENTS.md) — CLI entry point and main package
- [`./internal/AGENTS.md`](internal/AGENTS.md) — Core application logic
- [`./internal/web/AGENTS.md`](internal/web/AGENTS.md) — Web handlers, templates, and assets
- [`./scripts/AGENTS.md`](scripts/AGENTS.md) — Utility scripts and tooling
