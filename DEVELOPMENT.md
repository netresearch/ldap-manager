# Development Guide - LDAP Manager

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
# Setup (one-time)
make setup-hooks

# Manual execution
pre-commit run --all-files
```

**Hook categories:**
- Go formatting and imports
- Security scanning with detect-secrets
- Markdown linting
- Trailing whitespace and file endings
- Large file detection

## Development Environment

### direnv Integration (.envrc)

Automatic environment setup when entering the project directory:

```bash
# Required: Install direnv first
# Ubuntu/Debian: apt install direnv
# macOS: brew install direnv

# Allow the .envrc file
direnv allow

# Environment automatically loads with:
export CGO_ENABLED=0
export GOOS=linux  
export GOARCH=amd64
export PROJECT_ROOT="$(pwd)"
```

**Benefits:**
- Consistent build environment
- Automatic tool verification
- Development shortcuts display
- Project-specific settings

### Testing Infrastructure

Comprehensive testing with multiple execution modes:

```bash
# Full test suite with coverage
make test                    # Runs scripts/test.sh

# Quick testing modes  
make test-quick             # No coverage reporting
make test-short             # No race detection

# Performance testing
make benchmark              # Benchmark tests with memory profiling
```

**Coverage thresholds** (`.testcoverage.yml`):
- Total coverage: 80%
- File coverage: 70%  
- Package coverage: 75%

**Test outputs:**
- `coverage.out` - Coverage data
- `coverage.html` - HTML coverage report
- `benchmark-results.txt` - Performance benchmarks

## CI/CD Pipeline

### GitHub Actions Workflows

**Quality Assurance** (`.github/workflows/quality.yml`):
- **Linting**: golangci-lint, staticcheck, gosec with SARIF output
- **Testing**: Full test suite with OpenLDAP integration tests
- **Building**: Multi-platform binary verification  
- **Security**: Vulnerability scanning and Docker security
- **Dependencies**: Go and Node.js dependency auditing

**Docker Pipeline** (`.github/workflows/docker.yml`):
- Multi-architecture builds (amd64, arm64, arm/v7)
- Container Registry (GHCR) publishing
- Trivy security scanning
- Automated tagging and metadata

### Quality Gates

The pipeline enforces quality standards:

```yaml
# Critical checks (must pass)
- Linting and static analysis
- Test suite execution  
- Security vulnerability scans

# Warning checks (logged but don't fail)  
- Docker security recommendations
- Dependency audit findings
```

## Security Practices

### Secret Detection

Automated secret scanning with `detect-secrets`:

```bash
# Scan for secrets
detect-secrets scan --baseline .secrets.baseline

# Update baseline after review
detect-secrets scan --update .secrets.baseline
```

### Vulnerability Management

Regular security scanning:

```bash
# Go module vulnerabilities
govulncheck ./...

# Node.js dependencies  
pnpm audit --audit-level moderate

# Docker image security (CI only)
trivy image ldap-manager:latest
```

## Performance Monitoring

### Benchmarking

Regular performance testing:

```bash
# Run benchmarks
make benchmark

# Profile memory usage
go test -bench=. -benchmem -memprofile=mem.prof ./...

# CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./...
```

### Build Optimization

Optimized builds with size and performance focus:

```bash
# Development build
make build

# Release build (optimized)
make build-release

# Build flags used:
-ldflags="-s -w"        # Strip debug info
-trimpath              # Remove build paths  
CGO_ENABLED=0          # Static linking
```

## Troubleshooting

### Common Issues

**golangci-lint failures:**
```bash
# Update to latest version
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Clear cache
golangci-lint cache clean
```

**Pre-commit hook failures:**
```bash
# Update hooks
pre-commit autoupdate

# Skip hooks temporarily (not recommended)
git commit --no-verify
```

**Docker build issues:**
```bash
# Check Docker is running
docker info

# Clear build cache
docker builder prune
```

### Development Tools Verification

The `.envrc` file includes tool verification. If tools are missing:

```bash
# Install missing Go tools
make setup-tools

# Install Node.js dependencies
make setup-node

# Verify installation
make info
```

## Contributing

### Code Standards

- **Go version**: 1.25.1+
- **Code style**: Enforced by gofumpt and goimports
- **Test coverage**: Minimum 80% overall
- **Documentation**: All public functions documented
- **Security**: No secrets in code, use environment variables

### Development Workflow

1. **Setup**: `make setup && make setup-hooks`
2. **Branch**: Create feature branch from main
3. **Develop**: Use `make dev` for hot reload
4. **Quality**: Run `make check` before commit
5. **Test**: Ensure `make test` passes
6. **Commit**: Pre-commit hooks auto-run
7. **PR**: GitHub Actions validate changes

### Release Process

1. **Quality Gate**: All CI checks pass
2. **Versioning**: Semantic versioning (git tags)
3. **Builds**: Multi-platform binaries generated
4. **Docker**: Images published to GHCR
5. **Security**: Vulnerability scans completed

---

## Environment Setup (Original Development Guide)

### Prerequisites

- Docker and Docker Compose installed
- Git for version control
- Node.js 22+ and pnpm (for frontend development)
- Go 1.25.1+ (for backend development)

### Docker Compose Setup

The easiest way to get started is using the provided Docker Compose configuration:

```bash
# Clone the repository
git clone https://github.com/netresearch/ldap-manager.git
cd ldap-manager

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f
```

This will start:
- **OpenLDAP server** on port 389 (LDAP) and 636 (LDAPS)
- **phpLDAPadmin** on port 8080 for LDAP management
- **LDAP Manager** on port 3000

## Services Overview

### OpenLDAP Server
- **URL**: `ldap://localhost:389`
- **Base DN**: `dc=netresearch,dc=local`
- **Admin DN**: `cn=admin,dc=netresearch,dc=local`
- **Admin Password**: `admin`
- **Web Management**: http://localhost:8080 (phpLDAPadmin)

### LDAP Manager Application
- **URL**: http://localhost:3000
- **Session Storage**: BBolt database (`/data/session.bbolt`)
- **Log Level**: Debug (for development)
- **Health Endpoints**: 
  - `/health` - Comprehensive cache metrics and health status
  - `/health/ready` - Readiness check with cache warming status
  - `/health/live` - Simple liveness check

## Local Development

For active development, you might prefer running the application locally:

### 1. Start OpenLDAP Only

```bash
docker-compose up -d openldap phpldapadmin
```

### 2. Install Dependencies

```bash
# Install frontend dependencies
pnpm install

# Download Go dependencies
go mod download
```

### 3. Build Assets

```bash
# Build CSS
pnpm css:build

# Generate templates
pnpm templ:build
```

### 4. Run Application Locally

```bash
# Set environment variables
export LDAP_HOST=localhost
export LDAP_PORT=389
export LDAP_BASE_DN="dc=netresearch,dc=local"
export LDAP_BIND_DN="cn=admin,dc=netresearch,dc=local"
export LDAP_BIND_PASSWORD="admin"
export LDAP_USE_TLS=false
export SESSION_PATH="./session.bbolt"
export SESSION_SECRET="dev-secret"
export LOG_LEVEL=debug

# Run the application
go run .
```

### 5. Development with Hot Reload

```bash
# Start development mode with hot reload
pnpm dev
```

This runs:
- CSS watcher (rebuilds on Tailwind changes)
- Template watcher (regenerates on .templ changes)
- Go application with restart on changes

## Testing LDAP Operations

### Add Test Users and Groups

Use phpLDAPadmin (http://localhost:8080) or LDAP commands:

```bash
# Add organizational units
ldapadd -x -H ldap://localhost:389 -D "cn=admin,dc=netresearch,dc=local" -w admin <<EOF
dn: ou=users,dc=netresearch,dc=local
objectClass: organizationalUnit
ou: users

dn: ou=groups,dc=netresearch,dc=local
objectClass: organizationalUnit
ou: groups
EOF

# Add a test user
ldapadd -x -H ldap://localhost:389 -D "cn=admin,dc=netresearch,dc=local" -w admin <<EOF
dn: cn=testuser,ou=users,dc=netresearch,dc=local
objectClass: inetOrgPerson
objectClass: posixAccount
objectClass: shadowAccount
cn: testuser
sn: User
givenName: Test
displayName: Test User
uidNumber: 1001
gidNumber: 1001
homeDirectory: /home/testuser
loginShell: /bin/bash
userPassword: {SSHA}password123
EOF
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LDAP_HOST` | `localhost` | LDAP server hostname |
| `LDAP_PORT` | `389` | LDAP server port |
| `LDAP_BASE_DN` | `dc=netresearch,dc=local` | LDAP base DN |
| `LDAP_BIND_DN` | `cn=admin,dc=netresearch,dc=local` | LDAP bind DN |
| `LDAP_BIND_PASSWORD` | `admin` | LDAP bind password |
| `LDAP_USE_TLS` | `false` | Enable TLS/SSL |
| `SESSION_PATH` | `./session.bbolt` | BBolt session database path |
| `SESSION_SECRET` | `dev-secret` | Session encryption secret |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `PORT` | `3000` | Application port |

## Troubleshooting

### Container Issues

```bash
# Check container status
docker-compose ps

# View logs for specific service
docker-compose logs openldap
docker-compose logs ldap-manager

# Restart services
docker-compose restart

# Clean up and restart
docker-compose down -v
docker-compose up -d
```

### LDAP Connection Issues

1. Verify OpenLDAP is running: `docker-compose ps`
2. Test LDAP connection: `ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=netresearch,dc=local" -w admin -b "dc=netresearch,dc=local"`
3. Check firewall settings for ports 389 and 3000

### Session Database Issues

The application uses BBolt for session storage. If you encounter session-related issues:

```bash
# Remove session database to start fresh
rm -f session.bbolt

# Or in Docker
docker-compose down -v  # This removes all volumes
```

## Security Notes for Development

- **Default passwords**: Change all default passwords for production use
- **Session secret**: Use a strong, random session secret in production
- **TLS/SSL**: Enable TLS for production LDAP connections
- **Firewall**: Restrict LDAP port access in production environments
- **Session storage**: The BBolt database contains session data - protect it appropriately

## Contributing

1. Make changes to the codebase
2. Test with the local development environment
3. Ensure all tests pass: `go test ./...`
4. Build and test with Docker: `docker-compose build && docker-compose up -d`
5. Submit pull request

## Additional Resources

- [LDAP Manager Documentation](README.md)
- [OpenLDAP Documentation](https://www.openldap.org/doc/)
- [phpLDAPadmin Documentation](http://phpldapadmin.sourceforge.net/wiki/index.php/Main_Page)
- [BBolt Documentation](https://pkg.go.dev/go.etcd.io/bbolt)