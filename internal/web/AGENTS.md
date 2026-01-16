# AGENTS.md — internal/web/

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2026-01-16 -->

## Overview

HTTP layer for LDAP Manager using Fiber v2 framework and Templ templates.

**Key files:**

- `server.go` — Server setup, routing, middleware registration
- `auth.go` — Authentication handlers and session management
- `users.go` — User management endpoints
- `groups.go` — Group management endpoints
- `computers.go` — Computer object endpoints
- `health.go` — Health check and readiness endpoints
- `middleware.go` — Custom middleware (logging, auth, CSRF)
- `template_cache.go` — Templ template preloading and caching
- `assets.go` — Static asset serving

**Frontend:**

- `templates/` — Templ template files (`.templ`)
- `static/` — CSS, images, favicon
- `tailwind.css` — TailwindCSS input (builds to `static/styles.css`)

## Setup & Environment

```bash
# Install Go + Node dependencies
make setup
make setup-hooks  # Install pre-commit hooks

# Install templ CLI (if not already installed)
go install github.com/a-h/templ/cmd/templ@latest

# Build frontend assets
pnpm build:assets

# Development mode (hot reload)
make dev    # Watches: CSS, templates, Go files
make watch  # Alternative: watch and rebuild assets
```

Environment variables (see `.envrc` or `.env`):

- `PORT` — HTTP listen port (default: 3000)
- `SESSION_KEY` — Secret key for session encryption (auto-generated if not set)
- `COOKIE_SECURE` — Set to `true` for HTTPS, `false` for HTTP-only
- `LOG_LEVEL` — Logging level (debug/info/warn/error)

## Build & Tests (File-scoped)

```bash
# Build web package
go build ./internal/web

# Test web handlers
go test ./internal/web/
go test -v ./internal/web/ -run TestAuthHandler

# Test with coverage
go test -coverprofile=coverage.out ./internal/web/
go tool cover -html=coverage.out

# Frontend assets
pnpm css:build     # Build CSS
pnpm templ:build   # Generate Go from .templ files
pnpm build:assets  # Build both

# Development watch
pnpm dev           # Auto-rebuild on changes
```

## Code Style & Conventions

### Handler Patterns

Follow Fiber v2 conventions:

```go
// Good: Handler signature
func HandleUsers(c *fiber.Ctx) error {
    // 1. Parse and validate input
    var req UserRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
    }

    // 2. Call business logic (from internal/ldap)
    users, err := ldapClient.SearchUsers(req.Filter)
    if err != nil {
        log.Error().Err(err).Msg("Failed to search users")
        return c.Status(500).JSON(fiber.Map{"error": "search failed"})
    }

    // 3. Return response
    return c.JSON(users)
}

// Bad: Business logic in handler
func BadHandler(c *fiber.Ctx) error {
    // BAD: LDAP operations directly in handler
    conn, _ := ldap.Dial("tcp", "...")
    result, _ := conn.Search(...)
    // ... complex logic ...

    // Handlers should be thin wrappers
}
```

### Routing Organization

```go
// Good: Logical route grouping in server.go
func (s *Server) setupRoutes() {
    // Public routes
    s.app.Get("/health", s.handleHealth)
    s.app.Get("/ready", s.handleReady)

    // Auth routes
    auth := s.app.Group("/auth")
    auth.Post("/login", s.handleLogin)
    auth.Post("/logout", s.handleLogout)

    // Protected API routes
    api := s.app.Group("/api", s.authMiddleware)
    api.Get("/users", s.handleUsers)
    api.Post("/users", s.handleCreateUser)
}
```

### Template Patterns (Templ)

```templ
// Good: Component-based templates
templ UserCard(user User) {
    <div class="card">
        <h3>{user.Name}</h3>
        <p>{user.Email}</p>
    </div>
}

templ UserList(users []User) {
    <div class="user-list">
        for _, user := range users {
            @UserCard(user)
        }
    </div>
}

// Use in handler:
// return UserList(users).Render(c.Context(), c.Response().BodyWriter())
```

### Middleware Conventions

```go
// Good: Middleware with logging
func AuthMiddleware(c *fiber.Ctx) error {
    session := c.Locals("session")
    if session == nil {
        log.Warn().
            Str("path", c.Path()).
            Msg("Unauthorized access attempt")
        return c.Redirect("/auth/login")
    }
    return c.Next()
}

// Register early in chain
app.Use(LoggingMiddleware)  // First: log all requests
app.Use(AuthMiddleware)      // Then: check auth
```

### Error Handling

```go
// Good: Consistent error responses
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message,omitempty"`
}

func handleError(c *fiber.Ctx, status int, err error) error {
    log.Error().
        Err(err).
        Str("path", c.Path()).
        Msg("Request failed")

    return c.Status(status).JSON(ErrorResponse{
        Error:   http.StatusText(status),
        Message: err.Error(), // Be careful not to leak sensitive info
    })
}

// Usage
if err != nil {
    return handleError(c, 500, err)
}
```

## Security & Safety

### Input Validation

```go
// Good: Validate all user input
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,alphanum,min=3,max=32"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

// Use validator library or custom validation
if err := validate.Struct(req); err != nil {
    return c.Status(400).JSON(fiber.Map{"error": "validation failed"})
}
```

### CSRF Protection

```go
// CSRF middleware enabled for state-changing operations
app.Use(csrf.New(csrf.Config{
    KeyLookup:      "header:X-CSRF-Token",
    CookieName:     "csrf_",
    CookieSameSite: "Strict",
    Expiration:     1 * time.Hour,
}))
```

### Session Security

```go
// Good: Secure session configuration
store := session.New(session.Config{
    Expiration:   24 * time.Hour,
    CookieSecure: true, // HTTPS only in production
    CookieHTTPOnly: true, // No JavaScript access
    CookieSameSite: "Strict",
})

// Regenerate session ID after login
sess, _ := store.Get(c)
sess.Regenerate() // Prevent session fixation
sess.Set("user_id", user.ID)
sess.Save()
```

### Content Security

```go
// Good: Set security headers
app.Use(func(c *fiber.Ctx) error {
    c.Set("X-Content-Type-Options", "nosniff")
    c.Set("X-Frame-Options", "DENY")
    c.Set("X-XSS-Protection", "1; mode=block")
    c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
    return c.Next()
})
```

## Frontend Assets & Styling

### TailwindCSS

```bash
# Development (watch mode)
pnpm css:dev

# Production (minified + purged)
pnpm css:build:prod

# Analyze bundle size
pnpm css:analyze
```

Configuration: `tailwind.config.js` and `postcss.config.mjs`

### Template Development

```bash
# Generate Go code from .templ files
pnpm templ:build

# Watch mode (auto-regenerate)
pnpm templ:dev
```

Templ files in `templates/` compile to `*_templ.go` (excluded from linting).

### Static Assets

- Place in `static/` directory
- Served at `/static/*` route
- Cache busting: `scripts/cache-bust.mjs` adds hashes to filenames

## Code Style

### Web Layer Standards

- Run `make format-all` before commit (Go + JS/CSS)
- Handlers must be thin - business logic belongs in `internal/ldap/`
- All inputs must be validated and sanitized
- Use `zerolog` for logging, never `console.log` or `fmt.Println()`
- Templates use Templ syntax (`.templ` files compile to `*_templ.go`)
- TailwindCSS v4 for styling - no custom CSS unless necessary

### Security Requirements

- **CSRF protection**: Enabled on all state-changing operations
- **Cookie security**: `COOKIE_SECURE=true` for HTTPS environments
- **Session handling**: HTTPOnly, SameSite=Strict, regenerate after login
- **Input validation**: All user input validated before processing
- **Error responses**: Never leak sensitive data in error messages

## Security

### Critical Web Security Rules

- **CSRF**: All POST/PUT/DELETE endpoints require CSRF token validation
- **Sessions**: Regenerate session ID after authentication
- **Cookies**: Configure `COOKIE_SECURE=true` for HTTPS (see server.go:45-52)
- **Headers**: Set security headers (X-Content-Type-Options, X-Frame-Options, etc.)
- **Input validation**: Validate all request data before processing
- **Error handling**: Sanitize error messages to avoid leaking sensitive information

### Session Configuration

```go
// Secure session configuration (see createSessionStore in server.go)
session.Config{
    CookieSecure:   opts.CookieSecure,  // true for HTTPS
    CookieHTTPOnly: true,                // No JS access
    CookieSameSite: "Strict",            // CSRF protection
    Expiration:     opts.SessionDuration,
}
```

## PR & Commit Checklist

- [ ] Handlers are thin (business logic in `internal/ldap`)
- [ ] All inputs validated and sanitized
- [ ] Error responses don't leak sensitive data
- [ ] Tests cover happy path and error cases
- [ ] CSRF protection on state-changing endpoints
- [ ] Session handling follows security best practices
- [ ] Templates compiled (`pnpm templ:build`)
- [ ] CSS built and minified (`pnpm css:build:prod`)
- [ ] No console.log or debug prints in production code
- [ ] `make format-all` - all code formatted
- [ ] `make lint` - passes linters
- [ ] `make test` - tests pass with coverage

## Examples: Good vs Bad

### ✅ Good: Clean handler

```go
func (s *Server) handleListUsers(c *fiber.Ctx) error {
    filter := c.Query("filter", "")

    users, err := s.ldap.SearchUsers(filter)
    if err != nil {
        return s.handleError(c, 500, err)
    }

    return c.JSON(fiber.Map{
        "users": users,
        "count": len(users),
    })
}
```

### ❌ Bad: Mixed concerns

```go
// BAD: LDAP logic + HTML rendering in handler
func BadHandler(c *fiber.Ctx) error {
    conn, _ := ldap.Dial(...) // Business logic in handler
    result, _ := conn.Search(...)

    html := "<html><body>" // Manual HTML construction
    for _, entry := range result.Entries {
        html += "<div>" + entry.GetAttributeValue("cn") + "</div>"
    }
    html += "</body></html>"

    return c.SendString(html) // Use templates instead
}
```

### ✅ Good: Secure session handling

```go
// After successful login - regenerate session ID
sess, _ := store.Get(c)
sess.Regenerate()           // Prevent session fixation
sess.Set("user_id", user.ID)
sess.Save()
```

### ❌ Bad: Insecure session

```go
// Missing session regeneration - vulnerable to session fixation
sess.Set("user_id", user.ID)
sess.Save()
```

## When You're Stuck

1. **Routing**: Check `server.go` for route setup patterns
2. **Handlers**: Review existing handlers in `users.go`, `groups.go`, `auth.go`
3. **Templates**: See `templates/` for Templ component examples
4. **Middleware**: Look at `middleware.go` for auth/logging patterns
5. **Testing**: Check `handlers_test.go` and `cookie_security_test.go` for patterns
6. **Frontend**: Review `tailwind.config.js` and `package.json` scripts
7. **Assets not building**: Run `make clean && pnpm install && pnpm build:assets`
8. **CSRF errors**: Check `createCSRFConfig` in server.go for configuration

## House Rules

- **Thin handlers**: All business logic belongs in `internal/ldap/`, not in handlers
- **Security first**: CSRF on POST/PUT/DELETE, session regeneration after auth
- **Templ templates**: Never manually construct HTML - use `.templ` components
- **TailwindCSS only**: No custom CSS unless absolutely necessary
- **Structured logging**: Use `zerolog`, never `fmt.Println()` or `console.log`
- **Asset pipeline**: `pnpm build:assets` required before deployment
- **Cookie security**: Always configure `COOKIE_SECURE` based on HTTPS availability
