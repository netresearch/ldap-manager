# AGENTS.md Update Changelog

**Date**: 2025-10-02
**Author**: Claude Code
**Purpose**: Update all AGENTS.md files to follow public agents.md convention

## Summary

Updated LDAP Manager repository to follow the latest public agents.md convention with:
- Thin root AGENTS.md with global defaults
- Scoped AGENTS.md files for cmd/, internal/, internal/web/, and scripts/
- Proper 9-section schema structure
- Validated all commands against Makefile and package.json
- Added missing .editorconfig fundamental

## Files Modified

### 1. /.editorconfig (NEW)
**Status**: ✅ Created
**Purpose**: Cross-editor configuration standard (missing fundamental)

Added comprehensive EditorConfig with:
- Global defaults (UTF-8, LF, final newline, trim whitespace)
- Go-specific rules (tabs, indent size 4)
- Frontend rules (spaces, indent size 2 for JS/TS/CSS/HTML)
- Makefile rules (tabs required)
- Templ template rules (spaces, indent size 2)

### 2. /AGENTS.md (ROOT)
**Status**: ✅ Updated
**Changes**:
- Updated header with latest convention format and date (2025-10-02)
- Restructured to follow 9-section schema:
  1. ✅ Overview
  2. ✅ Setup & Environment
  3. ✅ Build & Tests
  4. ✅ Code Style
  5. ✅ Security
  6. ✅ PR & Commit Checklist
  7. ✅ Examples: Good vs Bad
  8. ✅ When You're Stuck
  9. ✅ House Rules
- Added comprehensive Setup & Environment section with prerequisites
- Expanded Build & Tests with all verified commands
- Added Security section with critical rules and cookie security details
- Enhanced PR checklist with all quality gates
- Added practical Good vs Bad examples for Go code
- Included troubleshooting guide with make commands
- Added House Rules with consolidated best practices
- Updated Index of Scoped AGENTS.md to include scripts/

**Commands Validated**:
- ✅ `make setup` - Install dependencies and tools
- ✅ `make setup-hooks` - Install pre-commit hooks
- ✅ `make dev` - Start hot-reload dev server
- ✅ `make up` - Start Docker services
- ✅ `make watch` - Watch and rebuild assets
- ✅ `make logs-app` - View application logs
- ✅ `make build` - Build application binary
- ✅ `pnpm build:assets` - Build CSS + templates
- ✅ `make build-release` - Build optimized binaries
- ✅ `make test` - Full test suite with coverage
- ✅ `make test-quick` - Quick test without coverage
- ✅ `make test-race` - Race detection tests
- ✅ `make benchmark` - Performance benchmarks
- ✅ `go test ./...` - Native Go test runner
- ✅ `make check` - Full quality check (lint + test)
- ✅ `make lint` - All linting and static analysis
- ✅ `make lint-security` - Security checks (govulncheck)
- ✅ `make format-all` - Format all code

### 3. /cmd/AGENTS.md
**Status**: ✅ Updated
**Changes**:
- Updated header with latest convention format and date
- Enhanced Setup & Environment with .envrc and CLI flag precedence
- Updated Build & Tests section with verified commands
- Renamed "PR/Commit Checklist" to "PR & Commit Checklist" for consistency
- Renamed "Good vs. Bad Examples" to "Examples: Good vs Bad" for consistency
- Added "When You're Stuck" section with troubleshooting steps
- Added House Rules section with main package best practices
- Added more Good vs Bad examples (error handling, logging)

**Commands Validated**:
- ✅ `make setup` - Install dependencies
- ✅ `make setup-hooks` - Install pre-commit hooks
- ✅ `go build ./cmd/ldap-manager` - Build binary
- ✅ `go run ./cmd/ldap-manager` - Run directly
- ✅ `make build` - Build with version info
- ✅ `go test ./cmd/ldap-manager/...` - Test package
- ✅ `make test` - Integration test
- ✅ `make format-go` - Format Go code
- ✅ `make lint` - Run linters
- ✅ `make test` - Run tests

### 4. /internal/AGENTS.md
**Status**: ✅ Updated
**Changes**:
- Updated header with latest convention format and date
- Enhanced Setup & Environment with COOKIE_SECURE variable
- Added Code Style section with Go standards, package organization, security practices
- Renamed "PR/Commit Checklist" to "PR & Commit Checklist" for consistency
- Updated checklist with ≥80% coverage requirement and make commands
- Renamed "Good vs. Bad Examples" to "Examples: Good vs Bad"
- Added more examples (error wrapping, generic errors)
- Enhanced "When You're Stuck" with additional troubleshooting steps
- Added House Rules section with core internal/ principles

**Commands Validated**:
- ✅ `make setup` - Install Go tools and deps
- ✅ `make setup-hooks` - Install pre-commit hooks
- ✅ `go mod download` - Just Go deps
- ✅ `go build ./internal/...` - Build all internal packages
- ✅ `go test ./internal/ldap/` - Test specific package
- ✅ `go test ./internal/web/` - Test web package
- ✅ `go test -coverprofile=coverage.out ./internal/...` - Test with coverage
- ✅ `go tool cover -html=coverage.out` - View coverage HTML
- ✅ `go test -race ./internal/...` - Race detection
- ✅ `go test -bench=. ./internal/...` - Benchmarks
- ✅ `make format-go` - Format Go code
- ✅ `make lint` - Run linters
- ✅ `make test` - Run tests

### 5. /internal/web/AGENTS.md
**Status**: ✅ Updated
**Changes**:
- Updated header with latest convention format and date
- Enhanced Setup & Environment with COOKIE_SECURE and make watch
- Added Code Style section with web layer standards and security requirements
- Added Security section with critical web security rules and session configuration
- Renamed "PR/Commit Checklist" to "PR & Commit Checklist"
- Updated checklist with all make commands
- Renamed "Good vs. Bad Examples" to "Examples: Good vs Bad"
- Added secure/insecure session handling examples
- Enhanced "When You're Stuck" with CSRF and asset troubleshooting
- Added House Rules section with web-specific principles

**Commands Validated**:
- ✅ `make setup` - Install Go + Node dependencies
- ✅ `make setup-hooks` - Install pre-commit hooks
- ✅ `go install github.com/a-h/templ/cmd/templ@latest` - Install templ
- ✅ `pnpm build:assets` - Build frontend assets
- ✅ `make dev` - Hot reload dev server
- ✅ `make watch` - Watch and rebuild assets
- ✅ `go build ./internal/web` - Build web package
- ✅ `go test ./internal/web/` - Test web handlers
- ✅ `go test -v ./internal/web/ -run TestAuthHandler` - Specific test
- ✅ `go test -coverprofile=coverage.out ./internal/web/` - Coverage
- ✅ `pnpm css:build` - Build CSS
- ✅ `pnpm templ:build` - Generate Go from .templ
- ✅ `pnpm build:assets` - Build both CSS and templates
- ✅ `pnpm dev` - Auto-rebuild on changes
- ✅ `pnpm css:dev` - CSS watch mode
- ✅ `pnpm css:build:prod` - Production CSS
- ✅ `pnpm css:analyze` - Analyze CSS bundle
- ✅ `pnpm templ:build` - Generate templates
- ✅ `pnpm templ:dev` - Template watch mode
- ✅ `make format-all` - Format all code
- ✅ `make lint` - Run linters
- ✅ `make test` - Run tests

### 6. /scripts/AGENTS.md (NEW)
**Status**: ✅ Created
**Purpose**: Document utility scripts for build automation

Added comprehensive documentation for:
- `cache-bust.mjs` - CSS cache-busting with MD5 hashing
- `analyze-css.mjs` - CSS bundle analysis and reporting

Includes:
- Overview of script purposes
- Setup & Environment (Node.js 18+ ESM requirements)
- Build & Tests with script execution examples
- Code Style for Node.js ESM scripts
- Security section with safe file operations
- PR & Commit Checklist for scripts
- Examples: Good vs Bad for script patterns
- When You're Stuck troubleshooting guide
- House Rules for script development

**Commands Validated**:
- ✅ `node scripts/cache-bust.mjs` - Run cache-busting
- ✅ `pnpm css:build:prod` - Build CSS + cache-bust
- ✅ `node scripts/analyze-css.mjs` - Run CSS analysis
- ✅ `pnpm css:analyze` - Analyze CSS bundle
- ✅ `make format-js` - Format JavaScript

## Convention Compliance

### ✅ Structure (9-section schema)
1. ✅ Overview - Project/scope description
2. ✅ Setup & Environment - Prerequisites and setup
3. ✅ Build & Tests - File-scoped commands
4. ✅ Code Style - Style rules and conventions
5. ✅ Security - Security rules and practices
6. ✅ PR & Commit Checklist - Pre-commit requirements
7. ✅ Examples: Good vs Bad - Practical examples
8. ✅ When You're Stuck - Troubleshooting guide
9. ✅ House Rules - Core principles

### ✅ Header Format
```markdown
<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2025-10-02 -->
```

### ✅ Command Validation
- All Makefile commands verified against actual Makefile
- All pnpm scripts verified against package.json
- All go commands tested for validity
- File-scoped commands clearly marked

### ✅ Precedence Rules
- Root AGENTS.md holds global defaults only
- Scoped AGENTS.md files override root for their domains
- Explicit user prompts override all files
- Security rules always take precedence
- Quality gates are non-negotiable

### ✅ Index Maintenance
Root AGENTS.md includes complete index:
- ✅ `./cmd/AGENTS.md` — CLI entry point and main package
- ✅ `./internal/AGENTS.md` — Core application logic
- ✅ `./internal/web/AGENTS.md` — Web handlers, templates, and assets
- ✅ `./scripts/AGENTS.md` — Utility scripts and tooling

## Missing Fundamentals Addressed

### ✅ .editorconfig
**Status**: Created
**Purpose**: Cross-editor configuration consistency

Added comprehensive configuration for:
- Global defaults (UTF-8, LF, final newline, whitespace)
- Go (tabs, 4-space indent)
- JavaScript/TypeScript (spaces, 2-space indent)
- CSS/HTML (spaces, 2-space indent)
- Makefile (tabs)
- Templ templates (spaces, 2-space indent)

### ✅ .envrc
**Status**: Verified existing
**Content**: Already has good help text and structure

### ⚠️ husky/commitlint
**Status**: Not added (out of scope)
**Reason**: No pre-commit framework requested, would require npm deps

### ✅ .golangci.yml
**Status**: Verified existing
**Content**: Comprehensive linter configuration already in place

## Command Verification Summary

### Makefile Commands (All Verified ✅)
- Application Control: up, down, restart, start, stop, ps, shell-app, shell-ldap, logs, logs-app, logs-ldap, rebuild, fresh
- Development Workflow: watch, css, css-watch, templates, templates-watch, build-assets, format-go, format-js, format-all, fix, dev, serve
- Building: build, build-assets, build-release, docker, docker-run, docker-dev-build, docker-dev
- Testing & Quality: test, test-quick, test-short, test-race, benchmark, docker-test, docker-lint, docker-check, lint, lint-go, lint-security, lint-format, lint-complexity, check, check-all, release
- Database & LDAP: ldap-reset, ldap-admin, sessions-clean
- Monitoring & Debugging: health, stats, inspect, debug, docker-shell
- Quick Access: open, ldap-admin, urls
- Dependencies & Setup: setup, setup-go, setup-node, setup-tools, setup-hooks, deps
- Cleanup: clean, docker-clean
- Git Workflow: git-status, commit, push
- Information: info, help

### pnpm Scripts (All Verified ✅)
- start: Build assets and run Go server
- dev: Hot-reload development mode (CSS + templates + Go)
- build: Production build (assets + Go binary)
- build:assets: Build both CSS and templates (production)
- build:assets:dev: Build CSS and templates (development)
- build:assets:prod: Build CSS and templates (production)
- css:build: Build CSS (production)
- css:build:dev: Build CSS (development)
- css:build:prod: Build CSS (production) + cache-bust
- css:dev: Watch CSS (development)
- css:analyze: Build CSS + run analysis script
- templ:build: Generate Go code from .templ files
- templ:dev: Watch .templ files and regenerate
- go:start: Run Go application
- go:build: Build Go binary
- go:dev: Watch Go files and restart with debug settings

### Go Commands (All Verified ✅)
- go build ./... - Build all packages
- go test ./... - Run all tests
- go test -coverprofile=coverage.out ./... - Test with coverage
- go tool cover -html=coverage.out - View coverage HTML
- go test -race ./... - Race detection
- go test -bench=. ./... - Run benchmarks
- go mod tidy - Clean up dependencies
- go mod download - Download dependencies
- go install github.com/a-h/templ/cmd/templ@latest - Install templ CLI

### Script Commands (All Verified ✅)
- node scripts/cache-bust.mjs - CSS cache-busting
- node scripts/analyze-css.mjs - CSS analysis

## Quality Assurance

### ✅ All Files Follow Convention
- Header format: `<!-- Managed by agent: keep sections & order; ... -->`
- 9-section schema applied consistently
- File-scoped commands clearly marked
- Precedence rules documented
- Index maintained in root

### ✅ All Commands Validated
- Cross-referenced against Makefile
- Cross-referenced against package.json
- Tested for existence and validity
- No broken or invalid commands

### ✅ Consistency Checks
- Section naming consistent across all files
- "PR & Commit Checklist" (not "PR/Commit")
- "Examples: Good vs Bad" (not "Good vs. Bad")
- "When You're Stuck" (not "When Stuck")
- All dates updated to 2025-10-02

### ✅ Content Completeness
- All sections have substantial content
- Examples are practical and actionable
- Security rules are clear and specific
- Troubleshooting steps are helpful
- House Rules capture core principles

## Next Steps

### Recommended (Optional)
1. Add husky + commitlint for automated commit message validation
2. Consider adding scripts/AGENTS.md reference to package.json scripts
3. Add AGENTS.md validation to pre-commit hooks
4. Create make targets for AGENTS.md validation

### Maintenance
1. Update AGENTS.md files when adding new commands
2. Keep precedence rules synchronized
3. Validate all commands after Makefile/package.json changes
4. Update dates in headers when making changes
5. Maintain index in root AGENTS.md when adding new scoped files

## Files Created/Modified

**Created**:
- ✅ .editorconfig (new fundamental)
- ✅ scripts/AGENTS.md (new scoped file)
- ✅ AGENTS_CHANGELOG.md (this file)

**Modified**:
- ✅ AGENTS.md (root - comprehensive update)
- ✅ cmd/AGENTS.md (updated to convention)
- ✅ internal/AGENTS.md (updated to convention)
- ✅ internal/web/AGENTS.md (updated to convention)

**Total**: 3 new files, 4 modified files

---

**Completion Date**: 2025-10-02
**Validation Status**: ✅ All commands verified
**Convention Compliance**: ✅ 100%
**Quality Check**: ✅ Passed
