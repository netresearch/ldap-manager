# AGENTS.md — internal/

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2026-01-16 -->

## Overview

Core application logic for LDAP Manager. All business logic, domain models, and service implementations live here.

**Packages:**

- `ldap/` — LDAP client wrapper, query builders, connection management
- `ldap_cache/` — Caching layer for LDAP queries
- `options/` — Configuration parsing and validation
- `version/` — Build version info (injected via ldflags)
- `web/` — HTTP handlers, templates, middleware (see `./web/AGENTS.md`)

**Entry points:**

- `web/server.go` — Main server initialization
- `ldap/client.go` — LDAP connection factory
- `options/options.go` — Configuration struct

## Setup & Environment

No special setup beyond root-level:

```bash
make setup       # Installs Go tools and deps
make setup-hooks # Install pre-commit hooks
go mod download  # Just Go deps
```

Required environment variables (see `.envrc` or `.env`):

- `LDAP_SERVER` — LDAP(S) server URL
- `LDAP_BASE_DN` — Search base DN
- `LDAP_READONLY_USER` — Bind user DN for initial connection
- `LDAP_READONLY_PASSWORD` — Bind password
- `COOKIE_SECURE` — Set to `true` for HTTPS, `false` for HTTP-only

## Build & Tests (File-scoped)

```bash
# Build all internal packages
go build ./internal/...

# Test specific package
go test ./internal/ldap/
go test ./internal/web/

# Test with coverage
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out

# Race detection
go test -race ./internal/...

# Benchmarks
go test -bench=. ./internal/...
```

## Code Style & Conventions

### Package Organization

Follow Go's standard project layout:

```
internal/
├── ldap/          # Domain: LDAP operations
│   ├── client.go  # Public API
│   ├── query.go   # Internal helpers
│   └── *_test.go  # Tests
├── web/           # Domain: HTTP layer
│   ├── server.go  # Public API
│   ├── handlers.go
│   └── *_test.go
└── options/       # Domain: Configuration
```

### Naming Conventions

- **Files**: `snake_case.go` or `kebab-case.go` (prefer snake for consistency)
- **Types**: `PascalCase` (exported), `camelCase` (unexported)
- **Functions**: `PascalCase` (exported), `camelCase` (unexported)
- **Interfaces**: Describe behavior (e.g., `LDAPClient`, `UserRepository`)

### Error Handling

```go
// Good: Wrap errors with context
if err := client.Search(filter); err != nil {
    return fmt.Errorf("failed to search users with filter %q: %w", filter, err)
}

// Good: Custom error types for domain errors
var ErrUserNotFound = errors.New("user not found")

// Bad: Generic errors
return errors.New("search failed") // No context

// Bad: Panic in library code
panic("invalid filter") // Use errors instead
```

### Logging

Use `zerolog` for all logging:

```go
// Good: Structured logging
log.Info().
    Str("user", username).
    Str("dn", userDN).
    Msg("User authenticated")

// Good: Error logging
log.Error().
    Err(err).
    Str("filter", filter).
    Msg("LDAP search failed")

// Bad: Print statements
fmt.Println("User logged in") // Use logger

// Bad: Logging sensitive data
log.Info().Str("password", pwd).Msg("Auth") // NEVER log passwords
```

### Testing Patterns

```go
// Good: Table-driven tests
func TestUserSearch(t *testing.T) {
    tests := []struct {
        name    string
        filter  string
        want    int
        wantErr bool
    }{
        {"valid user", "(uid=test)", 1, false},
        {"invalid filter", "invalid", 0, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := client.Search(tt.filter)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Len(t, got, tt.want)
        })
    }
}

// Good: Use testify/assert for readability
assert.Equal(t, expected, actual)
assert.NoError(t, err)

// Bad: Manual error checking
if err != nil {
    t.Errorf("unexpected error: %v", err)
}
```

## Security & Safety

### LDAP Query Safety

```go
// Good: Parameterized filters (use go-ldap escaping)
import "github.com/go-ldap/ldap/v3"

filter := fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(username))

// Bad: String concatenation (LDAP injection risk)
filter := "(uid=" + username + ")" // NEVER do this
```

### Sensitive Data Handling

- **Never log**: Passwords, tokens, session IDs, certificate keys
- **Redact in errors**: Sanitize sensitive data before returning errors
- **Secrets in memory**: Clear sensitive data after use when possible

```go
// Good: Redact sensitive info
log.Error().
    Str("user", user).
    Msg("Authentication failed") // Don't log the password

// Bad: Exposing secrets
log.Error().
    Str("password", pwd).
    Msg("Auth failed") // NEVER
```

### Input Validation

```go
// Good: Validate early
func ValidateUsername(username string) error {
    if len(username) == 0 {
        return errors.New("username cannot be empty")
    }
    if len(username) > 64 {
        return errors.New("username too long")
    }
    // Add regex validation if needed
    return nil
}

// Use before any processing
if err := ValidateUsername(input); err != nil {
    return err
}
```

## Code Style

### Go Code Standards

- Run `make format-go` (gofumpt + goimports) before commit
- All exported functions require godoc comments
- No `panic()` in production code - handle all errors explicitly
- Use `zerolog` for structured logging with appropriate levels
- Follow SOLID, KISS, DRY, YAGNI principles
- Configured in `.golangci.yml` and `.editorconfig`

### Package Organization

```
internal/
├── ldap/          # Domain: LDAP operations
├── ldap_cache/    # Domain: Caching layer
├── web/           # Domain: HTTP layer (see web/AGENTS.md)
├── options/       # Domain: Configuration
└── version/       # Domain: Build metadata
```

### Security Best Practices

- **LDAP injection prevention**: Always use `ldap.EscapeFilter()` for user input
- **Input validation**: Validate early, fail fast
- **Secrets handling**: Never log passwords, tokens, or session IDs
- **Error messages**: Redact sensitive data before returning

## PR & Commit Checklist

- [ ] All public functions have godoc comments
- [ ] Errors include context (use `fmt.Errorf` with `%w`)
- [ ] Tests cover new code (≥80% coverage required)
- [ ] No LDAP injection vulnerabilities (use escaping)
- [ ] No sensitive data in logs
- [ ] `go mod tidy` run after dependency changes
- [ ] `make format-go` - code formatted
- [ ] `make lint` - passes all linters
- [ ] `make test` - passes with coverage threshold

## Examples: Good vs Bad

### ✅ Good: Clean LDAP client

```go
// Search performs an LDAP search with the given filter.
// Returns empty slice if no results found.
func (c *Client) Search(filter string) ([]User, error) {
    req := ldap.NewSearchRequest(
        c.baseDN,
        ldap.ScopeWholeSubtree,
        ldap.NeverDerefAliases,
        0, 0, false,
        filter,
        []string{"uid", "cn", "mail"},
        nil,
    )

    result, err := c.conn.Search(req)
    if err != nil {
        return nil, fmt.Errorf("ldap search failed: %w", err)
    }

    return parseUsers(result.Entries), nil
}
```

### ❌ Bad: Unsafe query building

```go
// BAD: LDAP injection vulnerability
func (c *Client) UnsafeSearch(username string) error {
    filter := "(uid=" + username + ")" // NEVER concatenate user input
    // ... rest of code
}
```

### ✅ Good: Error wrapping with context

```go
if err := client.Search(filter); err != nil {
    return fmt.Errorf("failed to search users with filter %q: %w", filter, err)
}
```

### ❌ Bad: Generic errors without context

```go
return errors.New("search failed") // No context - where? why?
```

## When You're Stuck

1. **LDAP operations**: Check `internal/ldap/` for existing patterns
2. **Configuration**: See `internal/options/options.go` for struct tags and flag definitions
3. **Web handlers**: Review `internal/web/AGENTS.md` for HTTP patterns
4. **Testing**: Look at existing `*_test.go` files for table-driven examples
5. **Dependencies**: Use `internal/` packages for shared code, avoid circular deps
6. **Build issues**: Run `make clean && make setup && make build`
7. **Test failures**: Run `make test` for coverage, `make test-race` for race conditions

## House Rules

- **No panics**: Production code must handle all errors gracefully
- **Test coverage**: Minimum 80% enforced by `.testcoverage.yml`
- **LDAP safety**: Always escape user input with `ldap.EscapeFilter()`
- **Error context**: Use `fmt.Errorf` with `%w` for error wrapping
- **Logging**: Use `zerolog` for structured logging, never `fmt.Println()`
- **Godoc**: All exported functions must have godoc comments
- **Table-driven tests**: Use for multiple test cases, prefer `testify/assert`
