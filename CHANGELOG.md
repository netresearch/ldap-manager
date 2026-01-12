# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- CONTRIBUTING.md with contribution guidelines
- CHANGELOG.md (this file)
- Release labeler workflow for automatic PR/issue tagging

---

## [v1.1.1] - 2026-01-12

### Fixed

- Restored Docker HEALTHCHECK with built-in `--health-check` flag ([#385](https://github.com/netresearch/ldap-manager/pull/385))
  - Added `--health-check` CLI flag that performs HTTP health check against `/health/live`
  - Works with distroless images (no shell/curl required)

### Changed

- Replaced go-mutesting with gremlins for mutation testing ([#379](https://github.com/netresearch/ldap-manager/pull/379))

### Testing

- Added mutation-killing tests for retry package ([#380](https://github.com/netresearch/ldap-manager/pull/380))
- Added mutation-killing tests for Parse function ([#382](https://github.com/netresearch/ldap-manager/pull/382))

---

## [v1.1.0] - 2025-12-29

### Added

- **Enhanced Detail Views**: Email, description, copy-to-clipboard for users/groups/computers ([#373](https://github.com/netresearch/ldap-manager/pull/373))
- **GUI Rework**: Theme switching (light/dark/system), density controls, accessibility improvements ([#370](https://github.com/netresearch/ldap-manager/pull/370))
- **Client-side Search**: Real-time search filter for users, groups, and computers lists
- **Searchable Combobox**: Filterable dropdown for user/group selection
- **Rate Limiting**: Rate limiting for authentication endpoints
- **Graceful Shutdown**: Proper signal handling with context propagation
- **Retry Logic**: Exponential backoff for LDAP operations
- **TLS Skip Verify**: Support for self-signed certificates
- **WCAG Compliance**: Title attributes and accessibility improvements

### Fixed

- LDAP DN handling for special characters ([#371](https://github.com/netresearch/ldap-manager/pull/371))
- Invalid UTF-8 handling in URL parsing ([#369](https://github.com/netresearch/ldap-manager/pull/369))
- Data race in template cache Get method
- CSS cache busting for old versions

### Changed

- Updated Go to 1.25.x
- Updated simple-ldap-go to v1.6.0
- Updated Tailwind CSS to v4.1.x

---

## [v1.0.8] - 2025-02-14

### Fixed

- CSS build process

---

## [v1.0.7] - 2025-01-31

### Changed

- Dependency updates (pnpm, tailwindcss, prettier)

---

## [v1.0.6] - 2024-11-13

### Changed

- Updated Go to 1.23
- Various dependency updates

---

## Earlier Releases

For releases prior to v1.0.6, see the [GitHub Releases](https://github.com/netresearch/ldap-manager/releases) page.
