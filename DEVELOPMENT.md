# Development Environment Setup

This document describes how to set up a complete development environment for the LDAP Manager application.

## Prerequisites

- Docker and Docker Compose installed
- Git for version control
- Node.js 22+ and pnpm (for frontend development)
- Go 1.25.1+ (for backend development)

## Quick Start with Docker Compose

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