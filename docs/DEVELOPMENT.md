# LDAP Manager Development Guide

## Overview

LDAP Manager uses a modern Go web stack with type-safe HTML templates, TailwindCSS for styling, and concurrent development workflows. This guide covers the development environment, architecture patterns, and contribution guidelines.

## Technology Stack

### Core Technologies

- **Backend**: Go 1.23+ with Fiber v2 web framework
- **Templates**: templ - Type-safe Go HTML templates
- **Styling**: TailwindCSS v4 with PostCSS processing
- **LDAP**: simple-ldap-go library for directory operations
- **Sessions**: Configurable storage (Memory or BBolt database)
- **Logging**: Zerolog structured logging

### Development Tools

- **Package Management**: PNPM with workspace configuration
- **Build System**: Concurrent asset processing with nodemon
- **Hot Reload**: Automatic rebuilds for Go, CSS, and templates
- **Formatting**: Prettier with Go template support
- **Containerization**: Docker with multi-stage builds

## Development Environment Setup

### Prerequisites

Install the required development tools:

```bash
# Go 1.23+ with module support
go version  # Should show 1.23 or higher

# Node.js v16+ with corepack for PNPM
node --version  # Should show v16 or higher
npm install -g corepack
corepack enable

# templ for type-safe HTML templates
go install github.com/a-h/templ/cmd/templ@latest

# Verify templ installation
templ --version
```

### Project Initialization

```bash
# Clone and setup dependencies
git clone <repository-url>
cd ldap-manager

# Install Node.js dependencies
pnpm install

# Create development configuration
cp .env.example .env.local
# Edit .env.local with your LDAP settings
```

### Development Configuration

Create `.env.local` with your development LDAP settings:

```bash
# Development LDAP configuration
LDAP_SERVER=ldap://your-dev-ldap:389
LDAP_BASE_DN=DC=dev,DC=local
LDAP_READONLY_USER=readonly
LDAP_READONLY_PASSWORD=devpassword
LDAP_IS_AD=false

# Development settings
LOG_LEVEL=debug
PERSIST_SESSIONS=true
SESSION_PATH=dev-session.bbolt
SESSION_DURATION=2h
```

## Development Workflow

### Available Commands

The project uses PNPM scripts for development workflow:

```bash
# Development mode with hot reload
pnpm dev

# Production build
pnpm build

# Start production build
pnpm start

# Individual asset builds
pnpm css:build    # Build TailwindCSS
pnpm css:dev      # Watch CSS changes
pnpm templ:build  # Generate Go template files
pnpm templ:dev    # Watch template changes
```

### Hot Reload Development

The `pnpm dev` command starts three concurrent processes:

1. **CSS Watcher**: Rebuilds TailwindCSS on style changes
2. **Template Watcher**: Regenerates Go templates on .templ changes
3. **Go Server**: Restarts application on Go code changes

```bash
pnpm dev
# Output shows three concurrent processes:
# [css] Rebuilding CSS...
# [templ] Generating templates...
# [go] Starting Go server...
```

### File Watching Behavior

- **CSS Changes**: `internal/web/tailwind.css` → Auto-rebuild styles
- **Template Changes**: `**/*.templ` → Regenerate Go template files
- **Go Changes**: `**/*.go` → Restart application server
- **Static Assets**: Manual refresh required

## Project Architecture

### Directory Structure

```
ldap-manager/
├── main.go                     # Application entry point
├── internal/
│   ├── build.go               # Build information
│   ├── options/
│   │   └── app.go            # CLI/ENV configuration parsing
│   ├── ldap_cache/           # LDAP data caching layer
│   │   ├── manager.go        # Cache management and refresh
│   │   └── cache.go          # Generic concurrent-safe cache
│   └── web/                  # Web application layer
│       ├── server.go         # Fiber app setup and routing
│       ├── auth.go           # Authentication handlers
│       ├── users.go          # User management handlers
│       ├── groups.go         # Group management handlers
│       ├── computers.go      # Computer management handlers
│       ├── templates/        # templ template files
│       │   ├── base.templ    # Base HTML layout
│       │   ├── flash.go      # Flash message system
│       │   ├── *.templ       # Page-specific templates
│       │   └── *_templ.go    # Generated Go template code
│       └── static/           # Static assets and embedded files
├── docs/                     # Documentation
├── package.json             # PNPM configuration
├── tailwind.config.js       # TailwindCSS configuration
├── postcss.config.mjs       # PostCSS processing
└── Dockerfile              # Container build configuration
```

### Architecture Layers

#### 1. Main Application (`main.go`)

- Application entry point with logging setup
- Configuration parsing and validation
- Web server initialization and startup

#### 2. Configuration Layer (`internal/options/`)

- CLI flag and environment variable parsing
- Configuration validation and defaults
- Support for `.env` file loading with godotenv

#### 3. LDAP Caching Layer (`internal/ldap_cache/`)

- **Manager**: Coordinates cache refresh and LDAP operations
- **Cache**: Generic thread-safe storage for LDAP entities
- **Background Refresh**: Automatic 30-second cache updates
- **Relationship Population**: Builds full objects with dependencies

#### 4. Web Layer (`internal/web/`)

- **Server**: Fiber application setup with middleware
- **Handlers**: HTTP request processing for each resource type
- **Templates**: Type-safe HTML generation with templ
- **Session Management**: Authentication and session handling

### Template System (templ)

#### Template Architecture

LDAP Manager uses [templ](https://templ.guide/) for type-safe HTML templates:

```go
// templates/base.templ - Base layout
templ base(title string) {
    <!DOCTYPE html>
    <html>
        <head><title>{title} - LDAP Manager</title></head>
        <body>{children...}</body>
    </html>
}

// templates/users.templ - User listing
templ Users(users []ldap.User) {
    @base("Users") {
        <div class="container">
            for _, user := range users {
                <div class="user-card">{user.Name}</div>
            }
        </div>
    }
}
```

#### Template Generation

Templates are compiled to Go code during build:

```bash
# Generate Go template files
templ generate

# Files created:
# templates/base_templ.go
# templates/users_templ.go
# (etc...)
```

#### Template Best Practices

- **Type Safety**: Use strongly-typed parameters for all templates
- **Composition**: Build complex pages with component templates
- **CSS Classes**: Use TailwindCSS utility classes for styling
- **Flash Messages**: Use the flash system for user feedback
- **Conditional Rendering**: Leverage Go's template conditionals

### Flash Message System

The application includes a type-safe flash message system:

```go
// Flash message types
type FlashType string
const (
    FlashTypeSuccess FlashType = "success"
    FlashTypeError   FlashType = "error"
    FlashTypeInfo    FlashType = "info"
)

// Usage in handlers
flashes := templates.Flashes(
    templates.SuccessFlash("User updated successfully"),
    templates.ErrorFlash("Invalid password format"),
)
```

### Styling System (TailwindCSS)

#### CSS Architecture

- **Base Styles**: `internal/web/tailwind.css` - TailwindCSS directives
- **Generated CSS**: `internal/web/static/styles.css` - Compiled output
- **Dark Theme**: Default dark theme with customizable colors
- **Responsive**: Mobile-first responsive design patterns

#### TailwindCSS Configuration

```javascript
// tailwind.config.js
module.exports = {
  content: ["./internal/**/*.{templ,go}"],
  theme: {
    extend: {
      colors: {
        // Custom color palette
      }
    }
  },
  plugins: [require("@tailwindcss/forms")]
};
```

#### CSS Development Workflow

```bash
# Watch for changes during development
pnpm css:dev

# Manual build for production
pnpm css:build

# Output: internal/web/static/styles.css
```

## Development Patterns

### Handler Pattern

All web handlers follow a consistent pattern:

```go
func (a *App) userHandler(c *fiber.Ctx) error {
    // 1. Session validation
    sess, err := a.sessionStore.Get(c)
    if err != nil {
        return handle500(c, err)
    }

    // 2. Authentication check
    if sess.Fresh() {
        return c.Redirect("/login")
    }

    // 3. Data retrieval
    userDN := c.Params("userDN")
    user, err := a.ldapCache.FindUserByDN(userDN)
    if err != nil {
        return handle500(c, err)
    }

    // 4. Template rendering
    c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
    return templates.User(user).Render(c.UserContext(), c.Response().BodyWriter())
}
```

### LDAP Cache Pattern

The caching layer provides consistent data access:

```go
// Cache initialization
manager := ldap_cache.New(ldapClient)
go manager.Run() // Start background refresh

// Data access
users := manager.FindUsers(showDisabled)
user, err := manager.FindUserByDN(userDN)
fullUser := manager.PopulateGroupsForUser(user)

// Cache maintenance (automatic)
// - Refreshes every 30 seconds
// - Thread-safe concurrent access
// - Relationship population on demand
```

### Error Handling Pattern

Consistent error handling across the application:

```go
// Global error handler
func handle500(c *fiber.Ctx, err error) error {
    log.Error().Err(err).Send()
    c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
    return templates.FiveHundred(err).Render(c.UserContext(), c.Response().BodyWriter())
}

// Usage in handlers
if err != nil {
    return handle500(c, err)
}
```

## Testing Strategy

### Unit Testing

```bash
# Run Go tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/ldap_cache/
```

### Integration Testing

- Test LDAP connectivity with development directory
- Validate session management with different storage backends
- Verify template rendering with sample data

### Manual Testing Checklist

- [ ] Login/logout functionality
- [ ] User listing and detail views
- [ ] Group management operations
- [ ] Computer object access
- [ ] Session persistence across restarts
- [ ] Error handling and flash messages
- [ ] Responsive design on mobile

## Code Quality Standards

### Go Code Standards

- Follow standard Go formatting with `gofmt`
- Use meaningful variable and function names
- Add package-level documentation for public APIs
- Implement proper error handling with context
- Use structured logging with zerolog

### Template Standards

- Use semantic HTML5 elements
- Apply TailwindCSS utility classes consistently
- Implement proper accessibility attributes
- Handle empty states and error conditions
- Use templ's type safety features

### CSS Standards

- Use TailwindCSS utilities over custom CSS
- Follow mobile-first responsive patterns
- Maintain consistent spacing and typography
- Use semantic color names in theme configuration

### Commit Standards

The project uses [Conventional Commits](https://www.conventionalcommits.org/):

```bash
# Feature additions
feat: add user search functionality
feat(auth): implement session timeout warnings

# Bug fixes
fix: resolve LDAP connection timeout issues
fix(ui): correct mobile navigation layout

# Documentation
docs: update API documentation
docs(config): add environment variable examples

# Refactoring
refactor: simplify cache refresh logic
refactor(templates): extract common components

# Maintenance
chore: update dependencies
chore(ci): improve build performance
```

## Build and Deployment

### Local Build

```bash
# Full production build
pnpm build

# Creates:
# - internal/web/static/styles.css (compiled CSS)
# - internal/web/templates/*_templ.go (generated templates)
# - ldap-manager (Go binary)
```

### Docker Build

```bash
# Build container image
docker build -t ldap-manager .

# Multi-stage build process:
# 1. Node.js stage: Install dependencies and build assets
# 2. Go stage: Build application binary
# 3. Runtime stage: Minimal Alpine-based final image
```

### Production Deployment

- Use environment variables for configuration
- Enable HTTPS with reverse proxy (nginx, Traefik)
- Configure session persistence for high availability
- Monitor LDAP connection health
- Set appropriate log levels for performance

## Contributing Guidelines

### Development Process

1. **Fork** the repository and create a feature branch
2. **Setup** development environment with required tools
3. **Implement** changes following code quality standards
4. **Test** functionality with manual and automated tests
5. **Document** changes in relevant documentation files
6. **Commit** using conventional commit format
7. **Submit** pull request with clear description

### Pull Request Requirements

- [ ] Code follows project formatting standards
- [ ] Templates compile without errors (`templ generate`)
- [ ] CSS builds successfully (`pnpm css:build`)
- [ ] Go code compiles and tests pass (`go test ./...`)
- [ ] Documentation updated for user-facing changes
- [ ] Commit messages follow conventional format

### Code Review Process

- Maintainers review for code quality and security
- Template safety and accessibility considerations
- Performance impact on LDAP operations
- Backward compatibility with existing configurations

## Troubleshooting

### Common Development Issues

**Template Compilation Errors**

```bash
# Ensure templ is installed and in PATH
templ version

# Regenerate all templates
templ generate

# Check for syntax errors in .templ files
templ fmt --verify internal/web/templates/
```

**CSS Build Failures**

```bash
# Check TailwindCSS configuration
pnpm css:build --verbose

# Verify PostCSS configuration
cat postcss.config.mjs

# Clear CSS cache
rm internal/web/static/styles.css
pnpm css:build
```

**Hot Reload Not Working**

```bash
# Restart development server
pnpm dev

# Check file watchers
# On Linux: may need to increase inotify limits
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

**LDAP Connection Issues**

- Verify `.env.local` configuration matches your LDAP server
- Test LDAP connectivity with command-line tools
- Check firewall rules for LDAP ports (389/636)
- Enable debug logging: `LOG_LEVEL=debug`

For additional help, check the project issues or create a new issue with:

- Go and Node.js versions (`go version`, `node --version`)
- Operating system details
- Complete error messages and stack traces
- Steps to reproduce the issue
