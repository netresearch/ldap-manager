# OpenSSF Best Practices badge — evidence pack

Registration at [bestpractices.dev](https://www.bestpractices.dev) needs a GitHub login, so a maintainer has to create
the project entry. This page collects the answers the questionnaire asks for, so filling it in is a lookup rather than
research. Scorecard alert 116 (`CIIBestPracticesID`) clears once the badge exists.

Two netresearch projects already hold a passing badge — `t3x-cowriter` and `t3x-contexts_geolocation` — so their
entries are useful templates.

## Project identity

| Question | Answer |
| --- | --- |
| Project name | LDAP Manager |
| Homepage and repository | <https://github.com/netresearch/ldap-manager> |
| Description | Web-based LDAP administration interface for users, groups and computers, for Active Directory and OpenLDAP |
| Programming language | Go |
| License | MIT, [LICENSE](../../LICENSE) — an OSI-approved FLOSS licence, machine-readable in the repository root |

## Documentation

| Question | Evidence |
| --- | --- |
| Basic user documentation | [README](../../README.md), [docs/user-guide/](../user-guide/) — installation, configuration, API |
| Interface documentation | [docs/user-guide/api.md](../user-guide/api.md), [docs/development/go-doc-reference.md](go-doc-reference.md) |
| Architecture / design | [docs/development/architecture.md](architecture.md), decision records in [docs/adr/](../adr/) |
| Quick start | README "Quick Start (local test run)" |

## Change control and releases

| Question | Evidence |
| --- | --- |
| Public version-controlled repository | GitHub, full history |
| Unique version numbering | Semantic Versioning; git tags `vX.Y.Z` |
| Release notes | [CHANGELOG.md](../../CHANGELOG.md) plus per-release GitHub notes |
| Release process | Signed tag triggers `.github/workflows/release.yml`; binaries, container image and attestations are published |

## Reporting

| Question | Evidence |
| --- | --- |
| Bug reporting process | GitHub Issues, <https://github.com/netresearch/ldap-manager/issues> |
| Vulnerability reporting process | [SECURITY.md](../../SECURITY.md) — GitHub private vulnerability reporting |
| Private reporting channel | Yes, GitHub Security Advisories |
| Response time | Stated in SECURITY.md |

## Quality

| Question | Evidence |
| --- | --- |
| Working build system | `make build`, `go build ./...` |
| Automated test suite | `go test -race ./...`; unit, integration against an OpenLDAP container, fuzz and Playwright e2e tiers |
| Tests run on every change | `.github/workflows/ci.yml` on every pull request and push |
| Policy: tests added for new functionality | [CONTRIBUTING.md](../../CONTRIBUTING.md) and the AGENTS.md files |
| Enforced coverage floor | `coverage-threshold: 77` in `.github/workflows/ci.yml`, enforced by CI |
| Warning flags enabled | golangci-lint with a strict config; `go vet`; both gate CI |

## Security

| Question | Evidence |
| --- | --- |
| Static analysis | golangci-lint, gosec, CodeQL, SonarCloud — all in CI |
| Dynamic analysis | Go fuzzing (`Fuzz (corpus replay)` job), Playwright e2e |
| Dependency scanning | Dependabot and Renovate; `govulncheck`; Trivy on the container image |
| Secrets scanning | betterleaks on every push and pull request, plus a `detect-secrets` pre-commit hook |
| Supply-chain hardening | `step-security/harden-runner` in workflows; third-party actions pinned by SHA; base images pinned by digest |
| Delivery over HTTPS | GitHub and GHCR |
| Crypto | TLS for LDAPS and for the web interface behind a terminator; cookies default to `Secure`; no home-grown cryptography |

## Gaps to settle before applying

1. **No `CODE_OF_CONDUCT.md`.** Not required for the passing level, required for silver. The other netresearch
   projects carry one; copying theirs is the cheapest path.
2. **No issue or pull-request templates** under `.github/`. Not required, but the form asks how reports are guided.
3. **`make help` lists no targets** — its `awk` matches `target: ## text` while the Makefile writes `## Name: text`
   above each rule. Cosmetic, but "documentation of how to contribute" points at it.
