# Docker Development Guide

This guide explains how to use the containerized development environment for LDAP Manager.

## Quick Start

All development tools are now containerized and available through the Makefile:

```bash
# Run linter in container (no local Go installation needed)
make docker-lint

# Run tests in container
make docker-test  

# Run all quality checks in container
make docker-check

# Start development environment with live reload
make docker-dev

# Open shell in development container
make docker-shell

# Clean up Docker resources
make docker-clean
```

## Available Docker Targets

| Target | Description |
|--------|-------------|
| `make docker-dev-build` | Build development container with all tools |
| `make docker-dev` | Start development environment with live reload |
| `make docker-test` | Run tests in container |
| `make docker-lint` | Run linter in container |
| `make docker-check` | Run all quality checks in container |
| `make docker-shell` | Open shell in development container |
| `make docker-clean` | Clean up containers and volumes |
| `make docker-logs` | Show logs from development container |

## Development Container Features

The development container includes:

- **Go 1.25.1** with all development tools
- **Node.js & PNPM** for frontend development
- **golangci-lint** for comprehensive linting
- **gofumpt, goimports** for code formatting
- **templ** for template generation
- **make, git, curl** for development workflow

## Docker Compose Profiles

| Profile | Usage | Description |
|---------|-------|-------------|
| `dev` | `docker compose --profile dev up` | Development with live reload |
| `test` | `docker compose --profile test run ldap-manager-test` | Testing environment |
| `prod` | `docker compose up` (default) | Production deployment |

## Persistent Volumes

The development setup uses persistent volumes for performance:

- `go_modules`: Cache Go module downloads
- `go_cache`: Cache Go build artifacts
- `ldap_sessions`: Persist session data
- `ldap_data`: Persist LDAP server data

## Environment Variables

The containerized environment uses the same environment variables as documented in the main [Configuration Guide](user-guide/configuration.md).

## Troubleshooting

### Network Conflicts

If you see network subnet conflicts:

```bash
# Clean up existing networks
make docker-clean

# Or manually remove conflicting networks
docker network ls | grep ldap
docker network rm <network-name>
```

### Port Conflicts

Services use these ports:

- `3000`: LDAP Manager application
- `389`: LDAP server (plain)
- `636`: LDAP server (TLS)
- `8080`: phpLDAPadmin web interface

### Permission Issues

If you encounter permission issues with mounted volumes:

```bash
# Fix ownership in development container
make docker-shell
chown -R $USER:$USER /app
```

## CI/CD Integration

Use containerized targets in CI/CD:

```yaml
# Example GitHub Actions
- name: Run Quality Checks
  run: make docker-check

- name: Run Tests
  run: make docker-test
```

This ensures consistent behavior between local development and CI/CD environments.