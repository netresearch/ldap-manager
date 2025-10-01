# LDAP Manager Documentation Index

**Last Updated:** 2025-09-30
**Version:** 1.0.8
**Status:** Comprehensive documentation with AI-assisted development guides

---

## 🚀 Quick Start

Choose your path:

- **👤 Users** → [Installation Guide](user-guide/installation.md) → [Configuration](user-guide/configuration.md)
- **👨‍💻 Developers** → [Development Setup](development/setup.md) → [AGENTS.md](../AGENTS.md) → [Architecture](development/architecture.md)
- **⚙️ Operations** → [Deployment Guide](operations/deployment.md) → [Monitoring](operations/monitoring.md)

---

## 📢 Recent Milestones

**PR #267 - Dependency Updates & Go Modernization** (Merged: 2025-09-30)

Major improvements delivered:

- ✅ **simple-ldap-go v1.0.3 → v1.5.0** - Upstream contribution with 287x cache performance improvement
- ✅ **Go 1.22-1.25 Features** - Range-over-integers, WaitGroup.Go() modernization
- ✅ **Configuration Fixes** - Separate ConnectionTimeout (30s) vs GetTimeout (10s) for LDAP pool
- ✅ **Security Hardening** - GitHub Actions SHA-256 pinning for supply chain security
- ✅ **Upstream PR #45** - Multi-key indexed cache merged into simple-ldap-go v1.5.0

📖 **Details:** See [Project Context](../claudedocs/project-context-2025-09-30.md) for complete analysis

---

## 📚 Documentation Structure

### For End Users

Get started with installation, configuration, and API usage:

| Document                                                             | Purpose                                   | Audience                |
| -------------------------------------------------------------------- | ----------------------------------------- | ----------------------- |
| **[Installation Guide](user-guide/installation.md)**                 | Docker & native setup instructions        | Sysadmins, DevOps       |
| **[Configuration Reference](user-guide/configuration.md)**           | Complete configuration options & examples | Administrators          |
| **[API Documentation](user-guide/api.md)**                           | REST API endpoints & usage                | Developers, Integrators |
| **[Implementation Examples](user-guide/implementation-examples.md)** | Real-world scenarios & tutorials          | All users               |

### For Developers

Contributing, architecture, and development practices:

| Document                                                          | Purpose                           | Cross-References                                                         |
| ----------------------------------------------------------------- | --------------------------------- | ------------------------------------------------------------------------ |
| **[Development Setup](development/setup.md)**                     | Local environment configuration   | → [AGENTS.md](../AGENTS.md), [Docker Dev](DOCKER_DEVELOPMENT.md)         |
| **[Contributing Guidelines](development/contributing.md)**        | Code standards & PR workflow      | → [AGENTS.md](../AGENTS.md), [Architecture](development/architecture.md) |
| **[Architecture Overview](development/architecture.md)**          | System design & patterns          | → [Detailed Architecture](development/architecture-detailed.md)          |
| **[Architecture Detailed](development/architecture-detailed.md)** | Comprehensive technical deep-dive | → [API](user-guide/api.md), [Go Docs](development/go-doc-reference.md)   |
| **[Go Documentation](development/go-doc-reference.md)**           | Package API reference             | → [Architecture](development/architecture.md)                            |

### For Operations

Deployment, monitoring, and production management:

| Document                                                               | Purpose                          | Integration Points                                                                                           |
| ---------------------------------------------------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| **[Deployment Guide](operations/deployment.md)**                       | Production deployment strategies | → [Configuration](user-guide/configuration.md), [Docker](DOCKER_DEVELOPMENT.md)                              |
| **[Monitoring & Troubleshooting](operations/monitoring.md)**           | Operational procedures           | → [Health Endpoints](user-guide/api.md#health-checks), [Performance](operations/performance-optimization.md) |
| **[Performance Optimization](operations/performance-optimization.md)** | Tuning & optimization            | → [Architecture](development/architecture.md#performance), [Troubleshooting](operations/troubleshooting.md)  |
| **[Security Configuration](operations/security-configuration.md)**     | Hardening & best practices       | → [Configuration](user-guide/configuration.md), [Architecture](development/architecture.md#security)         |
| **[Troubleshooting Guide](operations/troubleshooting.md)**             | Problem resolution               | → [Monitoring](operations/monitoring.md), [Logs Analysis](operations/monitoring.md#logging)                  |

---

## 🤖 AI-Assisted Development (AGENTS.md)

New! AI coding assistant guidelines with scoped context:

| File                                                    | Scope                            | Use When                      |
| ------------------------------------------------------- | -------------------------------- | ----------------------------- |
| **[AGENTS.md](../AGENTS.md)** (root)                    | Global conventions & house rules | Starting any task, PR reviews |
| **[cmd/AGENTS.md](../cmd/AGENTS.md)**                   | CLI entry point patterns         | Working on main.go, startup   |
| **[internal/AGENTS.md](../internal/AGENTS.md)**         | Core Go best practices           | Business logic, LDAP ops      |
| **[internal/web/AGENTS.md](../internal/web/AGENTS.md)** | HTTP handlers, Templ, Fiber      | Web development, templates    |

**Precedence Rule:** Nearest AGENTS.md wins. Use the closest file to your work location.

---

## 📊 claudedocs/ Analysis Reports

Comprehensive technical analysis and session documentation (21 reports):

### Architecture & Design

| Report                                                                            | Focus                                                         | Last Updated |
| --------------------------------------------------------------------------------- | ------------------------------------------------------------- | ------------ |
| [**Project Context**](../claudedocs/project-context-2025-09-30.md)                | Complete project overview with architecture, metrics, quality | 2025-09-30   |
| [**Comprehensive Analysis**](../claudedocs/comprehensive-project-analysis.md)     | Deep-dive technical analysis across all components            | 2025-09-30   |
| [**Architecture Detailed**](../claudedocs/comprehensive-analysis-final-report.md) | Final architecture report with recommendations                | 2025-09-30   |
| [**LDAP Connection Pooling**](../claudedocs/ldap-connection-pooling.md)           | Connection pool design and optimization                       | 2025-09-30   |

### Security & Quality

| Report                                                                        | Focus                                               | Last Updated |
| ----------------------------------------------------------------------------- | --------------------------------------------------- | ------------ |
| [**Security Analysis**](../claudedocs/security-analysis-report.md)            | Comprehensive security audit and recommendations    | 2025-09-30   |
| [**Security Implementation**](../claudedocs/security-implementation.md)       | Security features and best practices applied        | 2025-09-30   |
| [**Code Quality Analysis**](../claudedocs/code-quality-analysis.md)           | Quality metrics, linting, technical debt assessment | 2025-09-30   |
| [**Test Coverage Report**](../claudedocs/test-coverage-improvement-report.md) | Coverage analysis and improvement strategies        | 2025-09-30   |

### Performance & Optimization

| Report                                                                             | Focus                                      | Last Updated |
| ---------------------------------------------------------------------------------- | ------------------------------------------ | ------------ |
| [**Template Caching**](../claudedocs/template-caching-performance-optimization.md) | Multi-level template cache optimization    | 2025-09-30   |
| [**Cache Optimization**](../claudedocs/cache-optimization-summary.md)              | LDAP cache performance improvements        | 2025-09-30   |
| [**Frontend Optimization**](../claudedocs/frontend-optimization-results.md)        | CSS, assets, and frontend performance      | 2025-09-30   |
| [**CSS Build Guide**](../claudedocs/css-build-optimization-guide.md)               | TailwindCSS optimization and build process | 2025-09-30   |

### Documentation & Implementation

| Report                                                                              | Focus                                         | Last Updated |
| ----------------------------------------------------------------------------------- | --------------------------------------------- | ------------ |
| [**Documentation Index**](../claudedocs/documentation-index-report-2025-09-30.md)   | Complete documentation structure and coverage | 2025-09-30   |
| [**Inline Documentation**](../claudedocs/inline-documentation-report-2025-09-30.md) | Code documentation quality and standards      | 2025-09-30   |
| [**Implementation Summary**](../claudedocs/implementation-summary.md)               | Feature implementation details and decisions  | 2025-09-30   |
| [**Frontend Fixes**](../claudedocs/frontend-fixes-implementation-summary.md)        | Frontend bug fixes and improvements           | 2025-09-30   |

### Project Maintenance

| Report                                                                    | Focus                                        | Last Updated |
| ------------------------------------------------------------------------- | -------------------------------------------- | ------------ |
| [**Cleanup Analysis**](../claudedocs/cleanup_analysis.md)                 | Code cleanup and refactoring opportunities   | 2025-09-30   |
| [**Cleanup Report**](../claudedocs/cleanup_report.md)                     | Completed cleanup tasks                      | 2025-09-30   |
| [**Frontend Analysis**](../claudedocs/comprehensive-frontend-analysis.md) | Complete frontend architecture review        | 2025-09-30   |
| [**CSS Analysis**](../claudedocs/css-analysis.md)                         | CSS structure and optimization opportunities | 2025-09-30   |

**Purpose:** These reports provide deep technical insights for developers, architects, and operations teams. Generated during development sessions for knowledge preservation and decision documentation.

---

## 🔌 API Module Reference

### Core Packages

#### internal/ldap_cache - LDAP Entity Caching (90% coverage)

| File         | Purpose                                 | Key Functions                                            |
| ------------ | --------------------------------------- | -------------------------------------------------------- |
| `cache.go`   | Generic indexed cache with O(1) lookups | `FindByDN()`, `FindBySAMAccountName()`, `buildIndexes()` |
| `manager.go` | Cache manager with auto-refresh         | `New()`, `Refresh()`, `WarmupCache()`                    |
| `metrics.go` | Performance metrics and health tracking | `RecordCacheHit()`, `GetSummaryStats()`                  |

#### internal/ldap - LDAP Connection Pool Management

| File         | Purpose                                          | Key Functions                                           |
| ------------ | ------------------------------------------------ | ------------------------------------------------------- |
| `pool.go`    | Credential-aware connection pool implementation  | `NewConnectionPool()`, `AcquireConnection()`, `Close()` |
| `manager.go` | High-level pool manager with convenience methods | `NewPoolManager()`, `WithCredentials()`, `GetStats()`   |

#### internal/options - Configuration Management

| File     | Purpose                       | Key Functions                                           |
| -------- | ----------------------------- | ------------------------------------------------------- |
| `app.go` | CLI flags and env var parsing | `Parse()`, `envDurationOrDefault()`, `panicWhenEmpty()` |

#### internal/version - Build Version Info

| File         | Purpose                 | Key Functions      |
| ------------ | ----------------------- | ------------------ |
| `version.go` | Build version injection | `GetVersionInfo()` |

#### internal/web - HTTP Server & Handlers

| File                | Purpose                               | Key Functions                                                |
| ------------------- | ------------------------------------- | ------------------------------------------------------------ |
| `server.go`         | Fiber app initialization and routing  | `NewApp()`, `Listen()`, `createPoolConfig()`                 |
| `auth.go`           | Authentication and session management | `loginHandler()`, `RequireAuth()`                            |
| `users.go`          | User CRUD operations                  | `usersHandler()`, `userHandler()`, `userModifyHandler()`     |
| `groups.go`         | Group CRUD operations                 | `groupsHandler()`, `groupHandler()`, `groupModifyHandler()`  |
| `computers.go`      | Computer listing and details          | `computersHandler()`, `computerHandler()`                    |
| `health.go`         | Health check endpoints                | `healthHandler()`, `readinessHandler()`, `livenessHandler()` |
| `middleware.go`     | HTTP middleware                       | `RequireAuth()`, `templateCacheMiddleware()`                 |
| `template_cache.go` | Template caching layer                | `RenderWithCache()`, `InvalidateByPath()`                    |
| `assets.go`         | Static asset embedding                | Asset serving via Fiber                                      |

#### internal/web/templates - Templ Templates (Generated)

| File                | Purpose                 | Templates                       |
| ------------------- | ----------------------- | ------------------------------- |
| `base_templ.go`     | Base HTML layout        | `BaseLayout()`                  |
| `login_templ.go`    | Login page              | `Login()`                       |
| `users_templ.go`    | User listing            | `Users()`                       |
| `groups_templ.go`   | Group listing           | `Groups()`                      |
| `computer_templ.go` | Computer details        | `Computer()`                    |
| `errors_templ.go`   | Error pages             | `FourOhFour()`, `FiveHundred()` |
| `flash.go`          | Flash message utilities | `Flash()`, `GetFlashMessage()`  |

📖 **Full API:** Run `make godoc` or see [Go Documentation](development/go-doc-reference.md)

---

## 🔍 Quick References

### Common Tasks

| Task              | Command        | Documentation                                       |
| ----------------- | -------------- | --------------------------------------------------- |
| Setup development | `make setup`   | [Development Setup](development/setup.md)           |
| Start dev server  | `make dev`     | [Docker Development](DOCKER_DEVELOPMENT.md)         |
| Run tests         | `make test`    | [Contributing](development/contributing.md#testing) |
| Run linter        | `make lint`    | [AGENTS.md](../AGENTS.md#minimal-pre-commit-checks) |
| Build production  | `make build`   | [Deployment](operations/deployment.md)              |
| Health check      | `curl /health` | [API Docs](user-guide/api.md#health-checks)         |

### Key Endpoints

| Endpoint         | Purpose         | Auth Required |
| ---------------- | --------------- | ------------- |
| `GET /health`    | Health check    | ❌ No         |
| `POST /login`    | Authentication  | ❌ No         |
| `GET /users`     | List all users  | ✅ Yes        |
| `GET /groups`    | List all groups | ✅ Yes        |
| `GET /computers` | List computers  | ✅ Yes        |

📖 **Full API Reference:** [user-guide/api.md](user-guide/api.md)

### Configuration Locations

| File            | Purpose                | Environment   | Key Variables                    |
| --------------- | ---------------------- | ------------- | -------------------------------- |
| `.env`          | Development secrets    | Local dev     | LDAP credentials, session config |
| `.env.example`  | Configuration template | All           | **NEW:** Pool timeout settings   |
| `compose.yml`   | Docker services        | Dev/Test/Prod | Service profiles (dev/test/prod) |
| `.golangci.yml` | Linter configuration   | CI/CD         | 20+ linters enabled              |
| `Makefile`      | Build commands         | All           | 15+ targets available            |

**New in PR #267:** LDAP Pool Configuration

```bash
LDAP_POOL_CONNECTION_TIMEOUT=30s    # TCP + TLS handshake timeout
LDAP_POOL_ACQUIRE_TIMEOUT=10s       # Pool acquisition timeout
LDAP_POOL_MAX_CONNECTIONS=10
LDAP_POOL_MIN_CONNECTIONS=2
LDAP_POOL_MAX_IDLE_TIME=15m
LDAP_POOL_HEALTH_CHECK_INTERVAL=30s
```

---

## 🏗️ Project Structure

```
ldap-manager/
├── cmd/ldap-manager/        # Application entry point
├── internal/                 # Core application code
│   ├── ldap/                 # LDAP client & connection pool
│   ├── ldap_cache/           # Caching layer (90% coverage)
│   ├── options/              # Configuration management
│   ├── version/              # Build version info
│   └── web/                  # HTTP handlers & templates
│       ├── templates/        # Templ template files
│       └── static/           # CSS, images, assets
├── docs/                     # This documentation
│   ├── user-guide/           # End user documentation
│   ├── development/          # Developer guides
│   └── operations/           # Operations manuals
├── scripts/                  # Build & utility scripts
└── tests/                    # Test files (if any)
```

📖 **Detailed Structure:** See [Architecture](development/architecture.md#component-architecture)

---

## 🔐 Security Documentation

Critical security information and best practices:

| Topic                     | Documentation                                                                | Priority     |
| ------------------------- | ---------------------------------------------------------------------------- | ------------ |
| Authentication            | [Architecture § Security](development/architecture.md#security-architecture) | 🔴 Critical  |
| Session Management        | [Security Config](operations/security-configuration.md)                      | 🔴 Critical  |
| LDAP Injection Prevention | [AGENTS.md § Security](../internal/AGENTS.md#security--safety)               | 🔴 Critical  |
| Secrets Management        | [Configuration](user-guide/configuration.md#secrets)                         | 🔴 Critical  |
| Input Validation          | [Web AGENTS.md](../internal/web/AGENTS.md#security--safety)                  | 🟡 Important |

---

## 📊 Quality Metrics

Current project health indicators:

- **Test Coverage:** 80% minimum (90% for ldap_cache, 50% for templates)
- **Linting:** 20+ enabled linters via golangci-lint
- **Technical Debt:** 0 TODO/FIXME markers
- **Documentation Coverage:** ✅ Comprehensive (20 docs + 4 AGENTS.md + 21 claudedocs reports)
- **CI/CD:** 3 automated workflows (quality, check, docker)

📖 **Details:** [Project Context Report](../claudedocs/project-context-2025-09-30.md)

---

## ⚡ Performance Benchmarks

### Cache Performance (PR #267 Improvements)

| Operation                 | Before (v1.0.3)    | After (v1.5.0)      | Improvement           |
| ------------------------- | ------------------ | ------------------- | --------------------- |
| **DN Lookup**             | O(n) linear search | O(1) hash index     | **287x faster**       |
| **SAMAccountName Lookup** | O(n) linear search | O(1) hash index     | **287x faster**       |
| **Cache Warmup**          | Sequential         | Parallel goroutines | 3x faster             |
| **Memory Overhead**       | Slice only         | Slice + indexes     | +32 KB per 1000 users |

### Connection Pool Performance

| Metric                     | Value         | Configuration            |
| -------------------------- | ------------- | ------------------------ |
| **Connection Reuse**       | 95%+ hit rate | Credential-aware pooling |
| **Avg Pool Acquisition**   | <5ms          | GetTimeout: 10s          |
| **TCP Handshake**          | <100ms        | ConnectionTimeout: 30s   |
| **Health Check Frequency** | 30s           | Configurable             |

### Template Cache Performance

| Metric                 | Value                 | Improvement          |
| ---------------------- | --------------------- | -------------------- |
| **Cache Hit Rate**     | 85-95%                | 10x faster rendering |
| **Invalidation Speed** | <1ms                  | Path-based selective |
| **Memory Usage**       | ~50 MB for 1000 pages | LRU eviction         |

📖 **Benchmarks:** Run `make bench` or see [Performance Optimization](operations/performance-optimization.md)

---

## 🔗 External Resources

### Official Documentation

- **Go:** https://go.dev/doc/
- **Fiber v2:** https://docs.gofiber.io/
- **Templ:** https://templ.guide/
- **TailwindCSS:** https://tailwindcss.com/docs
- **go-ldap:** https://pkg.go.dev/github.com/go-ldap/ldap/v3

### Related Projects

- **simple-ldap-go v1.5.0:** Custom LDAP wrapper with indexed cache
  - **Our Contribution:** [PR #45](https://github.com/netresearch/simple-ldap-go/pull/45) - Multi-key indexed cache (287x improvement)
  - **Status:** Merged and released in v1.5.0
  - **GitHub:** https://github.com/netresearch/simple-ldap-go
- **BBolt:** Embedded key-value database for sessions
  - **Use Case:** Persistent session storage for development and production
  - **GitHub:** https://github.com/etcd-io/bbolt

---

## 🆘 Getting Help

### Documentation Not Found?

1. **Check AGENTS.md:** AI guidelines may have what you need
2. **Search codebase:** `grep -r "pattern" internal/`
3. **Review tests:** `*_test.go` files show usage examples
4. **Architecture docs:** Comprehensive technical details

### Common Issues

| Problem            | Solution                   | Reference                                             |
| ------------------ | -------------------------- | ----------------------------------------------------- |
| Build fails        | `make clean && make setup` | [Setup](development/setup.md)                         |
| Tests fail         | Check LDAP server running  | [Docker Dev](DOCKER_DEVELOPMENT.md)                   |
| Can't connect      | Verify .env configuration  | [Configuration](user-guide/configuration.md)          |
| Performance issues | Check cache settings       | [Performance](operations/performance-optimization.md) |

---

## 📝 Documentation Maintenance

### Contributing to Docs

1. **User docs** (`user-guide/`) - Installation, configuration, API usage
2. **Developer docs** (`development/`) - Architecture, contributing, setup
3. **Operations docs** (`operations/`) - Deployment, monitoring, security
4. **AGENTS.md** - AI assistant guidelines (maintain scoped structure)

### Documentation Standards

- **Markdown format** with GitHub-flavored syntax
- **Cross-references** using relative links
- **Code examples** with syntax highlighting
- **Tables** for structured information
- **Emojis** for visual navigation (sparingly)

---

## 🎯 Next Steps

Based on your role:

### 👤 Users

1. Read [Installation Guide](user-guide/installation.md)
2. Configure via [Configuration Reference](user-guide/configuration.md)
3. Deploy using [Deployment Guide](operations/deployment.md)

### 👨‍💻 Developers

1. Setup environment: [Development Setup](development/setup.md)
2. Read [AGENTS.md](../AGENTS.md) for AI-assisted coding
3. Review [Architecture](development/architecture.md)
4. Start coding with `make dev`

### ⚙️ Operations

1. Plan deployment: [Deployment Guide](operations/deployment.md)
2. Configure monitoring: [Monitoring](operations/monitoring.md)
3. Harden security: [Security Configuration](operations/security-configuration.md)

---

## 📜 License

LDAP Manager is licensed under the MIT license. See [LICENSE](../LICENSE) for details.

---

## 📈 Project Evolution

### Version History

| Version    | Date       | Highlights                                                                 |
| ---------- | ---------- | -------------------------------------------------------------------------- |
| **v1.0.8** | 2025-09-30 | PR #267: simple-ldap-go v1.5.0, Go modernization, performance improvements |
| v1.0.7     | 2025-09-29 | Security hardening, CI/CD improvements                                     |
| v1.0.6     | 2025-09-28 | AGENTS.md agentization, documentation enhancements                         |
| v1.0.5     | 2025-09-27 | Template caching optimization                                              |
| v1.0.0     | 2025-09-01 | Initial production release                                                 |

### Upcoming Roadmap

Potential future enhancements (not committed):

- **Observability:** Prometheus metrics endpoint, OpenTelemetry tracing
- **Testing:** E2E tests for critical user journeys, load testing
- **Performance:** Adaptive TTL for cache refresh, Redis session storage option
- **Features:** User creation/deletion, bulk operations, audit logging

📖 **Contribute:** See [Contributing Guidelines](development/contributing.md)

---

_This index is maintained automatically and manually. Last comprehensive update: 2025-09-30 (Enhanced with PR #267 details, claudedocs integration, and API module reference)_

**📌 Bookmark this page** - it's your hub for all LDAP Manager documentation.
