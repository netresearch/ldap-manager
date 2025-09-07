# Implementation Examples and Tutorials

Practical examples and tutorials for implementing and using LDAP Manager in real-world scenarios.

## Table of Contents

- [Quick Start Guide](#quick-start-guide)
- [Basic Implementation](#basic-implementation)
- [Advanced Configurations](#advanced-configurations)
- [Integration Examples](#integration-examples)
- [Common Use Cases](#common-use-cases)
- [Troubleshooting Examples](#troubleshooting-examples)
- [Best Practices](#best-practices)

---

## Quick Start Guide

### 10-Minute Setup

Get LDAP Manager running in 10 minutes with Docker:

```bash
# 1. Clone or download configuration template
curl -O https://raw.githubusercontent.com/netresearch/ldap-manager/main/.env.example
mv .env.example .env

# 2. Edit configuration (minimum required settings)
cat > .env << EOF
LDAP_SERVER=ldaps://dc.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_IS_AD=true
LDAP_READONLY_USER=CN=ldap-reader,OU=Service Accounts,DC=example,DC=com
LDAP_READONLY_PASSWORD=your_service_account_password
LOG_LEVEL=info
EOF

# 3. Run with Docker
docker run -d \
  --name ldap-manager \
  --env-file .env \
  -p 3000:3000 \
  ldap-manager:latest

# 4. Verify it's running
curl http://localhost:3000/health
```

### First Login

1. **Open Browser**: Navigate to `http://localhost:3000`
2. **Login**: Use your LDAP credentials (domain user account)
3. **Verify**: You should see the dashboard with user/group/computer management

---

## Basic Implementation

### Standard Active Directory Setup

Complete setup for Active Directory environment:

#### Step 1: Service Account Creation

```powershell
# Create service account in Active Directory
Import-Module ActiveDirectory

# Create the service account
New-ADUser -Name "ldap-manager-service" `
  -UserPrincipalName "ldap-manager-service@example.com" `
  -Path "OU=Service Accounts,DC=example,DC=com" `
  -AccountPassword (Read-Host -AsSecureString "Enter Password") `
  -Enabled $true `
  -Description "LDAP Manager Service Account - Read Only"

# Grant read permissions on Users container
$UsersOU = "OU=Users,DC=example,DC=com"
$ServiceAccount = "ldap-manager-service@example.com"
dsacls $UsersOU /G "${ServiceAccount}:GR"

# Grant read permissions on Groups container
$GroupsOU = "OU=Groups,DC=example,DC=com"
dsacls $GroupsOU /G "${ServiceAccount}:GR"

# Grant read permissions on Computers container  
$ComputersOU = "OU=Computers,DC=example,DC=com"
dsacls $ComputersOU /G "${ServiceAccount}:GR"

# Verify permissions
dsacls $UsersOU | Select-String $ServiceAccount
```

#### Step 2: Application Configuration

```bash
# Complete .env configuration for Active Directory
cat > .env << EOF
# LDAP Server Configuration
LDAP_SERVER=ldaps://dc.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_IS_AD=true

# Service Account (Read-Only)
LDAP_READONLY_USER=CN=ldap-manager-service,OU=Service Accounts,DC=example,DC=com
LDAP_READONLY_PASSWORD=SecureServiceAccountPassword

# Application Settings
LOG_LEVEL=info
PERSIST_SESSIONS=true
SESSION_PATH=/app/data/sessions.db
SESSION_DURATION=30m

# Connection Pool Settings
LDAP_POOL_MAX_CONNECTIONS=10
LDAP_POOL_MIN_CONNECTIONS=3
LDAP_POOL_MAX_IDLE_TIME=15m
LDAP_POOL_MAX_LIFETIME=1h
LDAP_POOL_HEALTH_CHECK_INTERVAL=30s
LDAP_POOL_ACQUIRE_TIMEOUT=10s
EOF
```

#### Step 3: Docker Deployment

```bash
# Create data directory for persistent sessions
mkdir -p /opt/ldap-manager/data
chown 1000:1000 /opt/ldap-manager/data

# Docker Compose deployment
cat > docker-compose.yml << EOF
version: '3.8'
services:
  ldap-manager:
    image: ldap-manager:latest
    container_name: ldap-manager
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - /opt/ldap-manager/data:/app/data
    env_file:
      - .env
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
EOF

# Start the service
docker-compose up -d

# Verify deployment
docker-compose logs -f ldap-manager
curl http://localhost:3000/health/ready
```

### OpenLDAP Setup

Configuration for standard OpenLDAP directory:

#### Step 1: OpenLDAP Service Account

```bash
# Create service account LDIF
cat > service-account.ldif << EOF
dn: cn=ldap-manager,ou=System,dc=example,dc=com
objectClass: simpleSecurityObject
objectClass: organizationalRole
cn: ldap-manager
description: LDAP Manager Service Account
userPassword: {SSHA}your_hashed_password_here
EOF

# Add service account to OpenLDAP
ldapadd -x -D "cn=admin,dc=example,dc=com" -W -f service-account.ldif

# Grant read access with ACL
cat > read-access.ldif << EOF  
dn: olcDatabase={1}mdb,cn=config
changetype: modify
replace: olcAccess
olcAccess: {0}to attrs=userPassword by self write by dn="cn=admin,dc=example,dc=com" write by dn="cn=ldap-manager,ou=System,dc=example,dc=com" read by anonymous auth by * none
olcAccess: {1}to * by dn="cn=admin,dc=example,dc=com" write by dn="cn=ldap-manager,ou=System,dc=example,dc=com" read by users read by * none
EOF

# Apply ACL changes
ldapmodify -Y EXTERNAL -H ldapi:/// -f read-access.ldif
```

#### Step 2: OpenLDAP Configuration

```bash
# OpenLDAP environment configuration
cat > .env << EOF
# OpenLDAP Server Configuration
LDAP_SERVER=ldaps://ldap.example.com:636
LDAP_BASE_DN=dc=example,dc=com
LDAP_IS_AD=false

# Service Account
LDAP_READONLY_USER=cn=ldap-manager,ou=System,dc=example,dc=com
LDAP_READONLY_PASSWORD=your_service_password

# Standard settings
LOG_LEVEL=info
PERSIST_SESSIONS=true
SESSION_DURATION=30m
EOF
```

---

## Advanced Configurations

### High-Availability Setup

Production-ready high-availability deployment:

#### Load Balancer Configuration

```nginx
# nginx load balancer configuration
upstream ldap_manager_backend {
    least_conn;
    server ldap-manager-1:3000 max_fails=3 fail_timeout=30s;
    server ldap-manager-2:3000 max_fails=3 fail_timeout=30s;
    server ldap-manager-3:3000 max_fails=3 fail_timeout=30s;
}

server {
    listen 443 ssl http2;
    server_name ldap-manager.example.com;
    
    # SSL configuration
    ssl_certificate /etc/nginx/ssl/ldap-manager.crt;
    ssl_certificate_key /etc/nginx/ssl/ldap-manager.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    location / {
        proxy_pass http://ldap_manager_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Health check configuration
        proxy_connect_timeout 5s;
        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
        
        # Session affinity (optional, improves cache performance)
        ip_hash;
    }
    
    # Health check endpoint
    location /health {
        proxy_pass http://ldap_manager_backend;
        access_log off;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name ldap-manager.example.com;
    return 301 https://$server_name$request_uri;
}
```

#### Kubernetes Deployment

```yaml
# Complete Kubernetes deployment
apiVersion: v1
kind: ConfigMap
metadata:
  name: ldap-manager-config
data:
  LDAP_SERVER: "ldaps://dc.example.com:636"
  LDAP_BASE_DN: "DC=example,DC=com"
  LDAP_IS_AD: "true"
  LDAP_READONLY_USER: "CN=ldap-manager-service,OU=Service Accounts,DC=example,DC=com"
  LOG_LEVEL: "info"
  PERSIST_SESSIONS: "true"
  SESSION_PATH: "/shared/sessions.db"
  SESSION_DURATION: "30m"
  LDAP_POOL_MAX_CONNECTIONS: "15"
  LDAP_POOL_MIN_CONNECTIONS: "5"

---
apiVersion: v1
kind: Secret
metadata:
  name: ldap-manager-secret
type: Opaque
stringData:
  LDAP_READONLY_PASSWORD: "your_service_account_password"

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ldap-manager-sessions
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-manager
  labels:
    app: ldap-manager
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ldap-manager
  template:
    metadata:
      labels:
        app: ldap-manager
    spec:
      containers:
      - name: ldap-manager
        image: ldap-manager:latest
        ports:
        - containerPort: 3000
        envFrom:
        - configMapRef:
            name: ldap-manager-config
        - secretRef:
            name: ldap-manager-secret
        volumeMounts:
        - name: sessions
          mountPath: /shared
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health/live
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 3000
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2
      volumes:
      - name: sessions
        persistentVolumeClaim:
          claimName: ldap-manager-sessions

---
apiVersion: v1
kind: Service
metadata:
  name: ldap-manager-service
spec:
  selector:
    app: ldap-manager
  ports:
    - protocol: TCP
      port: 80
      targetPort: 3000

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ldap-manager-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - ldap-manager.example.com
    secretName: ldap-manager-tls
  rules:
  - host: ldap-manager.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ldap-manager-service
            port:
              number: 80
```

### Multi-Domain Configuration

Support for multiple LDAP domains:

```bash
# Multi-domain setup with multiple instances
# Instance 1: Domain A
cat > .env.domain-a << EOF
LDAP_SERVER=ldaps://dc-a.example.com:636
LDAP_BASE_DN=DC=domain-a,DC=example,DC=com
LDAP_READONLY_USER=CN=ldap-manager,OU=Service,DC=domain-a,DC=example,DC=com
LDAP_READONLY_PASSWORD=password_a
SESSION_PATH=/data/sessions-domain-a.db
EOF

# Instance 2: Domain B  
cat > .env.domain-b << EOF
LDAP_SERVER=ldaps://dc-b.example.com:636
LDAP_BASE_DN=DC=domain-b,DC=example,DC=com
LDAP_READONLY_USER=CN=ldap-manager,OU=Service,DC=domain-b,DC=example,DC=com
LDAP_READONLY_PASSWORD=password_b
SESSION_PATH=/data/sessions-domain-b.db
EOF

# Docker Compose for multi-domain
cat > docker-compose.multi-domain.yml << EOF
version: '3.8'
services:
  ldap-manager-domain-a:
    image: ldap-manager:latest
    container_name: ldap-manager-domain-a
    ports:
      - "3001:3000"
    env_file:
      - .env.domain-a
    volumes:
      - ./data:/data
      
  ldap-manager-domain-b:
    image: ldap-manager:latest  
    container_name: ldap-manager-domain-b
    ports:
      - "3002:3000"
    env_file:
      - .env.domain-b
    volumes:
      - ./data:/data
EOF

# Start multi-domain deployment
docker-compose -f docker-compose.multi-domain.yml up -d
```

---

## Integration Examples

### SAML Integration

Integrate with SAML authentication:

```nginx
# Nginx with auth_request module for SAML integration
location /auth {
    internal;
    proxy_pass http://saml-auth-service/verify;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_set_header X-Original-URI $request_uri;
}

location / {
    auth_request /auth;
    
    # Pass SAML attributes to application
    auth_request_set $user $upstream_http_x_user;
    auth_request_set $groups $upstream_http_x_groups;
    
    proxy_pass http://ldap-manager:3000;
    proxy_set_header X-Remote-User $user;
    proxy_set_header X-Remote-Groups $groups;
}
```

### Monitoring Integration

#### Prometheus Metrics

```bash
# Add Prometheus metrics endpoint (custom implementation)
cat > prometheus-metrics.sh << EOF
#!/bin/bash
# Collect LDAP Manager metrics for Prometheus

# Get application statistics
CACHE_STATS=$(curl -s -b "session=$SESSION_COOKIE" http://localhost:3000/debug/cache)
POOL_STATS=$(curl -s -b "session=$SESSION_COOKIE" http://localhost:3000/debug/ldap-pool)

# Extract metrics
CACHE_HITS=$(echo $CACHE_STATS | jq '.hits')
CACHE_MISSES=$(echo $CACHE_STATS | jq '.misses')
POOL_ACTIVE=$(echo $POOL_STATS | jq '.stats.active_connections')
POOL_TOTAL=$(echo $POOL_STATS | jq '.stats.total_connections')

# Output Prometheus metrics
cat << PROM
# HELP ldap_manager_cache_hits_total Total cache hits
# TYPE ldap_manager_cache_hits_total counter
ldap_manager_cache_hits_total $CACHE_HITS

# HELP ldap_manager_cache_misses_total Total cache misses
# TYPE ldap_manager_cache_misses_total counter
ldap_manager_cache_misses_total $CACHE_MISSES

# HELP ldap_manager_pool_active_connections Active LDAP connections
# TYPE ldap_manager_pool_active_connections gauge
ldap_manager_pool_active_connections $POOL_ACTIVE

# HELP ldap_manager_pool_total_connections Total LDAP connections
# TYPE ldap_manager_pool_total_connections gauge
ldap_manager_pool_total_connections $POOL_TOTAL
PROM
EOF

chmod +x prometheus-metrics.sh
```

#### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "LDAP Manager Dashboard",
    "panels": [
      {
        "title": "Cache Performance",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(ldap_manager_cache_hits_total[5m]) / (rate(ldap_manager_cache_hits_total[5m]) + rate(ldap_manager_cache_misses_total[5m]))",
            "legendFormat": "Hit Ratio"
          }
        ]
      },
      {
        "title": "Connection Pool",
        "type": "graph", 
        "targets": [
          {
            "expr": "ldap_manager_pool_active_connections",
            "legendFormat": "Active Connections"
          },
          {
            "expr": "ldap_manager_pool_total_connections", 
            "legendFormat": "Total Connections"
          }
        ]
      }
    ]
  }
}
```

### Log Management Integration

#### ELK Stack Integration

```bash
# Logstash configuration for LDAP Manager logs
cat > logstash-ldap-manager.conf << EOF
input {
  file {
    path => "/var/log/ldap-manager/app.log"
    start_position => "beginning"
    codec => "json"
  }
}

filter {
  if [level] {
    mutate {
      add_field => { "log_level" => "%{level}" }
    }
  }
  
  if [event] {
    mutate {
      add_field => { "event_type" => "%{event}" }
    }
  }
  
  if [source_ip] {
    geoip {
      source => "source_ip"
      target => "geo"
    }
  }
  
  date {
    match => [ "time", "ISO8601" ]
    target => "@timestamp"
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "ldap-manager-%{+YYYY.MM.dd}"
  }
}
EOF
```

#### Elasticsearch Index Template

```bash
# Create Elasticsearch index template
curl -X PUT "elasticsearch:9200/_index_template/ldap-manager" -H 'Content-Type: application/json' -d'
{
  "index_patterns": ["ldap-manager-*"],
  "template": {
    "mappings": {
      "properties": {
        "timestamp": {"type": "date"},
        "level": {"type": "keyword"},
        "event": {"type": "keyword"},
        "user": {"type": "keyword"},
        "source_ip": {"type": "ip"},
        "message": {"type": "text"},
        "geo": {
          "properties": {
            "location": {"type": "geo_point"}
          }
        }
      }
    }
  }
}'
```

---

## Common Use Cases

### User Self-Service Portal

Configuration for user self-service scenarios:

#### Limited User Interface

```bash
# Configuration for self-service portal
cat > .env.self-service << EOF
# Standard LDAP configuration
LDAP_SERVER=ldaps://dc.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_IS_AD=true
LDAP_READONLY_USER=CN=ldap-reader,OU=Service,DC=example,DC=com
LDAP_READONLY_PASSWORD=service_password

# Self-service specific settings
SESSION_DURATION=15m                    # Shorter sessions
LOG_LEVEL=warn                         # Reduced logging
LDAP_POOL_MAX_CONNECTIONS=5            # Fewer connections needed
EOF
```

#### Custom CSS for Branding

```css
/* Custom branding CSS - mount as volume */
:root {
  --primary-color: #your-brand-color;
  --secondary-color: #your-secondary-color;
}

.logo {
  content: url('/static/your-company-logo.png');
}

.navbar-brand {
  color: var(--primary-color) !important;
}
```

### Help Desk Interface

Configuration for IT help desk usage:

```bash
# Help desk configuration
cat > .env.helpdesk << EOF
LDAP_SERVER=ldaps://dc.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_IS_AD=true
LDAP_READONLY_USER=CN=helpdesk-ldap,OU=Service,DC=example,DC=com
LDAP_READONLY_PASSWORD=helpdesk_service_password

# Help desk optimized settings
SESSION_DURATION=8h                    # Full work day sessions
LOG_LEVEL=info                         # Standard logging
LDAP_POOL_MAX_CONNECTIONS=15           # Higher concurrency
PERSIST_SESSIONS=true                  # Maintain sessions across shifts
EOF
```

### Read-Only Directory Browser

Configuration for read-only directory browsing:

```bash
# Read-only browser configuration  
cat > .env.readonly << EOF
LDAP_SERVER=ldaps://dc.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_IS_AD=true
LDAP_READONLY_USER=CN=directory-reader,OU=Service,DC=example,DC=com
LDAP_READONLY_PASSWORD=readonly_password

# Read-only optimized settings
SESSION_DURATION=1h                    # Short sessions
LDAP_POOL_MAX_CONNECTIONS=8            # Moderate concurrency
LDAP_POOL_MAX_IDLE_TIME=30m            # Longer idle time for browsing
EOF

# Custom CSS to hide edit functionality (visual only - server still enforces permissions)
cat > readonly-custom.css << EOF
.edit-button, .modify-form, .delete-button {
  display: none !important;
}

.readonly-notice {
  background-color: #f0f8ff;
  border: 1px solid #0066cc;
  padding: 10px;
  margin: 10px 0;
  border-radius: 4px;
}
EOF
```

---

## Troubleshooting Examples

### Connection Issues

#### LDAP Connection Troubleshooting

```bash
# Test LDAP connectivity
#!/bin/bash
echo "Testing LDAP connectivity..."

# Test basic connectivity
echo "1. Testing basic connectivity:"
nc -zv dc.example.com 636
echo

# Test SSL/TLS certificate
echo "2. Testing SSL certificate:"
openssl s_client -connect dc.example.com:636 -servername dc.example.com < /dev/null
echo

# Test LDAP authentication
echo "3. Testing LDAP authentication:"
ldapsearch -H ldaps://dc.example.com:636 \
  -D "CN=ldap-reader,OU=Service,DC=example,DC=com" \
  -W -b "DC=example,DC=com" \
  -s base "(objectclass=*)" \
  -LLL dn
echo

# Test LDAP search
echo "4. Testing LDAP search:"
ldapsearch -H ldaps://dc.example.com:636 \
  -D "CN=ldap-reader,OU=Service,DC=example,DC=com" \
  -W -b "OU=Users,DC=example,DC=com" \
  "(objectclass=user)" cn mail -LLL | head -20
```

#### Network Troubleshooting

```bash
# Network diagnostics script
#!/bin/bash
echo "Network diagnostics for LDAP Manager..."

# Check DNS resolution
echo "1. DNS Resolution:"
nslookup dc.example.com
echo

# Check routing
echo "2. Routing test:"
traceroute dc.example.com
echo

# Check firewall/connectivity
echo "3. Port connectivity:"
for port in 636 389; do
    echo "Testing port $port:"
    timeout 5 bash -c "</dev/tcp/dc.example.com/$port && echo 'Port $port open' || echo 'Port $port closed'"
done
echo

# Check certificate chain
echo "4. Certificate validation:"
openssl s_client -connect dc.example.com:636 -verify_return_error -CApath /etc/ssl/certs/
```

### Performance Issues

#### Performance Diagnostics

```bash
# Performance diagnostics script
#!/bin/bash
echo "LDAP Manager Performance Diagnostics"
echo "====================================="

# Get application statistics
echo "1. Application Statistics:"
curl -s http://localhost:3000/health | jq '.'
echo

echo "2. Cache Statistics:"
curl -s -b "session=$SESSION_COOKIE" http://localhost:3000/debug/cache | jq '.'
echo

echo "3. Connection Pool Statistics:" 
curl -s -b "session=$SESSION_COOKIE" http://localhost:3000/debug/ldap-pool | jq '.'
echo

# System resource usage
echo "4. System Resources:"
echo "CPU Usage:"
top -bn1 | grep "ldap-manager" | head -5
echo

echo "Memory Usage:"
ps aux | grep ldap-manager | grep -v grep
echo

echo "Network Connections:"
ss -tuln | grep :3000
echo

# Docker container stats (if applicable)
if command -v docker &> /dev/null; then
    echo "5. Container Statistics:"
    docker stats ldap-manager --no-stream
fi
```

#### Load Testing

```bash
# Simple load testing script
#!/bin/bash
echo "LDAP Manager Load Testing"
echo "========================"

# Test unauthenticated endpoint
echo "1. Testing health endpoint (no auth required):"
ab -n 100 -c 10 http://localhost:3000/health
echo

# Test authenticated endpoint (requires valid session)  
if [ -n "$SESSION_COOKIE" ]; then
    echo "2. Testing authenticated endpoint:"
    ab -n 50 -c 5 -C "session=$SESSION_COOKIE" http://localhost:3000/users
else
    echo "2. Skipping authenticated test (no session cookie provided)"
    echo "   Set SESSION_COOKIE environment variable to test authenticated endpoints"
fi
```

### Authentication Issues

#### Authentication Diagnostics

```bash
# Authentication troubleshooting script
#!/bin/bash
echo "Authentication Diagnostics"
echo "=========================="

# Check LDAP server authentication
echo "1. Service Account Authentication:"
ldapwhoami -H ldaps://dc.example.com:636 \
  -D "CN=ldap-reader,OU=Service,DC=example,DC=com" \
  -W
echo

# Test user authentication
echo "2. User Authentication Test:"
read -p "Enter test username: " USERNAME
read -s -p "Enter test password: " PASSWORD
echo

ldapwhoami -H ldaps://dc.example.com:636 \
  -D "$USERNAME@example.com" \
  -w "$PASSWORD"
echo

# Check user permissions
echo "3. User Search Permissions:"
ldapsearch -H ldaps://dc.example.com:636 \
  -D "$USERNAME@example.com" \
  -w "$PASSWORD" \
  -b "OU=Users,DC=example,DC=com" \
  "(sAMAccountName=$USERNAME)" \
  -LLL dn cn mail
```

---

## Best Practices

### Configuration Management

#### Environment-Specific Configurations

```bash
# Use environment-specific configuration files
# .env.development
cat > .env.development << EOF
LOG_LEVEL=debug
SESSION_DURATION=4h
LDAP_POOL_MAX_CONNECTIONS=5
PERSIST_SESSIONS=false
EOF

# .env.staging  
cat > .env.staging << EOF
LOG_LEVEL=info
SESSION_DURATION=1h
LDAP_POOL_MAX_CONNECTIONS=10
PERSIST_SESSIONS=true
EOF

# .env.production
cat > .env.production << EOF
LOG_LEVEL=warn
SESSION_DURATION=30m
LDAP_POOL_MAX_CONNECTIONS=20
PERSIST_SESSIONS=true
EOF
```

#### Configuration Validation

```bash
# Configuration validation script
#!/bin/bash
validate_config() {
    echo "Validating LDAP Manager configuration..."
    
    # Required variables
    required_vars=(
        "LDAP_SERVER"
        "LDAP_BASE_DN"
        "LDAP_READONLY_USER"
        "LDAP_READONLY_PASSWORD"
    )
    
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            echo "ERROR: $var is not set"
            exit 1
        else
            echo "✓ $var is set"
        fi
    done
    
    # Validate LDAP server format
    if [[ ! "$LDAP_SERVER" =~ ^ldaps?:// ]]; then
        echo "ERROR: LDAP_SERVER must start with ldap:// or ldaps://"
        exit 1
    fi
    echo "✓ LDAP_SERVER format is valid"
    
    # Test connectivity
    hostname=$(echo "$LDAP_SERVER" | sed 's|.*://||' | sed 's|:.*||')
    port=$(echo "$LDAP_SERVER" | grep -o ':[0-9]*' | sed 's/://' || echo "389")
    
    if nc -zv "$hostname" "$port" 2>/dev/null; then
        echo "✓ LDAP server is reachable"
    else
        echo "WARNING: LDAP server connectivity test failed"
    fi
    
    echo "Configuration validation completed"
}

# Load configuration and validate
source .env
validate_config
```

### Deployment Best Practices

#### Blue-Green Deployment

```bash
# Blue-green deployment script
#!/bin/bash
deploy_blue_green() {
    local new_version=$1
    local current_service="ldap-manager-blue"
    local new_service="ldap-manager-green"
    
    echo "Starting blue-green deployment of version $new_version"
    
    # Deploy new version to green environment
    echo "Deploying to green environment..."
    docker-compose -f docker-compose.green.yml up -d
    
    # Wait for green to be healthy
    echo "Waiting for green environment to be healthy..."
    for i in {1..30}; do
        if curl -f http://localhost:3001/health/ready; then
            echo "Green environment is healthy"
            break
        fi
        echo "Waiting... ($i/30)"
        sleep 10
    done
    
    # Switch traffic to green
    echo "Switching traffic to green environment..."
    # Update load balancer configuration here
    # nginx reload, or update Kubernetes service, etc.
    
    # Keep blue running for rollback capability
    echo "Deployment complete. Blue environment kept for rollback."
    echo "To complete deployment: docker-compose -f docker-compose.blue.yml down"
    echo "To rollback: Update load balancer back to blue environment"
}
```

#### Health Check Monitoring

```bash
# Comprehensive health monitoring
#!/bin/bash
monitor_health() {
    while true; do
        # Basic health check
        if curl -f http://localhost:3000/health; then
            echo "$(date): Health check PASSED"
        else
            echo "$(date): Health check FAILED"
            # Alert here
        fi
        
        # Readiness check
        if curl -f http://localhost:3000/health/ready; then
            echo "$(date): Readiness check PASSED"
        else
            echo "$(date): Readiness check FAILED - LDAP connectivity issues"
            # Alert here
        fi
        
        # Performance check
        response_time=$(curl -o /dev/null -s -w '%{time_total}' http://localhost:3000/health)
        if (( $(echo "$response_time > 1.0" | bc -l) )); then
            echo "$(date): Performance degradation detected: ${response_time}s response time"
            # Alert here
        fi
        
        sleep 60
    done
}

# Start monitoring in background
monitor_health &
```

### Security Best Practices

#### Regular Security Audits

```bash
# Security audit script
#!/bin/bash
security_audit() {
    echo "LDAP Manager Security Audit"
    echo "=========================="
    
    # Check file permissions
    echo "1. File Permissions:"
    ls -la /opt/ldap-manager/
    
    # Check for sensitive data in logs
    echo "2. Log Security Check:"
    if grep -i "password\|secret\|key" /var/log/ldap-manager/*.log; then
        echo "WARNING: Sensitive data found in logs"
    else
        echo "✓ No sensitive data found in logs"
    fi
    
    # Check container security
    echo "3. Container Security:"
    docker exec ldap-manager whoami
    docker exec ldap-manager ls -la /app
    
    # Check network security
    echo "4. Network Security:"
    ss -tuln | grep :3000
    
    # Check SSL/TLS configuration
    echo "5. SSL/TLS Security:"
    openssl s_client -connect dc.example.com:636 -cipher 'ALL:!aNULL:!eNULL:!EXPORT:!DES:!RC4:!MD5:!PSK:!SRP:!CAMELLIA'
}
```

This comprehensive implementation guide provides practical examples for deploying and using LDAP Manager in various real-world scenarios. Regular testing and monitoring ensure optimal performance and security.

For additional technical details, see the [API Reference](api.md), [Configuration Reference](configuration.md), and [Architecture Documentation](../development/architecture-detailed.md).