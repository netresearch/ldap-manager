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

## 📚 Documentation Structure

### For End Users

Get started with installation, configuration, and API usage:

| Document | Purpose | Audience |
|----------|---------|----------|
| **[Installation Guide](user-guide/installation.md)** | Docker & native setup instructions | Sysadmins, DevOps |
| **[Configuration Reference](user-guide/configuration.md)** | Complete configuration options & examples | Administrators |
| **[API Documentation](user-guide/api.md)** | REST API endpoints & usage | Developers, Integrators |
| **[Implementation Examples](user-guide/implementation-examples.md)** | Real-world scenarios & tutorials | All users |

### For Developers

Contributing, architecture, and development practices:

| Document | Purpose | Cross-References |
|----------|---------|------------------|
| **[Development Setup](development/setup.md)** | Local environment configuration | → [AGENTS.md](../AGENTS.md), [Docker Dev](DOCKER_DEVELOPMENT.md) |
| **[Contributing Guidelines](development/contributing.md)** | Code standards & PR workflow | → [AGENTS.md](../AGENTS.md), [Architecture](development/architecture.md) |
| **[Architecture Overview](development/architecture.md)** | System design & patterns | → [Detailed Architecture](development/architecture-detailed.md) |
| **[Architecture Detailed](development/architecture-detailed.md)** | Comprehensive technical deep-dive | → [API](user-guide/api.md), [Go Docs](development/go-doc-reference.md) |
| **[Go Documentation](development/go-doc-reference.md)** | Package API reference | → [Architecture](development/architecture.md) |

### For Operations

Deployment, monitoring, and production management:

| Document | Purpose | Integration Points |
|----------|---------|-------------------|
| **[Deployment Guide](operations/deployment.md)** | Production deployment strategies | → [Configuration](user-guide/configuration.md), [Docker](DOCKER_DEVELOPMENT.md) |
| **[Monitoring & Troubleshooting](operations/monitoring.md)** | Operational procedures | → [Health Endpoints](user-guide/api.md#health-checks), [Performance](operations/performance-optimization.md) |
| **[Performance Optimization](operations/performance-optimization.md)** | Tuning & optimization | → [Architecture](development/architecture.md#performance), [Troubleshooting](operations/troubleshooting.md) |
| **[Security Configuration](operations/security-configuration.md)** | Hardening & best practices | → [Configuration](user-guide/configuration.md), [Architecture](development/architecture.md#security) |
| **[Troubleshooting Guide](operations/troubleshooting.md)** | Problem resolution | → [Monitoring](operations/monitoring.md), [Logs Analysis](operations/monitoring.md#logging) |

---

## 🤖 AI-Assisted Development (AGENTS.md)

New! AI coding assistant guidelines with scoped context:

| File | Scope | Use When |
|------|-------|----------|
| **[AGENTS.md](../AGENTS.md)** (root) | Global conventions & house rules | Starting any task, PR reviews |
| **[cmd/AGENTS.md](../cmd/AGENTS.md)** | CLI entry point patterns | Working on main.go, startup |
| **[internal/AGENTS.md](../internal/AGENTS.md)** | Core Go best practices | Business logic, LDAP ops |
| **[internal/web/AGENTS.md](../internal/web/AGENTS.md)** | HTTP handlers, Templ, Fiber | Web development, templates |

**Precedence Rule:** Nearest AGENTS.md wins. Use the closest file to your work location.

---

## 🔍 Quick References

### Common Tasks

| Task | Command | Documentation |
|------|---------|---------------|
| Setup development | `make setup` | [Development Setup](development/setup.md) |
| Start dev server | `make dev` | [Docker Development](DOCKER_DEVELOPMENT.md) |
| Run tests | `make test` | [Contributing](development/contributing.md#testing) |
| Run linter | `make lint` | [AGENTS.md](../AGENTS.md#minimal-pre-commit-checks) |
| Build production | `make build` | [Deployment](operations/deployment.md) |
| Health check | `curl /health` | [API Docs](user-guide/api.md#health-checks) |

### Key Endpoints

| Endpoint | Purpose | Auth Required |
|----------|---------|---------------|
| `GET /health` | Health check | ❌ No |
| `POST /login` | Authentication | ❌ No |
| `GET /users` | List all users | ✅ Yes |
| `GET /groups` | List all groups | ✅ Yes |
| `GET /computers` | List computers | ✅ Yes |

📖 **Full API Reference:** [user-guide/api.md](user-guide/api.md)

### Configuration Locations

| File | Purpose | Environment |
|------|---------|-------------|
| `.env` | Development secrets | Local dev |
| `.env.example` | Configuration template | All |
| `compose.yml` | Docker services | Dev/Test/Prod |
| `.golangci.yml` | Linter configuration | CI/CD |
| `Makefile` | Build commands | All |

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

| Topic | Documentation | Priority |
|-------|---------------|----------|
| Authentication | [Architecture § Security](development/architecture.md#security-architecture) | 🔴 Critical |
| Session Management | [Security Config](operations/security-configuration.md) | 🔴 Critical |
| LDAP Injection Prevention | [AGENTS.md § Security](../internal/AGENTS.md#security--safety) | 🔴 Critical |
| Secrets Management | [Configuration](user-guide/configuration.md#secrets) | 🔴 Critical |
| Input Validation | [Web AGENTS.md](../internal/web/AGENTS.md#security--safety) | 🟡 Important |

---

## 📊 Quality Metrics

Current project health indicators:

- **Test Coverage:** 80% minimum (90% for ldap_cache, 50% for templates)
- **Linting:** 20+ enabled linters via golangci-lint
- **Technical Debt:** 0 TODO/FIXME markers
- **Documentation Coverage:** ✅ Comprehensive (17 docs + 4 AGENTS.md)
- **CI/CD:** 3 automated workflows (quality, check, docker)

📖 **Details:** [Project Context Report](../claudedocs/project-context-2025-09-30.md)

---

## 🔗 External Resources

### Official Documentation

- **Go:** https://go.dev/doc/
- **Fiber v2:** https://docs.gofiber.io/
- **Templ:** https://templ.guide/
- **TailwindCSS:** https://tailwindcss.com/docs
- **go-ldap:** https://pkg.go.dev/github.com/go-ldap/ldap/v3

### Related Projects

- **simple-ldap-go:** Custom LDAP wrapper (v1.0.3)
- **BBolt:** Embedded key-value database for sessions

---

## 🆘 Getting Help

### Documentation Not Found?

1. **Check AGENTS.md:** AI guidelines may have what you need
2. **Search codebase:** `grep -r "pattern" internal/`
3. **Review tests:** `*_test.go` files show usage examples
4. **Architecture docs:** Comprehensive technical details

### Common Issues

| Problem | Solution | Reference |
|---------|----------|-----------|
| Build fails | `make clean && make setup` | [Setup](development/setup.md) |
| Tests fail | Check LDAP server running | [Docker Dev](DOCKER_DEVELOPMENT.md) |
| Can't connect | Verify .env configuration | [Configuration](user-guide/configuration.md) |
| Performance issues | Check cache settings | [Performance](operations/performance-optimization.md) |

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

*This index is maintained automatically and manually. Last comprehensive update: 2025-09-30*

**📌 Bookmark this page** - it's your hub for all LDAP Manager documentation.