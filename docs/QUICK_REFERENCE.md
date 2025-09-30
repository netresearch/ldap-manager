# LDAP Manager - Quick Reference Card

**Version:** 1.0.8 | **Last Updated:** 2025-09-30

Instant-access information for daily development and operations.

---

## 🚀 Common Commands

### Development

| Command | Purpose | Time |
|---------|---------|------|
| `make help` | Show all available targets | <1s |
| `make setup` | Install all dependencies (one-time) | ~2min |
| `make dev` | Start hot-reload dev server | ~5s |
| `make build` | Build production binary | ~10s |
| `make test` | Run full test suite with coverage | ~30s |
| `make test-quick` | Quick tests without coverage | ~15s |
| `make lint` | Run all linters | ~20s |
| `make fix` | Auto-fix formatting issues | ~5s |
| `make check` | Quality gate (lint + test) | ~45s |
| `make clean` | Remove artifacts and caches | ~2s |

### Docker

| Command | Purpose |
|---------|---------|
| `make docker-dev` | Full containerized dev environment |
| `make docker-test` | Run tests in container |
| `make docker-lint` | Run linter in container |
| `make docker-shell` | Open shell in dev container |
| `docker compose --profile dev up` | Start dev profile |
| `docker compose --profile test run --rm ldap-manager-test` | Run tests |

### Frontend Assets

| Command | Purpose |
|---------|---------|
| `pnpm build:assets` | Build CSS + templates |
| `pnpm css:build:prod` | Minified CSS with purging |
| `pnpm templ:build` | Generate Go from .templ files |
| `pnpm dev` | Watch all (CSS + templates + Go) |

---

## 📁 Key Files

### Configuration

| File | Purpose | Modify? |
|------|---------|---------|
| `.env` | Development secrets (gitignored) | ✅ Yes |
| `.env.example` | Configuration template | ⚠️ Update when adding vars |
| `.envrc` | direnv environment setup | ⚠️ Rarely |
| `compose.yml` | Docker services (3 profiles) | ⚠️ Rarely |
| `.golangci.yml` | Linter configuration | ⚠️ Rarely |
| `.testcoverage.yml` | Coverage thresholds | ⚠️ Rarely |
| `Makefile` | Build commands | ⚠️ Rarely |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | Project overview + quick start |
| `docs/INDEX.md` | 📌 Master documentation hub |
| `docs/API_REFERENCE.md` | Complete endpoint reference |
| `AGENTS.md` (root) | AI assistant global rules |
| `internal/AGENTS.md` | Go best practices |
| `internal/web/AGENTS.md` | Web handler patterns |

### Source Code

| Package | Purpose | Coverage |
|---------|---------|----------|
| `cmd/ldap-manager/` | Application entry point | N/A |
| `internal/ldap/` | LDAP client & connection pool | 75% |
| `internal/ldap_cache/` | Caching layer | **90%** |
| `internal/options/` | Configuration parsing | 75% |
| `internal/version/` | Build version info | N/A |
| `internal/web/` | HTTP handlers & templates | 75% |

---

## 🔍 Troubleshooting

### Build/Test Issues

| Problem | Quick Fix | Details |
|---------|-----------|---------|
| Build fails | `make clean && make setup` | [Setup Guide](development/setup.md) |
| Tests fail | Check LDAP server: `docker compose ps` | [Docker Dev](DOCKER_DEVELOPMENT.md) |
| Lint fails | `make fix` first, then `make lint` | [AGENTS.md](../AGENTS.md#minimal-pre-commit-checks) |
| Templates not compiling | `pnpm templ:build` | [Web AGENTS.md](../internal/web/AGENTS.md#template-development) |
| CSS not updating | `pnpm css:build` | [Web AGENTS.md](../internal/web/AGENTS.md#frontend-assets--styling) |

### Runtime Issues

| Problem | Quick Fix |
|---------|-----------|
| Can't connect to LDAP | Check `.env` LDAP settings |
| Session expires immediately | Check `SESSION_DURATION` env var |
| CSRF errors | Clear cookies and re-login |
| 404 on `/static/*` | Run `pnpm build:assets` |
| Slow performance | Check `/debug/cache` and `/debug/ldap-pool` |

### Docker Issues

| Problem | Quick Fix |
|---------|-----------|
| Port already in use | Change port in `compose.yml` or stop conflicting service |
| LDAP server not starting | `docker compose logs openldap` |
| Volume permission errors | Run with correct user or fix volume permissions |
| Out of disk space | `docker system prune -a` (⚠️ deletes all unused images) |

---

## 🎯 Development Workflow

### Starting New Feature

```bash
# 1. Create feature branch
git checkout -b feature/user-management-enhancements

# 2. Start development environment
make dev                    # Terminal 1: Hot reload server

# 3. Make changes
# Edit code in internal/web/, internal/ldap/, etc.
# Templates auto-rebuild, CSS auto-rebuilds, Go auto-restarts

# 4. Test incrementally
make test-quick            # Run during development

# 5. Pre-commit quality check
make check                 # Must pass before commit

# 6. Commit with conventional format
git add .
git commit -m "feat(users): add bulk user import functionality"

# 7. Push and create PR
git push origin feature/user-management-enhancements
```

### Fixing Bug

```bash
# 1. Write failing test first (TDD)
# Add test in *_test.go file

# 2. Run test to confirm failure
go test ./internal/ldap_cache/ -v -run TestSpecificCase

# 3. Fix the code
# Edit source file

# 4. Run test to confirm fix
go test ./internal/ldap_cache/ -v -run TestSpecificCase

# 5. Run full test suite
make test

# 6. Commit fix
git commit -m "fix(ldap): resolve connection pool deadlock"
```

### Adding New Endpoint

```bash
# 1. Add handler in internal/web/
# Follow patterns in users.go, groups.go

# 2. Register route in server.go setupRoutes()
# Add to appropriate group (public/protected/cacheable)

# 3. Create template in internal/web/templates/
# Use .templ file format

# 4. Build templates
pnpm templ:build

# 5. Test endpoint manually
curl -v http://localhost:3000/new-endpoint

# 6. Add tests in *_test.go

# 7. Run quality checks
make check
```

---

## 🔐 Security Checklist

### Before Production Deployment

- [ ] Set strong `SESSION_SECRET` in environment
- [ ] Enable HTTPS with valid certificate
- [ ] Configure `LDAP_USE_TLS=true` for LDAPS
- [ ] Review and update LDAP service account permissions
- [ ] Set `LOG_LEVEL=info` or `warn` (not `debug`)
- [ ] Enable persistent sessions with BBolt (`PERSIST_SESSIONS=true`)
- [ ] Configure proper CORS if needed
- [ ] Review CSP headers in `server.go`
- [ ] Run security scan: `make lint-security`
- [ ] Test authentication flow end-to-end

---

## 📊 Quality Gates

### Pre-Commit Requirements

```bash
# Must all pass:
make lint           # ✅ All linters pass
make test           # ✅ 80%+ coverage maintained
go build ./...      # ✅ Compiles successfully
pnpm build:assets   # ✅ Assets build successfully
```

### CI/CD Pipeline

**quality.yml** (runs on push/PR):
- Go linting (golangci-lint)
- Security scanning (govulncheck)
- Secret detection
- Code formatting validation

**check.yml** (runs on push/PR):
- Full test suite
- Coverage validation (80% minimum)
- Race detection

**docker.yml** (runs on tag):
- Docker image build
- Multi-arch support
- Registry push

---

## 🌐 Environment Variables

### Required

| Variable | Purpose | Example |
|----------|---------|---------|
| `LDAP_SERVER` | LDAP server URL | `ldaps://dc1.example.com:636` |
| `LDAP_BASE_DN` | Search base | `DC=example,DC=com` |
| `LDAP_READONLY_USER` | Bind username | `cn=readonly,dc=example,dc=com` |
| `LDAP_READONLY_PASSWORD` | Bind password | `SecurePassword123` |

### Optional

| Variable | Default | Purpose |
|----------|---------|---------|
| `PORT` | `3000` | HTTP listen port |
| `LOG_LEVEL` | `info` | Logging level (debug/info/warn/error) |
| `SESSION_DURATION` | `30m` | Session timeout |
| `PERSIST_SESSIONS` | `false` | Use BBolt for persistent sessions |
| `SESSION_PATH` | `/data/session.bbolt` | BBolt database path |
| `LDAP_IS_AD` | `false` | Active Directory mode |

---

## 📖 Quick Links

### Documentation

- **📌 Master Index:** [docs/INDEX.md](INDEX.md)
- **API Reference:** [docs/API_REFERENCE.md](API_REFERENCE.md)
- **Architecture:** [docs/development/architecture.md](development/architecture.md)
- **Setup Guide:** [docs/development/setup.md](development/setup.md)
- **Operations:** [docs/operations/deployment.md](operations/deployment.md)

### AGENTS.md (AI Guidelines)

- **Global:** [AGENTS.md](../AGENTS.md)
- **CLI:** [cmd/AGENTS.md](../cmd/AGENTS.md)
- **Core:** [internal/AGENTS.md](../internal/AGENTS.md)
- **Web:** [internal/web/AGENTS.md](../internal/web/AGENTS.md)

### External Resources

- **Fiber Docs:** https://docs.gofiber.io/
- **Templ Guide:** https://templ.guide/
- **Go LDAP:** https://pkg.go.dev/github.com/go-ldap/ldap/v3

---

## 💡 Pro Tips

### Performance

- Monitor `/debug/cache` for cache hit rates (target: >80%)
- Monitor `/debug/ldap-pool` for connection pool health
- Use `make benchmark` to profile critical paths
- Template cache invalidates on POST - design API accordingly

### Development

- Use `make dev` for best development experience (hot reload everything)
- Reference AGENTS.md nearest to your work for context-specific patterns
- Run `make test-quick` frequently during development
- Run `make check` before pushing to catch issues early

### Debugging

- Set `LOG_LEVEL=debug` to see detailed logging
- Check `/health/ready` for dependency status
- Use `/debug/cache` and `/debug/ldap-pool` for performance insights
- Add `log.Debug().Interface("data", data).Msg("debug")` for structured logging

---

## 🆘 Getting Help

**Quick:** Check [docs/INDEX.md](INDEX.md) for navigation to all documentation

**Detailed:**
- **User issues:** [docs/user-guide/](user-guide/)
- **Dev questions:** [docs/development/](development/)
- **Ops problems:** [docs/operations/](operations/)
- **AI assistance:** Nearest `AGENTS.md` file

---

*Keep this card bookmarked for instant access to daily development information.*