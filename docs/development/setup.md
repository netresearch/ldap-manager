# Development Setup

Complete guide for setting up a local development environment for LDAP Manager.

## Prerequisites

### Required Software

Install these tools before beginning development:

#### Go 1.23+

```bash
# Check current Go version
go version

# Should show 1.23.0 or higher
```

**Installation:**
- **Linux/macOS**: Use official installer from https://golang.org/dl/
- **Windows**: Download and run MSI installer
- **Package Managers**: 
  - Ubuntu: `sudo apt install golang-1.23`
  - macOS: `brew install go`

#### Node.js v16+

```bash
# Check current Node.js version
node --version

# Should show v16.0.0 or higher
```

**Installation:**
- **Official**: Download from https://nodejs.org/
- **nvm (recommended)**: 
  ```bash
  curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
  nvm install 16
  nvm use 16
  ```

#### Package Management

**Corepack for PNPM:**
```bash
# Enable corepack (comes with Node.js 16+)
npm install -g corepack
corepack enable

# Verify PNPM is available
pnpm --version
```

#### templ CLI

Required for type-safe HTML template compilation:

```bash
# Install templ
go install github.com/a-h/templ/cmd/templ@latest

# Verify installation
templ --version
```

### Optional Development Tools

These tools enhance the development experience:

```bash
# Air for hot reloading
go install github.com/cosmtrek/air@latest

# pre-commit for git hooks
pip install pre-commit
```

## Project Setup

### Clone and Initialize

```bash
# Clone the repository
git clone https://github.com/netresearch/ldap-manager.git
cd ldap-manager

# Install all dependencies and development tools
make setup

# This runs:
# - make setup-go      (Go dependencies)
# - make setup-node    (Node.js dependencies)  
# - make setup-tools   (Development tools)
```

### Development Configuration

Create your local environment configuration:

```bash
# Copy example configuration
cp .env.example .env.local

# Edit with your LDAP settings
nano .env.local
```

**Example `.env.local` for development:**

```bash
# LDAP Configuration
LDAP_SERVER=ldap://localhost:389
LDAP_BASE_DN=DC=dev,DC=local
LDAP_READONLY_USER=cn=admin,dc=dev,dc=local
LDAP_READONLY_PASSWORD=admin
LDAP_IS_AD=false

# Development Settings
LOG_LEVEL=debug
PERSIST_SESSIONS=true
SESSION_PATH=dev-session.bbolt
SESSION_DURATION=8h

# Optional: Custom listen address
LISTEN_ADDR=:3000
```

### Git Hooks Setup

Install pre-commit hooks for code quality:

```bash
make setup-hooks

# This installs hooks for:
# - Go formatting (gofumpt)
# - Import organization (goimports)
# - Markdown linting
# - Secret detection
```

## Development Workflow

### Start Development Server

The recommended way to start development:

```bash
# Start development server with hot reload
make dev
```

This starts multiple concurrent processes:
- **Go server** with Air hot reloading
- **CSS compilation** with TailwindCSS watch mode  
- **Template compilation** with templ watch mode

**Alternative PNPM commands:**

```bash
# Development with hot reload
pnpm dev

# Individual processes
pnpm css:dev      # Watch CSS changes
pnpm templ:dev    # Watch template changes
pnpm go:dev       # Go server with hot reload
```

### Development Server Features

When `make dev` is running:

- **Automatic Rebuilds**: Changes to Go, CSS, or templates trigger rebuilds
- **Browser Refresh**: Assets are automatically rebuilt and served
- **Debug Logging**: Verbose logging enabled by default
- **Long Sessions**: 8-hour session duration for convenience

### Build Commands

#### Quick Build

```bash
# Build application with assets
make build

# This creates:
# - ./ldap-manager (executable binary)
# - Compiled CSS in internal/web/static/
# - Compiled templates as Go files
```

#### Asset-Only Builds

```bash
# Build only CSS and templates
make build-assets

# Individual asset builds
pnpm css:build    # Build TailwindCSS
pnpm templ:build  # Build templ templates
```

#### Clean Build

```bash
# Remove all build artifacts
make clean

# Clean specific artifacts
pnpm clean:css    # Remove CSS builds
pnpm clean:templ  # Remove template builds
```

## Quality Assurance

### Running Tests

```bash
# Run complete test suite with coverage
make test

# Quick tests without coverage
make test-quick

# Tests without race detection (faster)
make test-short

# Performance benchmarks
make benchmark
```

**Test Output:**
- Coverage reports in `coverage-reports/coverage.html`
- Test results with pass/fail status
- Benchmark performance metrics

### Code Quality Checks

```bash
# Run all quality checks (linting + tests)
make check

# Individual quality tools
make lint         # All linters
make lint-go      # Go-specific linters
make lint-format  # Format checking
make lint-security # Security scanning
```

**Linting Tools Used:**
- **golangci-lint**: Comprehensive Go linting (30+ linters)
- **govulncheck**: Security vulnerability scanning
- **gocyclo**: Cyclomatic complexity analysis
- **staticcheck**: Advanced static analysis

### Auto-fixing

```bash
# Automatically fix code formatting
make fix

# This runs:
# - gofumpt (Go formatting)
# - goimports (import organization)
# - Prettier (CSS/JS formatting)
```

## Project Structure

### Directory Organization

```
ldap-manager/
├── cmd/                    # Application entry points
├── internal/               # Private application code
│   ├── web/               # HTTP handlers and middleware  
│   ├── ldap_cache/        # LDAP connection and caching
│   ├── options/           # Configuration management
│   └── build.go           # Build information injection
├── docs/                  # Documentation
├── scripts/               # Build and utility scripts
├── coverage-reports/      # Test coverage output
└── dist/                  # Release binaries
```

### Key Files

- **`Makefile`**: Development automation (40+ targets)
- **`.air.toml`**: Hot reload configuration
- **`.golangci.yml`**: Linter configuration
- **`package.json`**: Node.js dependencies and scripts
- **`tailwind.config.js`**: TailwindCSS configuration
- **`.env.example`**: Configuration template

## Development Tools

### Make Targets

The `Makefile` provides comprehensive automation:

#### Setup & Dependencies
- `make setup` - Complete development environment setup
- `make setup-go` - Go dependencies only
- `make setup-node` - Node.js dependencies only
- `make setup-tools` - Development tools installation

#### Building & Assets
- `make build` - Build application with assets
- `make build-assets` - Build CSS and templates only
- `make build-release` - Multi-platform release builds

#### Testing & Quality
- `make test` - Full test suite with coverage
- `make benchmark` - Performance benchmarks
- `make lint` - All linting tools
- `make check` - Quality gate (lint + test)

#### Development
- `make dev` - Development server with hot reload
- `make serve` - Run built application
- `make clean` - Remove build artifacts

#### Utilities
- `make help` - List all available targets
- `make info` - Show build environment information

### IDE Configuration

#### Visual Studio Code

Recommended extensions:

```json
{
  "recommendations": [
    "golang.go",
    "a-h.templ",
    "bradlc.vscode-tailwindcss",
    "esbenp.prettier-vscode"
  ]
}
```

**Settings** (`.vscode/settings.json`):

```json
{
  "go.toolsManagement.checkForUpdates": "local",
  "go.useLanguageServer": true,
  "go.formatTool": "gofumpt",
  "go.lintTool": "golangci-lint",
  "tailwindCSS.includeLanguages": {
    "templ": "html"
  },
  "[templ]": {
    "editor.defaultFormatter": "a-h.templ"
  }
}
```

#### GoLand/IntelliJ

1. Install Go plugin
2. Configure Go SDK (1.23+)
3. Set gofumpt as formatter
4. Enable golangci-lint integration

### Debugging

#### VS Code Debug Configuration

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch LDAP Manager",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/ldap-manager",
      "env": {
        "LOG_LEVEL": "debug"
      },
      "args": []
    }
  ]
}
```

#### Command-Line Debugging

```bash
# Debug with delve
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug cmd/ldap-manager/main.go

# Debug tests
go test -v ./internal/web -run TestSpecificFunction
```

## Testing Strategy

### Test Organization

Tests are organized by package:

```
internal/
├── web/
│   ├── handlers.go
│   ├── handlers_test.go     # Unit tests
│   └── integration_test.go  # Integration tests
├── ldap_cache/
│   ├── manager.go
│   ├── manager_test.go
│   └── benchmark_test.go    # Performance tests
└── options/
    ├── options.go
    └── options_test.go
```

### Test Categories

#### Unit Tests
- Test individual functions and methods
- Use mocking for external dependencies
- Fast execution, no external services

#### Integration Tests
- Test component interactions
- May use test LDAP server
- Slower execution, more realistic scenarios

#### Benchmark Tests
- Performance regression detection
- Memory allocation profiling
- Execution time measurement

### Coverage Requirements

- **Minimum**: 80% coverage (enforced by CI)
- **Target**: 90%+ for critical paths
- **Reports**: HTML coverage reports generated

### Mock LDAP Server

For integration testing without external dependencies:

```bash
# Start test LDAP server (Docker)
docker run -d --name test-ldap \
  -p 389:389 \
  -e LDAP_ORGANISATION="Test Org" \
  -e LDAP_DOMAIN="test.local" \
  -e LDAP_ADMIN_PASSWORD="admin" \
  osixia/openldap:latest

# Configure tests to use local LDAP
export LDAP_SERVER=ldap://localhost:389
export LDAP_BASE_DN=dc=test,dc=local
export LDAP_READONLY_USER=cn=admin,dc=test,dc=local
export LDAP_READONLY_PASSWORD=admin
```

## Performance Optimization

### Build Performance

```bash
# Parallel builds
make -j$(nproc) build

# Cache Go builds
export GOCACHE=$HOME/.cache/go-build
export GOMODCACHE=$HOME/go/pkg/mod
```

### Development Performance

```bash
# Skip slower checks during development
make test-quick    # Skip coverage
make test-short    # Skip race detection

# Selective testing
go test ./internal/web -run TestHandlers
```

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Memory profiling  
go test -memprofile=mem.prof -bench=.

# View profiles
go tool pprof cpu.prof
```

## Common Development Issues

### templ Command Not Found

```bash
# Reinstall templ
go install github.com/a-h/templ/cmd/templ@latest

# Add Go bin to PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### PNPM Not Available

```bash
# Enable corepack
npm install -g corepack
corepack enable

# Or install PNPM directly
npm install -g pnpm
```

### Build Failures

```bash
# Clean and rebuild
make clean
make setup
make build

# Check Go version
go version  # Should be 1.23+

# Check Node version
node --version  # Should be 16+
```

### Test Failures

```bash
# Run specific test with verbose output
go test -v ./internal/web -run TestFailingTest

# Check for race conditions
go test -race ./...

# Update test dependencies
go mod tidy
```

### LDAP Connection Issues

```bash
# Test LDAP connectivity
ldapsearch -H ldap://localhost:389 -x -s base

# Check Docker LDAP server
docker logs test-ldap

# Verify configuration
echo $LDAP_SERVER $LDAP_BASE_DN
```

## Next Steps

1. **Explore the Codebase**: Start with `internal/web/handlers.go`
2. **Make Your First Change**: Try modifying a template in `internal/web/`
3. **Run Tests**: Ensure your changes don't break existing functionality
4. **Review Architecture**: See [Architecture Guide](architecture.md)
5. **Contribute**: Follow [Contributing Guidelines](contributing.md)

## Additional Resources

- [Configuration Reference](../user-guide/configuration.md)
- [API Documentation](../user-guide/api.md)
- [Deployment Guide](../operations/deployment.md)
- [Contributing Guidelines](contributing.md)