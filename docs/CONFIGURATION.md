# LDAP Manager Configuration Guide

## Overview
LDAP Manager supports configuration through environment variables, `.env` files, and command-line flags. All settings can be specified using any of these methods, with command-line flags taking highest precedence.

## Configuration Precedence
1. **Command-line flags** (highest priority)
2. **Environment variables** 
3. **`.env.local` file**
4. **`.env` file**
5. **Default values** (lowest priority)

## Required Configuration

### LDAP Connection
These settings are mandatory for connecting to your LDAP directory:

| Setting | Environment Variable | CLI Flag | Description |
|---------|---------------------|----------|-------------|
| LDAP Server | `LDAP_SERVER` | `--ldap-server` | LDAP server URI (must start with `ldap://` or `ldaps://`) |
| Base DN | `LDAP_BASE_DN` | `--base-dn` | Base Distinguished Name for LDAP searches |
| Readonly User | `LDAP_READONLY_USER` | `--readonly-user` | Username for read-only LDAP access |
| Readonly Password | `LDAP_READONLY_PASSWORD` | `--readonly-password` | Password for readonly user |

### LDAP Server Examples
```bash
# Standard LDAP (port 389)
LDAP_SERVER=ldap://ldap.example.com:389

# Secure LDAP (port 636) - Required for Active Directory
LDAP_SERVER=ldaps://dc1.example.com:636

# Alternative secure port
LDAP_SERVER=ldaps://directory.company.org:3269
```

### Base DN Examples
```bash
# Standard domain
LDAP_BASE_DN=DC=example,DC=com

# Subdomain
LDAP_BASE_DN=DC=users,DC=internal,DC=example,DC=com

# Organizational unit focus
LDAP_BASE_DN=OU=Corporate,DC=company,DC=org
```

## Optional Configuration

### Active Directory Support
| Setting | Environment Variable | CLI Flag | Default | Description |
|---------|---------------------|----------|---------|-------------|
| Active Directory | `LDAP_IS_AD` | `--active-directory` | `false` | Enable Active Directory specific features |

**Note**: When using Active Directory, you **must** use `ldaps://` (secure LDAP) for the server connection.

### Session Management
| Setting | Environment Variable | CLI Flag | Default | Description |
|---------|---------------------|----------|---------|-------------|
| Persist Sessions | `PERSIST_SESSIONS` | `--persist-sessions` | `false` | Store sessions in BBolt database |
| Session Path | `SESSION_PATH` | `--session-path` | `db.bbolt` | Path to session database file |
| Session Duration | `SESSION_DURATION` | `--session-duration` | `30m` | Session timeout duration |

### Session Duration Format
Use Go duration format for session timeouts:
```bash
# Minutes
SESSION_DURATION=30m

# Hours  
SESSION_DURATION=2h

# Mixed units
SESSION_DURATION=1h30m

# Days (24 hour format)
SESSION_DURATION=24h
```

### Logging Configuration
| Setting | Environment Variable | CLI Flag | Default | Description |
|---------|---------------------|----------|---------|-------------|
| Log Level | `LOG_LEVEL` | `--log-level` | `info` | Logging verbosity level |

**Valid log levels**: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`

## Configuration Methods

### Method 1: Environment Variables
Create a `.env.local` file in the project root:

```bash
# .env.local
LDAP_SERVER=ldaps://dc1.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_READONLY_USER=readonly
LDAP_READONLY_PASSWORD=secretpassword
LDAP_IS_AD=true

# Optional settings
LOG_LEVEL=debug
PERSIST_SESSIONS=true
SESSION_DURATION=1h
```

### Method 2: Command-Line Flags
```bash
./ldap-manager \
  --ldap-server ldaps://dc1.example.com:636 \
  --base-dn "DC=example,DC=com" \
  --readonly-user readonly \
  --readonly-password secretpassword \
  --active-directory \
  --log-level debug \
  --persist-sessions \
  --session-duration 1h
```

### Method 3: Docker Environment
```bash
docker run \
  -e LDAP_SERVER=ldaps://dc1.example.com:636 \
  -e LDAP_BASE_DN="DC=example,DC=com" \
  -e LDAP_READONLY_USER=readonly \
  -e LDAP_READONLY_PASSWORD=secretpassword \
  -e LDAP_IS_AD=true \
  -e LOG_LEVEL=info \
  -p 3000:3000 \
  ghcr.io/netresearch/ldap-manager
```

## Configuration Examples

### Standard LDAP Server
```bash
# .env.local
LDAP_SERVER=ldap://openldap.company.local:389
LDAP_BASE_DN=DC=company,DC=local
LDAP_READONLY_USER=cn=readonly,dc=company,dc=local
LDAP_READONLY_PASSWORD=readonly123
LDAP_IS_AD=false
```

### Active Directory Domain Controller
```bash
# .env.local
LDAP_SERVER=ldaps://dc1.ad.example.com:636
LDAP_BASE_DN=DC=ad,DC=example,DC=com
LDAP_READONLY_USER=readonly@ad.example.com
LDAP_READONLY_PASSWORD=ComplexPassword123!
LDAP_IS_AD=true
```

### Development Setup
```bash
# .env.local
LDAP_SERVER=ldap://localhost:389
LDAP_BASE_DN=DC=dev,DC=local
LDAP_READONLY_USER=cn=admin,dc=dev,dc=local
LDAP_READONLY_PASSWORD=admin
LOG_LEVEL=debug
PERSIST_SESSIONS=true
SESSION_PATH=dev-session.bbolt
```

## Configuration Validation

The application validates configuration on startup and will exit with an error if:

- Required settings are missing or empty
- LDAP server URI format is invalid
- Log level is not recognized  
- Session duration cannot be parsed
- Session path is required but not provided when `PERSIST_SESSIONS=true`

### Example Validation Errors
```
FATAL the option --ldap-server is required
FATAL could not parse log level: invalid level "verbose"
FATAL could not parse environment variable "SESSION_DURATION" (containing "30minutes") as duration
```

## Security Considerations

### Credential Management
- **Never commit** `.env.local` files to version control
- Use strong passwords for readonly LDAP accounts
- Consider using dedicated service accounts with minimal privileges
- Regularly rotate LDAP service account passwords

### LDAPS (Secure LDAP)
- **Required** for Active Directory connections
- **Recommended** for all production deployments
- Ensure valid SSL certificates or configure certificate trust
- Default port 636 for LDAPS, 389 for plain LDAP

### Session Security
- Sessions use HTTP-only cookies with SameSite=Strict
- Session duration should be appropriate for your security requirements
- Persistent sessions store data encrypted in BBolt database
- Memory sessions are lost on application restart

## Troubleshooting

### Common Configuration Issues

**LDAP Connection Failed**
- Verify server hostname and port accessibility
- Check firewall rules for LDAP ports (389/636)
- Validate LDAPS certificate if using secure connection
- Test LDAP connectivity with tools like `ldapsearch`

**Authentication Errors**
- Verify readonly user credentials are correct
- Check user has sufficient permissions to read directory
- Ensure Base DN covers the user search scope
- For AD: verify user format (UPN vs DN format)

**Session Issues**
- Check session duration format matches Go duration syntax
- Verify session database file permissions when persisting
- Ensure sufficient disk space for session storage

### Debug Configuration
Enable debug logging to troubleshoot configuration issues:
```bash
LOG_LEVEL=debug ./ldap-manager --help
```

This will show:
- Configuration value sources (env vs flag vs default)
- LDAP connection attempts
- Session initialization details
- Cache refresh operations

## Performance Tuning

### Cache Refresh
- LDAP cache refreshes every 30 seconds automatically
- Refresh frequency is not configurable (hardcoded)
- Monitor cache refresh performance in debug logs
- Large directories may benefit from dedicated LDAP replicas

### Session Storage
- **Memory sessions**: Faster but lost on restart
- **BBolt sessions**: Persistent but with disk I/O overhead
- Choose based on your high availability requirements
- BBolt database grows with active session count

### Connection Pooling
- LDAP connections are pooled automatically
- Each user session creates individual LDAP connections
- Monitor concurrent session limits based on LDAP server capacity