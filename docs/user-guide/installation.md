# Installation Guide

This guide covers all methods for installing and running LDAP Manager in different environments.

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **Memory**: Minimum 512MB RAM (1GB+ recommended for production)
- **Storage**: 100MB for application, additional space for session storage if enabled
- **Network**: Access to LDAP server (ports 389/636)

### LDAP Server Requirements

- **LDAP v3 compatible server** (OpenLDAP, Active Directory, etc.)
- **Network connectivity** to LDAP server on standard ports:
  - Port 389 for LDAP (plain text)
  - Port 636 for LDAPS (secure)
- **Service account** with read access to the directory
- **Base DN** configured for your directory structure

## Installation Methods

### Method 1: Docker (Recommended)

Docker provides the easiest deployment option with all dependencies included.

#### Quick Start with Docker

```bash
docker run -d \
  --name ldap-manager \
  -p 3000:3000 \
  -e LDAP_SERVER=ldaps://dc1.example.com:636 \
  -e LDAP_BASE_DN="DC=example,DC=com" \
  -e LDAP_READONLY_USER=readonly \
  -e LDAP_READONLY_PASSWORD=your_password \
  -e LDAP_IS_AD=true \
  ghcr.io/netresearch/ldap-manager
```

#### Docker Compose (Production)

Create a `compose.yml` file:

```yaml

services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    container_name: ldap-manager
    ports:
      - "3000:3000"
    environment:
      LDAP_SERVER: ldaps://dc1.example.com:636
      LDAP_BASE_DN: DC=example,DC=com
      LDAP_READONLY_USER: readonly
      LDAP_READONLY_PASSWORD: your_password
      LDAP_IS_AD: true
      LOG_LEVEL: info
      PERSIST_SESSIONS: true
      SESSION_PATH: /data/sessions.bbolt
      SESSION_DURATION: 30m
    volumes:
      - ./data:/data
      - /etc/ssl/certs:/etc/ssl/certs:ro  # For custom SSL certificates
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/"]
      interval: 30s
      timeout: 10s
      retries: 3
```

Start with:

```bash
docker compose up -d
```

### Method 2: Pre-built Binaries

Download the latest release from the GitHub releases page.

#### Linux/macOS

```bash
# Download the latest release (replace with actual version)
wget https://github.com/netresearch/ldap-manager/releases/download/v1.0.0/ldap-manager-linux-amd64.tar.gz

# Extract
tar -xzf ldap-manager-linux-amd64.tar.gz

# Make executable
chmod +x ldap-manager

# Run with configuration
./ldap-manager \
  --ldap-server ldaps://dc1.example.com:636 \
  --base-dn "DC=example,DC=com" \
  --readonly-user readonly \
  --readonly-password your_password \
  --active-directory
```

#### Windows

1. Download `ldap-manager-windows-amd64.zip` from releases
2. Extract to a folder (e.g., `C:\ldap-manager`)
3. Create a configuration file or use command-line flags
4. Run from Command Prompt or PowerShell:

```cmd
ldap-manager.exe ^
  --ldap-server ldaps://dc1.example.com:636 ^
  --base-dn "DC=example,DC=com" ^
  --readonly-user readonly ^
  --readonly-password your_password ^
  --active-directory
```

### Method 3: Build from Source

For development or custom builds.

#### Prerequisites

- **Go 1.23+** with module support
- **Node.js v16+** with npm/pnpm
- **Git** for cloning the repository

#### Build Steps

```bash
# Clone repository
git clone https://github.com/netresearch/ldap-manager.git
cd ldap-manager

# Install dependencies
make setup

# Build application with assets
make build

# The binary will be in the current directory
./ldap-manager --help
```

For detailed development setup, see the [Development Setup Guide](../development/setup.md).

## Configuration

### Environment File Method

Create a `.env.local` file for your configuration:

```bash
# Required LDAP settings
LDAP_SERVER=ldaps://dc1.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_READONLY_USER=readonly
LDAP_READONLY_PASSWORD=your_secure_password

# Active Directory support
LDAP_IS_AD=true

# Optional settings
LOG_LEVEL=info
PERSIST_SESSIONS=true
SESSION_DURATION=30m
```

Run the application:

```bash
./ldap-manager  # Reads from .env.local automatically
```

### Command-Line Configuration

```bash
./ldap-manager \
  --ldap-server ldaps://dc1.example.com:636 \
  --base-dn "DC=example,DC=com" \
  --readonly-user readonly \
  --readonly-password your_password \
  --active-directory \
  --log-level info \
  --persist-sessions \
  --session-duration 30m
```

For complete configuration options, see the [Configuration Reference](configuration.md).

## Service Installation

### Linux Systemd Service

Create `/etc/systemd/system/ldap-manager.service`:

```ini
[Unit]
Description=LDAP Manager
Documentation=https://github.com/netresearch/ldap-manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ldap-manager
Group=ldap-manager
WorkingDirectory=/opt/ldap-manager
ExecStart=/opt/ldap-manager/ldap-manager
EnvironmentFile=/opt/ldap-manager/.env.local
Restart=on-failure
RestartSec=5
TimeoutStopSec=30

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/opt/ldap-manager
ProtectHome=yes

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
# Create user and directory
sudo useradd --system --create-home --home-dir /opt/ldap-manager ldap-manager

# Install binary and configuration
sudo cp ldap-manager /opt/ldap-manager/
sudo cp .env.local /opt/ldap-manager/
sudo chown -R ldap-manager:ldap-manager /opt/ldap-manager

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable ldap-manager
sudo systemctl start ldap-manager

# Check status
sudo systemctl status ldap-manager
```

### Windows Service

Use a tool like [NSSM](https://nssm.cc/) to install as a Windows service:

```cmd
# Download and install NSSM
nssm install "LDAP Manager" "C:\ldap-manager\ldap-manager.exe"
nssm set "LDAP Manager" AppDirectory "C:\ldap-manager"
nssm set "LDAP Manager" Description "LDAP Directory Manager"
nssm start "LDAP Manager"
```

## Network Configuration

### Reverse Proxy Setup

#### Nginx

```nginx
server {
    listen 80;
    server_name ldap.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name ldap.example.com;

    ssl_certificate /path/to/ssl/cert.pem;
    ssl_certificate_key /path/to/ssl/private.key;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

#### Traefik v2

```yaml
version: '3.8'

services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    environment:
      # Your LDAP configuration here
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.ldap-manager.rule=Host(`ldap.example.com`)"
      - "traefik.http.routers.ldap-manager.tls=true"
      - "traefik.http.routers.ldap-manager.tls.certresolver=letsencrypt"
    networks:
      - traefik

networks:
  traefik:
    external: true
```

### Firewall Configuration

Open required ports:

```bash
# LDAP Manager web interface
sudo ufw allow 3000/tcp

# If using reverse proxy
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Outbound LDAP connections
sudo ufw allow out 389/tcp
sudo ufw allow out 636/tcp
```

## SSL/TLS Configuration

### For LDAPS Connections

When connecting to LDAP servers with self-signed certificates:

#### Docker Volume Mount

```bash
docker run -d \
  -v /path/to/certs:/etc/ssl/certs:ro \
  # ... other options
  ghcr.io/netresearch/ldap-manager
```

#### System Certificate Store

Add your LDAP server's certificate to the system trust store:

```bash
# Linux (Ubuntu/Debian)
sudo cp ldap-server-cert.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates

# Linux (CentOS/RHEL)
sudo cp ldap-server-cert.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
```

## Verification

After installation, verify the setup:

### Health Check

```bash
curl -f http://localhost:3000/
```

Should return the login page HTML.

### LDAP Connectivity Test

Check the logs for LDAP connection status:

```bash
# Docker logs
docker logs ldap-manager

# Systemd service logs
journalctl -u ldap-manager -f

# Direct execution
./ldap-manager --log-level debug
```

Look for messages like:
```
INF Starting LDAP Manager server on :3000
DBG LDAP connection test successful
```

### Login Test

1. Open browser to `http://localhost:3000`
2. Try logging in with a valid LDAP user
3. Verify you can browse users/groups/computers

## Troubleshooting

### Common Issues

**LDAP Connection Failed**
- Verify server hostname and port are accessible
- Check firewall rules on both client and server
- Validate LDAPS certificate if using secure connection
- Test with `ldapsearch` command-line tool

**Authentication Failures**
- Verify readonly user credentials
- Check user has read permissions to Base DN
- Ensure Base DN covers the intended directory scope

**Permission Denied**
- Check file system permissions for binary and configuration
- Ensure service user has required access
- Verify network connectivity to LDAP server

**Session Issues**
- Check session duration format (Go duration syntax)
- Verify session file path permissions when using persistence
- Monitor disk space for session storage

For detailed troubleshooting, see the [Configuration Reference](configuration.md#troubleshooting) or [Monitoring Guide](../operations/monitoring.md).

## Next Steps

- Review the [Configuration Reference](configuration.md) for advanced options
- Set up monitoring using the [Operations Guide](../operations/deployment.md)
- Configure users and test functionality
- Consider implementing backup procedures for persistent sessions