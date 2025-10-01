# LDAP Manager - Searchable Knowledge Base

**Last Updated:** 2025-09-30
**Purpose:** Quick-reference knowledge base with comprehensive cross-references for developers, operators, and users

---

## 🔍 Quick Search Index

Use Ctrl+F or Command+F to search this document for:

- **Features:** Authentication, caching, pooling, sessions, templates
- **Technologies:** Go, Fiber, Templ, LDAP, TailwindCSS, BBolt
- **Operations:** Deployment, monitoring, troubleshooting, security
- **Components:** Users, groups, computers, health checks
- **Performance:** Benchmarks, optimization, caching strategies

---

## 📚 Documentation Quick Links

### By Topic

| Topic                 | Primary Doc                                                        | Secondary Docs                                                                           | Related                                                      |
| --------------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| **Installation**      | [Installation Guide](user-guide/installation.md)                   | [Docker Development](DOCKER_DEVELOPMENT.md)                                              | [Deployment](operations/deployment.md)                       |
| **Configuration**     | [Configuration Reference](user-guide/configuration.md)             | [Security Config](operations/security-configuration.md), [.env.example](../.env.example) | [INDEX.md](INDEX.md#configuration-locations)                 |
| **API Usage**         | [API Documentation](user-guide/api.md)                             | [Implementation Examples](user-guide/implementation-examples.md)                         | [API Reference](API_REFERENCE.md)                            |
| **Architecture**      | [Architecture Overview](development/architecture.md)               | [Architecture Detailed](development/architecture-detailed.md)                            | [AGENTS.md](../AGENTS.md)                                    |
| **Development Setup** | [Development Setup](development/setup.md)                          | [Docker Development](DOCKER_DEVELOPMENT.md), [Contributing](development/contributing.md) | [AGENTS.md](../AGENTS.md)                                    |
| **Security**          | [Security Configuration](operations/security-configuration.md)     | [Architecture § Security](development/architecture.md#security-architecture)             | [AGENTS.md Security](../internal/AGENTS.md#security--safety) |
| **Performance**       | [Performance Optimization](operations/performance-optimization.md) | [Monitoring](operations/monitoring.md)                                                   | [Benchmarks](INDEX.md#performance-benchmarks)                |
| **Troubleshooting**   | [Troubleshooting Guide](operations/troubleshooting.md)             | [Monitoring](operations/monitoring.md)                                                   | [Health Checks](user-guide/api.md#health-checks)             |

### By Persona

**👤 End Users (Administrators)**

1. [Installation Guide](user-guide/installation.md) - Get started
2. [Configuration Reference](user-guide/configuration.md) - Configure LDAP connection
3. [API Documentation](user-guide/api.md) - Integrate with other systems
4. [Implementation Examples](user-guide/implementation-examples.md) - Real-world scenarios

**👨‍💻 Developers**

1. [Development Setup](development/setup.md) - Setup local environment
2. [AGENTS.md](../AGENTS.md) - AI-assisted coding guidelines
3. [Architecture Overview](development/architecture.md) - Understand system design
4. [Contributing Guidelines](development/contributing.md) - Submit changes
5. [Go Documentation](development/go-doc-reference.md) - Package API reference

**⚙️ Operations / DevOps**

1. [Deployment Guide](operations/deployment.md) - Deploy to production
2. [Monitoring & Troubleshooting](operations/monitoring.md) - Operational procedures
3. [Security Configuration](operations/security-configuration.md) - Harden installation
4. [Performance Optimization](operations/performance-optimization.md) - Tune for scale

---

## 🎯 Feature Reference

### Authentication & Sessions

**How It Works:**

- Session-based authentication with HTTP-only cookies
- LDAP bind validation for user credentials
- Configurable session duration (default: 30 minutes)
- Optional persistent sessions using BBolt database

**Documentation:**

- Implementation: [auth.go](INDEX.md#internalweb---http-server--handlers) in internal/web/
- Configuration: [Configuration Reference](user-guide/configuration.md#session-configuration)
- Security: [Security Configuration](operations/security-configuration.md#authentication)
- AGENTS.md: [Web Security Patterns](../internal/web/AGENTS.md#security--safety)

**Key Functions:**

```go
loginHandler()       // POST /login - Authenticate user
RequireAuth()        // Middleware - Enforce authentication
logoutHandler()      // GET /logout - Clear session
```

**Configuration Variables:**

```bash
PERSIST_SESSIONS=true                 # Enable BBolt persistence
SESSION_PATH=./session.bbolt          # Database file location
SESSION_DURATION=30m                  # Session timeout
```

---

### LDAP Connection Pooling (v1.5.0)

**How It Works:**

- Credential-aware connection pooling from simple-ldap-go v1.5.0
- Separate connections maintained per credential set (security requirement)
- Configurable pool size, timeouts, health checks
- Automatic connection recycling and health monitoring

**Documentation:**

- Architecture: [LDAP Connection Pooling](../claudedocs/ldap-connection-pooling.md)
- Configuration: [INDEX.md § Pool Configuration](INDEX.md#configuration-locations)
- Analysis: [Project Context](../claudedocs/project-context-2025-09-30.md)

**Key Configuration (PR #267 Updates):**

```bash
LDAP_POOL_CONNECTION_TIMEOUT=30s      # NEW: TCP + TLS handshake
LDAP_POOL_ACQUIRE_TIMEOUT=10s         # Pool acquisition wait
LDAP_POOL_MAX_CONNECTIONS=10          # Pool size limit
LDAP_POOL_MIN_CONNECTIONS=2           # Minimum idle connections
LDAP_POOL_MAX_IDLE_TIME=15m          # Idle connection cleanup
LDAP_POOL_HEALTH_CHECK_INTERVAL=30s   # Health check frequency
```

**Performance:**

- Connection reuse: 95%+ hit rate
- Average pool acquisition: <5ms
- TCP handshake: <100ms (30s timeout)

---

### Indexed LDAP Cache (v1.5.0)

**How It Works:**

- O(1) hash-based lookups by Distinguished Name (DN) and SAMAccountName
- Background refresh every 30 seconds (configurable)
- Parallel cache warmup on startup
- Concurrent-safe with RWMutex

**Documentation:**

- Implementation: [cache.go](INDEX.md#internalldap_cache---ldap-entity-caching-90-coverage) and [manager.go](INDEX.md#internalldap_cache---ldap-entity-caching-90-coverage)
- Performance: [Cache Optimization](../claudedocs/cache-optimization-summary.md)
- Benchmarks: [INDEX.md § Performance](INDEX.md#performance-benchmarks)

**Performance (PR #267 Improvements):**

- **287x faster** DN and SAMAccountName lookups (O(1) vs O(n))
- 3x faster cache warmup with parallel goroutines
- Memory overhead: +32 KB per 1000 users

**Key Functions:**

```go
FindByDN(dn string)                        // O(1) indexed lookup
FindBySAMAccountName(samAccountName string) // O(1) indexed lookup
WarmupCache()                              // Parallel initial population
Refresh()                                  // Background update cycle
```

---

### Template Caching

**How It Works:**

- Multi-level caching: Template cache + Fiber response cache
- Path-based invalidation for selective cache clearing
- LRU eviction policy with configurable size limits
- Cache key generation from request path and query params

**Documentation:**

- Implementation: [template_cache.go](INDEX.md#internalweb---http-server--handlers)
- Performance: [Template Caching Optimization](../claudedocs/template-caching-performance-optimization.md)

**Performance:**

- Cache hit rate: 85-95%
- 10x faster rendering for cached templates
- Invalidation speed: <1ms
- Memory usage: ~50 MB for 1000 cached pages

**Key Functions:**

```go
RenderWithCache(c *fiber.Ctx, component templ.Component) error
InvalidateByPath(path string) int
generateCacheKey(c *fiber.Ctx) string
```

---

## 🔧 Configuration Reference

### Environment Variables

#### Required

```bash
LDAP_SERVER=ldaps://dc1.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_READONLY_USER=cn=readonly,DC=example,DC=com
LDAP_READONLY_PASSWORD=SecurePassword123
```

#### Optional - LDAP

```bash
LDAP_IS_AD=true                          # Mark as Active Directory
LDAP_POOL_MAX_CONNECTIONS=10
LDAP_POOL_MIN_CONNECTIONS=2
LDAP_POOL_MAX_IDLE_TIME=15m
LDAP_POOL_HEALTH_CHECK_INTERVAL=30s
LDAP_POOL_CONNECTION_TIMEOUT=30s         # NEW in PR #267
LDAP_POOL_ACQUIRE_TIMEOUT=10s           # NEW in PR #267
```

#### Optional - Application

```bash
LOG_LEVEL=info                           # trace|debug|info|warn|error
PERSIST_SESSIONS=false                   # Enable BBolt session storage
SESSION_PATH=./session.bbolt
SESSION_DURATION=30m
```

📖 **Full Reference:** [Configuration Guide](user-guide/configuration.md)

---

## 🛠️ Development Commands

### Setup & Build

```bash
make setup      # Install Go tools, pnpm deps, templ CLI (one-time)
make build      # Build binary with version injection
make clean      # Remove artifacts and caches
```

### Development

```bash
make dev        # Hot reload: CSS + templates + Go (recommended)
pnpm dev        # Alternative: Run all watchers
make run        # Build and run (no hot reload)
```

### Testing

```bash
make test       # Full test suite with 80% coverage requirement
make test-quick # Quick tests without coverage
make bench      # Run benchmarks
```

### Quality

```bash
make lint       # Run all linters (golangci-lint, security, format)
make fix        # Auto-fix formatting issues (gofmt, prettier)
make check      # Quality gate: lint + test (required pre-commit)
```

### Docker

```bash
make docker-dev      # Full containerized dev environment
make docker-build    # Build production Docker image
make docker-test     # Run tests in container
```

📖 **Complete Reference:** Run `make help` or see [Development Setup](development/setup.md)

---

## 🏗️ Architecture Quick Reference

### Component Map

```
┌─────────────────────────────────────────────┐
│  HTTP Layer (Fiber v2)                      │
│  ├─ server.go - App initialization          │
│  ├─ auth.go - Authentication                │
│  ├─ users.go, groups.go, computers.go       │
│  ├─ middleware.go - Auth, caching           │
│  └─ template_cache.go - Template caching    │
└─────────────────────────────────────────────┘
               ↓
┌─────────────────────────────────────────────┐
│  Business Logic Layer                       │
│  ├─ ldap_cache/ - Indexed caching (90% cov) │
│  │  ├─ cache.go - O(1) indexed lookups      │
│  │  ├─ manager.go - Auto-refresh            │
│  │  └─ metrics.go - Performance tracking    │
│  └─ options/ - Configuration management     │
└─────────────────────────────────────────────┘
               ↓
┌─────────────────────────────────────────────┐
│  Data Access Layer (simple-ldap-go v1.5.0)  │
│  ├─ Connection pool (credential-aware)      │
│  ├─ Health monitoring (30s intervals)       │
│  └─ Timeout management (separate TCP/pool)  │
└─────────────────────────────────────────────┘
```

📖 **Detailed Architecture:** [Architecture Overview](development/architecture.md)

---

## 🔐 Security Quick Reference

### Authentication Flow

1. User submits credentials via `/login` form
2. Server validates against LDAP server
3. Creates HTTP-only session cookie (SameSite=Strict)
4. All subsequent requests validated via `RequireAuth()` middleware

### Input Validation

- LDAP injection prevention: `escapeLDAPFilter()` escapes `\ * ( ) \x00`
- Form validation: Email format, field lengths, LDAP meta characters
- CSRF protection: Token validation on all POST requests

### Secrets Management

- **Never in VCS:** Enforced by detect-secrets baseline
- **Environment variables:** All secrets via `.env` or environment
- **No logging:** Passwords never logged (code review enforced)
- **LDAPS required:** TLS encryption for Active Directory

📖 **Full Security Guide:** [Security Configuration](operations/security-configuration.md)

---

## 📊 Performance Tuning

### Cache Configuration

**LDAP Cache Refresh Interval:**

```go
// Default: 30 seconds
manager := ldap_cache.NewWithConfig(client, 30*time.Second)
```

**Template Cache Settings:**

```go
// internal/web/template_cache.go
MaxEntries:      1000          // LRU cache size
CleanupInterval: 5 * time.Minute
```

### Connection Pool Tuning

**High-Traffic Environments:**

```bash
LDAP_POOL_MAX_CONNECTIONS=50        # Increase from default 10
LDAP_POOL_MIN_CONNECTIONS=10        # Maintain more idle connections
LDAP_POOL_MAX_IDLE_TIME=5m         # Shorter idle time
```

**Low-Latency Requirements:**

```bash
LDAP_POOL_CONNECTION_TIMEOUT=10s    # Faster failover
LDAP_POOL_ACQUIRE_TIMEOUT=5s        # Lower acquisition timeout
```

📖 **Tuning Guide:** [Performance Optimization](operations/performance-optimization.md)

---

## 🆘 Troubleshooting Quick Reference

### Common Issues

**Build Fails**

```bash
# Solution
make clean && make setup
go mod tidy
pnpm install
```

**Tests Fail**

```bash
# Check LDAP server is running
docker compose --profile dev up openldap -d
# Run with verbose output
go test -v ./...
```

**Can't Connect to LDAP**

```bash
# Verify configuration
cat .env | grep LDAP_
# Test LDAP connection
ldapsearch -H ldaps://dc1.example.com:636 -D "cn=readonly,DC=example,DC=com" -w password -b "DC=example,DC=com"
```

**Performance Issues**

```bash
# Check cache hit rates
curl http://localhost:3000/debug/cache
# Check pool stats
curl http://localhost:3000/debug/ldap-pool
```

📖 **Full Troubleshooting Guide:** [Troubleshooting](operations/troubleshooting.md)

---

## 📖 Document Cross-Reference Map

### Primary Documents → Related Resources

**[Installation Guide](user-guide/installation.md)**

- Prerequisites: Docker, Go 1.25+, Node.js v16+
- Related: [Deployment Guide](operations/deployment.md), [Docker Development](DOCKER_DEVELOPMENT.md)
- Configuration: [Configuration Reference](user-guide/configuration.md)

**[Configuration Reference](user-guide/configuration.md)**

- Template: [.env.example](../.env.example)
- Related: [Security Configuration](operations/security-configuration.md)
- AGENTS: [internal/AGENTS.md § Configuration](../internal/AGENTS.md)

**[API Documentation](user-guide/api.md)**

- Reference: [API_REFERENCE.md](API_REFERENCE.md)
- Examples: [Implementation Examples](user-guide/implementation-examples.md)
- Code: [Go Documentation](development/go-doc-reference.md)

**[Architecture Overview](development/architecture.md)**

- Deep Dive: [Architecture Detailed](development/architecture-detailed.md)
- Analysis: [Comprehensive Analysis](../claudedocs/comprehensive-project-analysis.md)
- Code: [AGENTS.md](../AGENTS.md)

**[Development Setup](development/setup.md)**

- Guidelines: [Contributing](development/contributing.md)
- AGENTS: [AGENTS.md](../AGENTS.md), [cmd/AGENTS.md](../cmd/AGENTS.md)
- Docker: [Docker Development](DOCKER_DEVELOPMENT.md)

---

## 🔗 External Resources Index

### Official Documentation

- **Go Language:** https://go.dev/doc/ ([Tutorial](https://go.dev/tour/), [Effective Go](https://go.dev/doc/effective_go))
- **Fiber v2:** https://docs.gofiber.io/ ([API](https://docs.gofiber.io/api/fiber), [Middleware](https://docs.gofiber.io/api/middleware))
- **Templ:** https://templ.guide/ ([Components](https://templ.guide/syntax-and-usage/components), [IDE Support](https://templ.guide/commands-and-tools/ide-support))
- **TailwindCSS:** https://tailwindcss.com/docs ([Configuration](https://tailwindcss.com/docs/configuration), [Utility Classes](https://tailwindcss.com/docs/utility-first))
- **go-ldap/ldap:** https://pkg.go.dev/github.com/go-ldap/ldap/v3

### Project Resources

- **simple-ldap-go v1.5.0:** https://github.com/netresearch/simple-ldap-go
  - Our Contribution: [PR #45 - Multi-key Indexed Cache](https://github.com/netresearch/simple-ldap-go/pull/45)
- **BBolt:** https://github.com/etcd-io/bbolt ([Getting Started](https://github.com/etcd-io/bbolt#getting-started))

### Learning Resources

- **Go Testing:** https://go.dev/doc/tutorial/add-a-test
- **LDAP Basics:** https://ldap.com/learn-about-ldap/
- **Active Directory:** https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/

---

## 📝 Keywords for Search

**Features:** Authentication, Authorization, Caching, Connection Pooling, Health Checks, LDAP, Active Directory, Session Management, Template Rendering, Static Assets

**Technologies:** Go, Golang, Fiber, Templ, TailwindCSS, BBolt, LDAP, Active Directory, Docker, pnpm

**Operations:** Deployment, Monitoring, Troubleshooting, Performance, Security, Configuration, Environment Variables

**Components:** Users, Groups, Computers, Health Endpoints, Login, Logout, Middleware, Templates

**Performance:** Benchmarks, Optimization, Caching, Indexed Cache, Connection Pool, Template Cache, O(1) Lookup, 287x Improvement

**Quality:** Testing, Coverage, Linting, golangci-lint, Security, AGENTS.md, Documentation

**Development:** Setup, Build, Hot Reload, Docker Compose, Make Targets, Pre-commit Hooks

---

_Last updated: 2025-09-30 | Comprehensive knowledge base with search aids and cross-references_
