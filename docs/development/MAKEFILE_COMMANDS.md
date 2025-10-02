# Makefile Commands Reference

Quick reference for all available `make` commands in the LDAP Manager project.

## 📋 Quick Start

```bash
make help          # Show all available commands
make setup         # Install all dependencies
make up            # Start development environment
make logs-app      # View application logs
make down          # Stop all services
```

---

## 🚀 Application Control

### Service Management
```bash
make up            # Start all services (LDAP + app)
make down          # Stop all services
make restart       # Restart all services
make start         # Start services without rebuilding
make stop          # Stop services without removing containers
make ps            # Show running services
```

### Container Access
```bash
make shell-app     # Open shell in app container
make shell-ldap    # Open shell in LDAP container
make logs          # Show logs from all services
make logs-app      # Show logs from app only
make logs-ldap     # Show logs from LDAP server
```

### Rebuild & Reset
```bash
make rebuild       # Rebuild and restart services
make fresh         # Clean everything and start fresh
```

---

## 🛠️ Development Workflow

### Asset Building
```bash
make watch         # Watch and rebuild assets on change
make css           # Build CSS only
make css-watch     # Watch and rebuild CSS
make templates     # Generate Go templates from .templ files
make templates-watch  # Watch and regenerate templates
make build-assets  # Build all assets (CSS + templates)
```

### Code Formatting
```bash
make format-go     # Format Go code (gofumpt + goimports)
make format-js     # Format JavaScript/JSON/CSS (prettier)
make format-all    # Format all code
make fix           # Alias for format-all
```

### Development Server
```bash
make dev           # Start dev server with hot reload (pnpm)
make serve         # Start built binary
```

---

## 🏗️ Building

### Local Build
```bash
make build         # Build application binary
make build-assets  # Build CSS and templates only
```

### Release Build
```bash
make build-release # Build optimized binaries for:
                   # - Linux (amd64)
                   # - macOS (amd64)
                   # - Windows (amd64)
```

### Docker Build
```bash
make docker        # Build production Docker image
make docker-run    # Build and run Docker container
make docker-dev-build  # Build development container
make docker-dev    # Start development environment
```

---

## ✅ Testing & Quality

### Testing
```bash
make test          # Run comprehensive test suite with coverage
make test-quick    # Run tests without coverage
make test-short    # Run tests without race detection
make test-race     # Run race detection tests
make benchmark     # Run performance benchmarks
```

### Docker Testing
```bash
make docker-test   # Run tests in container
make docker-lint   # Run linter in container
make docker-check  # Run all quality checks in container
```

### Linting
```bash
make lint          # Run all linting and static analysis
make lint-go       # Run Go linting (golangci-lint)
make lint-security # Run security vulnerability checks
make lint-format   # Check code formatting
make lint-complexity  # Check code complexity
```

### Quality Gates
```bash
make check         # Run all quality checks (lint + test)
make check-all     # Alias for check
make release       # Prepare release (check + build-release)
```

---

## 💾 Database & LDAP Management

### LDAP Operations
```bash
make ldap-reset    # Reset LDAP database (with confirmation)
make ldap-admin    # Open phpLDAPadmin in browser
```

### Session Management
```bash
make sessions-clean  # Clean session database files
```

---

## 🔍 Monitoring & Debugging

### Health Checks
```bash
make health        # Check LDAP and app health
make stats         # Show container resource usage
make inspect       # View container environment
```

### Debugging
```bash
make debug         # Start app in debug mode
make docker-shell  # Open shell in development container
```

---

## 🌐 Quick Access

### Browser Access
```bash
make open          # Open app in browser
make ldap-admin    # Open phpLDAPadmin in browser
make urls          # Show all service URLs
```

### Service URLs
- **App**: http://localhost:3000
- **phpLDAPadmin**: http://localhost:8080
- **LDAP Server**: ldap://localhost:389
- **LDAPS Server**: ldaps://localhost:636

---

## 📦 Dependencies & Setup

### Initial Setup
```bash
make setup         # Install all dependencies and tools
make setup-go      # Download Go dependencies
make setup-node    # Install Node.js dependencies
make setup-tools   # Install development tools
make setup-hooks   # Install pre-commit hooks
```

### Dependency Updates
```bash
make deps          # Update all dependencies (Go + npm)
```

---

## 🗑️ Cleanup

### Local Cleanup
```bash
make clean         # Remove build artifacts and caches
```

### Docker Cleanup
```bash
make docker-clean  # Clean up containers and volumes (with confirmation)
```

---

## 🔧 Git Workflow

### Git Operations
```bash
make git-status    # Show git status with branch info
make commit        # Interactive commit (stage all + message)
make push          # Push current branch to remote
```

---

## ℹ️ Information

### Build Info
```bash
make info          # Display build information
make help          # Show all available commands
```

---

## 📝 Command Aliases

### Common Shortcuts
```bash
make install       # Alias for: setup
make fmt           # Alias for: fix
make run           # Alias for: up
make dev-start     # Alias for: up
make dev-stop      # Alias for: down
make app-logs      # Alias for: logs-app
```

---

## 🔄 Typical Workflows

### First Time Setup
```bash
make setup         # Install dependencies
make up            # Start services
make open          # Open in browser
```

### Daily Development
```bash
make up            # Start services
make logs-app      # Watch logs
make watch         # Auto-rebuild on changes
# ... make changes ...
make test          # Run tests
make lint          # Check code quality
make down          # Stop when done
```

### Before Committing
```bash
make format-all    # Format all code
make check         # Run lint + tests
make commit        # Interactive commit
make push          # Push to remote
```

### Debugging Issues
```bash
make health        # Check service health
make logs-app      # View app logs
make shell-app     # Open container shell
make inspect       # View environment
make debug         # Start in debug mode
```

### Clean Start
```bash
make fresh         # Clean everything and start fresh
# or
make docker-clean  # Full Docker cleanup
make setup         # Reinstall
make up            # Start fresh
```

---

## 🎯 Pro Tips

1. **Use `make help`** - Always shows current available commands
2. **Combine with `&&`** - Chain commands: `make lint && make test && make build`
3. **Use aliases** - `make run` is shorter than `make up`
4. **Watch logs** - `make logs-app` follows logs in real-time
5. **Quick format** - `make fmt` before committing
6. **Health check** - `make health` to verify everything works
7. **Resource check** - `make stats` to see container usage
8. **Fresh start** - `make fresh` when things get weird

---

## 🐛 Troubleshooting

### Services won't start
```bash
make down
make docker-clean  # Confirm with 'y'
make fresh
```

### Tests failing
```bash
make ldap-reset    # Reset test LDAP
make test
```

### Build errors
```bash
make clean
make setup
make build
```

### Can't access app
```bash
make health        # Check if services healthy
make urls          # Verify URLs
make logs-app      # Check for errors
```
