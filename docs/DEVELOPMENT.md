# LDAP Manager Development Guide

Comprehensive developer tooling and quality assurance guide for the LDAP Manager project.

## Quick Start

```bash
# Initial setup
make setup              # Install all dependencies and tools
make setup-hooks        # Install pre-commit hooks

# Development workflow
make dev               # Start development server
make check            # Run all quality checks
make build            # Build the application
```

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

# Install all dependencies and tools
make setup

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

## Available Make Targets

The `Makefile` provides comprehensive development automation:

### Setup & Dependencies

- `make setup` - Install Go deps, Node deps, and development tools
- `make setup-go` - Install Go dependencies only
- `make setup-node` - Install Node.js dependencies only
- `make setup-tools` - Install Go development tools
- `make setup-hooks` - Install pre-commit hooks

### Building

- `make build` - Build application binary with assets
- `make build-assets` - Build CSS and template assets only
- `make build-release` - Build optimized multi-platform binaries
- `make docker` - Build Docker image

### Testing & Quality

- `make test` - Run comprehensive test suite with coverage
- `make test-quick` - Run tests without coverage
- `make test-short` - Run tests without race detection
- `make benchmark` - Run performance benchmarks
- `make lint` - Run all linting and static analysis
- `make check` - Run lint + test (quality gate)

### Development

- `make dev` - Start development server with hot reload
- `make fix` - Auto-fix code formatting
- `make clean` - Remove build artifacts and caches
- `make serve` - Start built application

### Utilities

- `make help` - Show all available targets
- `make info` - Display build information
- `make deps` - Update all dependencies

## Development Workflow

### Daily Development

1. **Start Development Server**:

   ```bash
   make dev
   ```

   This starts concurrent processes:
   - Go server with live reload (Air)
   - CSS compilation with TailwindCSS watch mode
   - Template compilation with templ watch mode

2. **Run Quality Checks**:

   ```bash
   make check  # Run all linting and tests
   ```

3. **Fix Formatting Issues**:
   ```bash
   make fix    # Auto-fix Go formatting and imports
   ```

### Alternative Commands

You can also use PNPM commands directly:

```bash
# Development mode with hot reload
pnpm dev

# Production build
pnpm build

# Individual asset builds
pnpm css:build    # Build TailwindCSS
pnpm css:dev      # Watch CSS changes
pnpm templ:build  # Build templates
pnpm templ:dev    # Watch template changes
```

## Code Quality Tools

### Go Linting (golangci-lint)

Comprehensive linting with 30+ enabled linters:

```bash
# Run linting
make lint-go

# Individual linter categories
golangci-lint run --config .golangci.yml
```

**Key linters enabled:**

- **Error handling**: errcheck, wrapcheck
- **Code quality**: gosimple, ineffassign, unused
- **Security**: gosec (via CI)
- **Performance**: prealloc, maligned
- **Style**: goimports, gofumpt, whitespace

### Static Analysis Tools

Multiple static analysis tools for comprehensive code review:

```bash
# Security scanning
make lint-security      # govulncheck for vulnerabilities

# Code complexity
make lint-complexity    # gocyclo for cyclomatic complexity

# Formatting checks
make lint-format       # gofumpt + goimports validation
```

### Pre-commit Hooks

Automatic quality checks before commits:

```bash
# Install hooks
make setup-hooks

# Manual hook execution
pre-commit run --all-files
```

**Enabled hooks:**

- Go formatting (gofumpt)
- Import organization (goimports)
- Markdown linting (markdownlint)
- Secret detection (detect-secrets)
- YAML validation

## Testing Strategy

### Test Coverage Requirements

- **Minimum Coverage**: 80% (enforced by CI)
- **Coverage Reporting**: HTML reports in `coverage-reports/`
- **Benchmark Testing**: Performance regression detection

### Running Tests

```bash
# Comprehensive test suite with coverage
make test

# Quick tests without coverage
make test-quick

# Race condition detection
make test-short

# Performance benchmarks
make benchmark
```

### Test Organization

Tests are organized by package with clear naming conventions:

- `*_test.go` - Standard unit tests
- `*_integration_test.go` - Integration tests
- `benchmark_*_test.go` - Performance benchmarks

## Build System

### Asset Processing

The build system handles multiple asset types:

1. **CSS Processing**:
   - TailwindCSS compilation
   - PostCSS processing with autoprefixer
   - Production optimization with cssnano

2. **Template Processing**:
   - templ compilation to Go files
   - Type-safe HTML generation
   - Development watch mode

3. **Go Compilation**:
   - Cross-platform binary generation
   - Build info injection (version, commit, timestamp)
   - Optimized release builds

### Docker Support

```bash
# Build Docker image
make docker

# Run locally
make docker-run

# Production deployment
docker-compose up -d
```

## Architecture Patterns

### Project Structure

```
internal/
├── web/           # HTTP handlers and middleware
├── ldap_cache/    # LDAP connection and caching
├── options/       # Configuration management
└── build.go       # Build information

docs/              # Project documentation
scripts/           # Development and CI scripts
```

### Code Organization

- **Separation of Concerns**: Clear boundaries between web, LDAP, and configuration layers
- **Dependency Injection**: Configurable components for testing
- **Error Handling**: Consistent error wrapping and logging
- **Concurrent Safety**: Proper synchronization for shared resources

## Contributing Guidelines

### Code Standards

1. **Go Code Style**:
   - Follow effective Go patterns
   - Use gofumpt for formatting
   - Organize imports with goimports
   - Write descriptive function and variable names

2. **Commit Messages**:
   - Use conventional commits format
   - Include scope and type (feat, fix, docs, etc.)
   - Write clear, concise descriptions

3. **Pull Request Process**:
   - Create feature branches from main
   - Pass all quality checks (make check)
   - Include tests for new functionality
   - Update documentation as needed

### Development Tools

All required tools are automatically installed via:

```bash
make setup-tools
```

This installs:

- golangci-lint (comprehensive linting)
- staticcheck (static analysis)
- govulncheck (security scanning)
- gocyclo (complexity analysis)
- goimports (import formatting)
- gofumpt (code formatting)
- templ (template compilation)

## Troubleshooting

### Common Issues

1. **templ command not found**:

   ```bash
   go install github.com/a-h/templ/cmd/templ@latest
   ```

2. **PNPM not available**:

   ```bash
   npm install -g corepack
   corepack enable
   ```

3. **Permission issues with pre-commit**:

   ```bash
   pip install --user pre-commit
   make setup-hooks
   ```

4. **Coverage below threshold**:
   - Add tests for uncovered code paths
   - Check coverage report: `coverage-reports/coverage.html`

### Performance Monitoring

Monitor application performance during development:

```bash
# Run benchmarks
make benchmark

# Check for race conditions
go test -race ./...

# Profile CPU usage
go test -cpuprofile=cpu.prof -bench=.

# Profile memory usage
go test -memprofile=mem.prof -bench=.
```

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).
For API documentation, see [API.md](API.md).
For architecture details, see [architecture.md](architecture.md).
