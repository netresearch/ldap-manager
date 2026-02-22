# AGENTS.md — cmd/

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2026-02-22 -->

## Overview

The `cmd/` directory contains the main entry point for the LDAP Manager application. This follows Go's standard project layout for executable binaries.

- **Location**: `cmd/ldap-manager/main.go`
- **Purpose**: CLI initialization, configuration parsing, server startup
- **Framework**: Uses Fiber v2 web framework via `internal/web` package

## Setup & Environment

No special setup needed beyond root-level dependencies:

```bash
make setup        # Install all dependencies
make setup-hooks  # Install pre-commit hooks
```

Environment variables are loaded from:

1. `.envrc` (direnv - committed with help text)
2. `.env` (local overrides - gitignored)
3. CLI flags override all environment variables

## Build & Tests

### File-scoped Commands

```bash
# Build binary
go build ./cmd/ldap-manager

# Run directly (requires .env or CLI flags)
go run ./cmd/ldap-manager

# Build with version info
make build        # Uses ldflags for version/commit/timestamp
```

### Testing

```bash
# Test this package (if tests exist)
go test ./cmd/ldap-manager/...

# Integration test (via Makefile)
make test
```

## Code Style & Conventions

### Main Package Patterns

- **Single responsibility**: `main.go` should only:
  1. Parse configuration (flags, env vars, .env file)
  2. Initialize logger
  3. Create and start server
  4. Handle graceful shutdown
- **Keep it thin**: Business logic belongs in `internal/`, not in `main()`
- **Exit codes**: Use `os.Exit(1)` for fatal errors, `os.Exit(0)` for clean shutdown

### Configuration Precedence

Follow this order (higher wins):

1. CLI flags (`-ldap-server`, `-port`, etc.)
2. Environment variables (`LDAP_SERVER`, `PORT`, etc.)
3. `.env` file values
4. Built-in defaults

### Error Handling

```go
// Good: Log and exit cleanly
if err := loadConfig(); err != nil {
    log.Fatal().Err(err).Msg("Failed to load configuration")
    os.Exit(1)
}

// Bad: Panic in main
panic("config error") // Never do this
```

### Logging in main()

```go
// Good: Structured logging with zerolog
log := zerolog.New(os.Stdout).With().Timestamp().Logger()
log.Info().Str("version", version.Version).Msg("Starting LDAP Manager")

// Bad: fmt.Println
fmt.Println("Starting server...") // Use logger instead
```

## Security & Safety

- **Secrets**: Never log sensitive values (passwords, tokens, session keys)
- **Validation**: Validate all configuration before server start
- **Graceful shutdown**: Always register signal handlers (SIGTERM, SIGINT)

## PR & Commit Checklist

- [ ] Configuration changes documented in `.envrc` or `.env.example`
- [ ] New CLI flags have help text (`-h` output)
- [ ] Version info still builds correctly (`make build`)
- [ ] No secrets in code or logs
- [ ] Graceful shutdown tested manually
- [ ] `make format-go` - code formatted
- [ ] `make lint` - passes linters
- [ ] `make test` - tests pass

## Examples: Good vs Bad

### ✅ Good: Minimal main.go

```go
// cmd/ldap-manager/main.go
func main() {
    cfg := options.Parse()         // Parse config
    log := setupLogger(cfg)        // Init logger
    srv := web.NewServer(cfg, log) // Create server

    if err := srv.Start(); err != nil {
        log.Fatal().Err(err).Msg("Server failed")
    }
}
```

### ❌ Bad: Business logic in main

```go
// cmd/ldap-manager/main.go
func main() {
    // ... setup ...

    // BAD: LDAP connection logic in main
    conn, err := ldap.Dial("tcp", cfg.Server)
    // ... complex LDAP logic ...

    // This belongs in internal/ldap/ package
}
```

### ✅ Good: Error handling in main

```go
if err := loadConfig(); err != nil {
    log.Fatal().Err(err).Msg("Failed to load configuration")
    os.Exit(1)
}
```

### ❌ Bad: Panic in main

```go
panic("config error") // Never do this - use log.Fatal() instead
```

## When stuck

1. Check existing patterns in `cmd/ldap-manager/main.go`
2. Review `internal/options/` for configuration handling
3. Look at `internal/web/server.go` for server initialization
4. See root `README.md` for CLI usage examples
5. Run `make help` for all available commands

## House Rules

- **Keep main() thin**: Only setup, start, shutdown - no business logic
- **Configuration precedence**: CLI flags > env vars > .envrc > defaults
- **Exit codes**: Use `os.Exit(1)` for errors, `os.Exit(0)` for clean shutdown
- **No panic**: Use `log.Fatal()` instead of `panic()` in main
- **Structured logging**: Always use `zerolog`, never `fmt.Println()`
- **Graceful shutdown**: Always handle SIGTERM/SIGINT for clean shutdown
