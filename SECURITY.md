# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in ldap-manager, please report it responsibly:

1. **Do NOT** open a public GitHub issue for security vulnerabilities
2. **Use** [GitHub's private vulnerability reporting](https://github.com/netresearch/ldap-manager/security/advisories/new)
3. **Include** a detailed description of the vulnerability and steps to reproduce

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Assessment**: We will assess the vulnerability and determine its severity
- **Fix Timeline**: Critical vulnerabilities will be addressed within 7 days
- **Disclosure**: We will coordinate disclosure timing with the reporter

## Security Measures

This project implements the following security controls:

- **CSRF Protection**: All state-changing operations require valid CSRF tokens
- **Rate Limiting**: Authentication endpoints are rate-limited to prevent brute force
- **Session Security**: Sessions use secure, HTTP-only cookies with configurable expiry
- **Input Validation**: All user input is validated server-side
- **Dependency Scanning**: Automated vulnerability scanning via Trivy and gosec
- **Code Analysis**: Static analysis with golangci-lint and CodeQL

## Security Configuration

See the [README](README.md) for security-related configuration options:

- `--cookie-secure`: Require HTTPS for cookies (recommended for production)
- `--tls-skip-verify`: Disable TLS verification (development only)

Rate limiting is enabled by default (5 failed attempts per 15 minutes triggers a 15-minute block).
