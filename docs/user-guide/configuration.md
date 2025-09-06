# Configuration Reference

Complete guide to configuring LDAP Manager for different environments and use cases.

## Configuration Methods

LDAP Manager supports multiple configuration approaches with the following precedence order:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **`.env.local` file**
4. **`.env` file**
5. **Default values** (lowest priority)

## Required Configuration

These settings must be configured for LDAP Manager to function:

### LDAP Connection Settings

| Setting           | Environment Variable     | CLI Flag              | Description                            | Example                       |
| ----------------- | ------------------------ | --------------------- | -------------------------------------- | ----------------------------- |
| LDAP Server       | `LDAP_SERVER`            | `--ldap-server`       | LDAP server URI with protocol and port | `ldaps://dc1.example.com:636` |
| Base DN           | `LDAP_BASE_DN`           | `--base-dn`           | Base Distinguished Name for searches   | `DC=example,DC=com`           |
| Readonly User     | `LDAP_READONLY_USER`     | `--readonly-user`     | Service account username               | `readonly`                    |
| Readonly Password | `LDAP_READONLY_PASSWORD` | `--readonly-password` | Service account password               | `secure_password123`          |

### LDAP Server URI Format

The LDAP server must be specified as a complete URI:

```bash
# Standard LDAP (unencrypted, port 389)
LDAP_SERVER=ldap://ldap.example.com:389

# Secure LDAP (encrypted, port 636) - Required for Active Directory
LDAP_SERVER=ldaps://dc1.example.com:636

# Alternative secure port
LDAP_SERVER=ldaps://directory.company.org:3269
```

### Base DN Examples

The Base DN defines the root of your directory search scope:

```bash
# Standard domain
LDAP_BASE_DN=DC=example,DC=com

# Organizational unit scope
LDAP_BASE_DN=OU=Users,DC=company,DC=org

# Multi-level domain
LDAP_BASE_DN=DC=subdomain,DC=example,DC=com
```

## Optional Configuration

### Directory Type Settings

| Setting          | Environment Variable | CLI Flag             | Default | Description                                              |
| ---------------- | -------------------- | -------------------- | ------- | -------------------------------------------------------- |
| Active Directory | `LDAP_IS_AD`         | `--active-directory` | `false` | Enable Active Directory specific features and attributes |

**Active Directory Notes:**

- Must use `ldaps://` (secure LDAP) for connections
- Enables AD-specific user attributes (sAMAccountName, userPrincipalName, etc.)
- Optimizes group membership queries for AD schema

### Session Management

| Setting          | Environment Variable | CLI Flag             | Default    | Description                      |
| ---------------- | -------------------- | -------------------- | ---------- | -------------------------------- |
| Persist Sessions | `PERSIST_SESSIONS`   | `--persist-sessions` | `false`    | Store sessions in BBolt database |
| Session Path     | `SESSION_PATH`       | `--session-path`     | `db.bbolt` | Path to session database file    |
| Session Duration | `SESSION_DURATION`   | `--session-duration` | `30m`      | Session timeout duration         |

#### Session Duration Format

Use Go duration syntax:

```bash
# Minutes
SESSION_DURATION=30m

# Hours
SESSION_DURATION=2h

# Mixed units
SESSION_DURATION=1h30m

# Days (as hours)
SESSION_DURATION=24h
```

#### Session Storage Options

**Memory Storage (Default)**

- Fast performance
- No persistent storage
- Sessions lost on application restart
- Suitable for development and testing

**BBolt Database Storage**

- Persistent across restarts
- Slightly slower performance
- Requires disk space and file permissions
- Recommended for production

### Logging Configuration

| Setting   | Environment Variable | CLI Flag      | Default | Description             |
| --------- | -------------------- | ------------- | ------- | ----------------------- |
| Log Level | `LOG_LEVEL`          | `--log-level` | `info`  | Logging verbosity level |

**Valid log levels** (in order of verbosity):

- `trace` - Extremely detailed debugging
- `debug` - Detailed debugging information
- `info` - General informational messages
- `warn` - Warning messages
- `error` - Error messages only
- `fatal` - Fatal errors only
- `panic` - Panic level messages

### Server Configuration

| Setting        | Environment Variable | CLI Flag   | Default | Description                    |
| -------------- | -------------------- | ---------- | ------- | ------------------------------ |
| Listen Address | `LISTEN_ADDR`        | `--listen` | `:3000` | Server listen address and port |

## Configuration Examples

### Standard LDAP Server

For traditional LDAP servers like OpenLDAP:

```bash
# .env.local
LDAP_SERVER=ldap://openldap.company.local:389
LDAP_BASE_DN=DC=company,DC=local
LDAP_READONLY_USER=cn=readonly,dc=company,dc=local
LDAP_READONLY_PASSWORD=readonly_password
LDAP_IS_AD=false

# Optional settings
LOG_LEVEL=info
PERSIST_SESSIONS=true
SESSION_DURATION=1h
```

### Active Directory Domain Controller

For Microsoft Active Directory:

```bash
# .env.local
LDAP_SERVER=ldaps://dc1.ad.example.com:636
LDAP_BASE_DN=DC=ad,DC=example,DC=com
LDAP_READONLY_USER=readonly@ad.example.com
LDAP_READONLY_PASSWORD=Complex_Password123!
LDAP_IS_AD=true

# Production settings
LOG_LEVEL=warn
PERSIST_SESSIONS=true
SESSION_PATH=/data/sessions.bbolt
SESSION_DURATION=30m
```

### Development Environment

For local development and testing:

```bash
# .env.local
LDAP_SERVER=ldap://localhost:389
LDAP_BASE_DN=DC=dev,DC=local
LDAP_READONLY_USER=cn=admin,dc=dev,dc=local
LDAP_READONLY_PASSWORD=admin
LDAP_IS_AD=false

# Development settings
LOG_LEVEL=debug
PERSIST_SESSIONS=true
SESSION_PATH=dev-session.bbolt
SESSION_DURATION=8h
```

### High-Availability Production

For production deployments with load balancing:

```bash
# .env.local
LDAP_SERVER=ldaps://ldap-cluster.prod.company.com:636
LDAP_BASE_DN=DC=prod,DC=company,DC=com
LDAP_READONLY_USER=svc_ldap_readonly
LDAP_READONLY_PASSWORD=very_secure_password_here
LDAP_IS_AD=true

# Production optimization
LOG_LEVEL=error
PERSIST_SESSIONS=true
SESSION_PATH=/persistent/sessions.bbolt
SESSION_DURATION=15m
LISTEN_ADDR=:8080
```

## Docker Configuration

### Environment Variables

```bash
docker run -d \
  -e LDAP_SERVER=ldaps://dc1.example.com:636 \
  -e LDAP_BASE_DN="DC=example,DC=com" \
  -e LDAP_READONLY_USER=readonly \
  -e LDAP_READONLY_PASSWORD=password \
  -e LDAP_IS_AD=true \
  -e LOG_LEVEL=info \
  -e PERSIST_SESSIONS=true \
  -e SESSION_PATH=/data/sessions.bbolt \
  -v /host/data:/data \
  -p 3000:3000 \
  ghcr.io/netresearch/ldap-manager
```

### Docker Compose

```yaml
version: "3.8"

services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    environment:
      LDAP_SERVER: ldaps://dc1.example.com:636
      LDAP_BASE_DN: DC=example,DC=com
      LDAP_READONLY_USER: readonly
      LDAP_READONLY_PASSWORD: password
      LDAP_IS_AD: "true"
      LOG_LEVEL: info
      PERSIST_SESSIONS: "true"
      SESSION_PATH: /data/sessions.bbolt
      SESSION_DURATION: 30m
    volumes:
      - ./data:/data
      - /etc/ssl/certs:/etc/ssl/certs:ro
    ports:
      - "3000:3000"
    restart: unless-stopped
```

### Environment File in Docker

```bash
# Create .env file
cat > .env << EOF
LDAP_SERVER=ldaps://dc1.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_READONLY_USER=readonly
LDAP_READONLY_PASSWORD=password
LDAP_IS_AD=true
EOF

# Use with Docker
docker run -d --env-file .env -p 3000:3000 ghcr.io/netresearch/ldap-manager
```

## Security Configuration

### SSL/TLS for LDAPS

#### Custom Certificate Authority

```bash
# Mount custom CA certificates
docker run -d \
  -v /path/to/ca-certificates:/etc/ssl/certs:ro \
  # ... other options
  ghcr.io/netresearch/ldap-manager
```

#### System Certificate Store

Add certificates to the system trust store:

```bash
# Linux
sudo cp ldap-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates

# Then restart LDAP Manager
```

### Service Account Security

**Principle of Least Privilege:**

- Create dedicated service account for LDAP Manager
- Grant only read permissions to necessary directory branches
- Use strong, unique passwords
- Regularly rotate credentials

**Active Directory Example:**

```powershell
# Create service account
New-ADUser -Name "LDAP-Manager-Readonly" -UserPrincipalName "ldap-readonly@domain.com" -AccountPassword (ConvertTo-SecureString "StrongPassword123!" -AsPlainText -Force) -Enabled $true

# Grant read permissions to specific OUs
dsacls "OU=Users,DC=domain,DC=com" /G "LDAP-Manager-Readonly:GR"
dsacls "OU=Groups,DC=domain,DC=com" /G "LDAP-Manager-Readonly:GR"
```

### Session Security

**Cookie Security:**

- HTTP-only cookies (XSS protection)
- SameSite=Strict policy (CSRF protection)
- Secure flag when using HTTPS

**Session Timeout:**

- Balance security vs. usability
- Shorter timeouts for sensitive environments
- Consider user activity patterns

## Performance Configuration

### Connection Pooling

LDAP Manager automatically manages connection pooling. Monitor performance with debug logging:

```bash
LOG_LEVEL=debug
```

### Cache Settings

LDAP directory data is cached automatically with 30-second refresh intervals. This is currently not configurable but provides optimal balance of performance and data freshness.

### Resource Limits

For Docker deployments, set appropriate resource limits:

```yaml
services:
  ldap-manager:
    # ... other config
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "0.5"
        reservations:
          memory: 256M
          cpus: "0.25"
```

## Configuration Validation

LDAP Manager validates configuration on startup and provides clear error messages:

### Common Validation Errors

```bash
# Missing required settings
FATAL the option --ldap-server is required
FATAL the option --base-dn is required

# Invalid format
FATAL could not parse log level: invalid level "verbose"
FATAL invalid LDAP server URI: must start with ldap:// or ldaps://

# Duration parsing errors
FATAL could not parse environment variable "SESSION_DURATION" (containing "30minutes") as duration
```

### Testing Configuration

```bash
# Test configuration without starting server
./ldap-manager --help

# Start with debug logging to verify settings
LOG_LEVEL=debug ./ldap-manager

# Test LDAP connectivity
ldapsearch -H ldaps://dc1.example.com:636 -D readonly -w password -b "DC=example,DC=com" "(objectClass=*)" dn
```

## Troubleshooting

### LDAP Connection Issues

**Cannot connect to LDAP server:**

```bash
# Test network connectivity
telnet dc1.example.com 636

# Test LDAPS certificate
openssl s_client -connect dc1.example.com:636 -showcerts

# Verify credentials
ldapsearch -H ldaps://dc1.example.com:636 -D readonly -w password -b "DC=example,DC=com" -s base
```

**Certificate verification failed:**

- Add server certificate to system trust store
- Use appropriate certificate volume mounts in Docker
- Verify certificate chain is complete

### Authentication Problems

**Invalid credentials:**

- Test readonly user credentials with ldapsearch
- Verify user format (DN vs. UPN for Active Directory)
- Check account is not locked or expired

**Permission denied:**

- Verify service account has read access to Base DN
- Check organizational unit permissions
- Ensure account can read user/group attributes

### Session Issues

**Sessions not persisting:**

- Verify `PERSIST_SESSIONS=true` is set
- Check session file path permissions
- Monitor disk space for session storage

**Session timeout too short/long:**

- Adjust `SESSION_DURATION` value
- Use appropriate Go duration format
- Consider user workflow requirements

### Performance Issues

**Slow LDAP queries:**

- Enable debug logging to identify slow operations
- Check LDAP server performance and indexing
- Consider using LDAP replica for read operations
- Monitor network latency to LDAP server

**High memory usage:**

- Monitor session count and storage
- Check for connection leaks in debug logs
- Consider shorter session timeouts
- Use resource limits in Docker

## Environment-Specific Configurations

### Development

```bash
# .env.local - Development
LDAP_SERVER=ldap://localhost:389
LDAP_BASE_DN=DC=dev,DC=local
LDAP_READONLY_USER=cn=admin,dc=dev,dc=local
LDAP_READONLY_PASSWORD=admin
LOG_LEVEL=debug
SESSION_DURATION=8h
```

### Staging

```bash
# .env.local - Staging
LDAP_SERVER=ldaps://ldap-staging.company.com:636
LDAP_BASE_DN=DC=staging,DC=company,DC=com
LDAP_READONLY_USER=readonly@staging.company.com
LDAP_READONLY_PASSWORD=${STAGING_LDAP_PASSWORD}
LDAP_IS_AD=true
LOG_LEVEL=info
PERSIST_SESSIONS=true
SESSION_DURATION=1h
```

### Production

```bash
# .env.local - Production
LDAP_SERVER=ldaps://ldap.company.com:636
LDAP_BASE_DN=DC=company,DC=com
LDAP_READONLY_USER=svc_ldap_readonly@company.com
LDAP_READONLY_PASSWORD=${PROD_LDAP_PASSWORD}
LDAP_IS_AD=true
LOG_LEVEL=warn
PERSIST_SESSIONS=true
SESSION_PATH=/data/sessions.bbolt
SESSION_DURATION=30m
LISTEN_ADDR=:8080
```

For advanced deployment scenarios, see the [Deployment Guide](../operations/deployment.md).
