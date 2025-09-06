# Monitoring & Troubleshooting

Comprehensive guide for monitoring LDAP Manager in production environments and troubleshooting common operational issues.

## Monitoring Overview

LDAP Manager provides multiple monitoring points for ensuring healthy operations:

- **Application Health**: HTTP endpoints and process status
- **LDAP Connectivity**: Directory server connection monitoring
- **Performance Metrics**: Response times and resource usage
- **Security Events**: Authentication failures and session management
- **System Resources**: CPU, memory, disk, and network usage

## Health Checks

### HTTP Health Checks

LDAP Manager responds to HTTP requests on its main endpoints, making it suitable for load balancer health checks.

**Basic Health Check:**
```bash
# Simple availability check
curl -f http://localhost:3000/

# Expected responses:
# - 200 OK: Login page (healthy)
# - 302 Found: Redirect to login (healthy)
# - Connection refused: Application down
# - 5xx errors: Application unhealthy
```

**Comprehensive Health Check Script:**
```bash
#!/bin/bash
# health-check.sh

LDAP_MANAGER_URL="http://localhost:3000"
TIMEOUT=10
RETRIES=3

check_health() {
    local attempt=1
    while [ $attempt -le $RETRIES ]; do
        response=$(curl -s -o /dev/null -w "%{http_code}:%{time_total}" \
                   --max-time $TIMEOUT "$LDAP_MANAGER_URL" 2>/dev/null)
        
        if [ $? -eq 0 ]; then
            status_code=$(echo $response | cut -d: -f1)
            response_time=$(echo $response | cut -d: -f2)
            
            if [[ "$status_code" =~ ^(200|302)$ ]]; then
                echo "OK - LDAP Manager healthy (HTTP $status_code, ${response_time}s)"
                return 0
            else
                echo "WARNING - Unexpected HTTP status: $status_code"
            fi
        else
            echo "CRITICAL - Connection failed (attempt $attempt/$RETRIES)"
        fi
        
        attempt=$((attempt + 1))
        sleep 2
    done
    
    echo "CRITICAL - LDAP Manager health check failed after $RETRIES attempts"
    return 2
}

check_health
```

### Docker Health Checks

**Dockerfile Health Check:**
```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/ || exit 1
```

**Docker Compose Health Check:**
```yaml
services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    # ... other configuration
```

### Kubernetes Health Checks

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-manager
spec:
  template:
    spec:
      containers:
      - name: ldap-manager
        image: ghcr.io/netresearch/ldap-manager:latest
        
        livenessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 60
          periodSeconds: 30
          timeoutSeconds: 10
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2
```

## Log Monitoring

### Log Levels and Formats

LDAP Manager uses structured logging with configurable levels:

**Log Levels:**
- `trace`: Extremely detailed debugging (development only)
- `debug`: Detailed operational information
- `info`: General operational events
- `warn`: Warning conditions
- `error`: Error conditions requiring attention
- `fatal`: Fatal errors causing shutdown

**Log Format Example:**
```json
{
  "level": "info",
  "time": "2024-09-06T10:30:45.123Z",
  "message": "user authentication successful",
  "user_dn": "CN=John Doe,OU=Users,DC=example,DC=com",
  "source_ip": "192.168.1.100",
  "duration": 245.5
}
```

### Log Analysis

**Key Log Patterns to Monitor:**

**Authentication Events:**
```bash
# Successful authentications
grep '"level":"info"' /var/log/ldap-manager.log | grep "authentication successful"

# Failed authentications
grep '"level":"warn"' /var/log/ldap-manager.log | grep "authentication failed"

# Session timeouts
grep '"level":"info"' /var/log/ldap-manager.log | grep "session expired"
```

**LDAP Operations:**
```bash
# LDAP connection issues
grep '"level":"error"' /var/log/ldap-manager.log | grep "ldap"

# Cache refresh operations
grep '"level":"debug"' /var/log/ldap-manager.log | grep "cache refresh"

# Slow LDAP queries (>1 second)
grep '"duration":[0-9]\{4,\}' /var/log/ldap-manager.log
```

**Application Errors:**
```bash
# Critical errors
grep '"level":"error"' /var/log/ldap-manager.log

# Fatal errors
grep '"level":"fatal"' /var/log/ldap-manager.log

# Panic recoveries
grep "panic" /var/log/ldap-manager.log
```

### Centralized Logging

**Docker Logging Driver:**
```yaml
services:
  ldap-manager:
    image: ghcr.io/netresearch/ldap-manager:latest
    logging:
      driver: "fluentd"
      options:
        fluentd-address: "logging-server:24224"
        fluentd-async-connect: "true"
        tag: "ldap-manager"
```

**Logrotate Configuration:**
```bash
# /etc/logrotate.d/ldap-manager
/var/log/ldap-manager/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0644 ldap-manager ldap-manager
    postrotate
        systemctl reload ldap-manager
    endscript
}
```

**ELK Stack Integration:**
```bash
# Filebeat configuration for LDAP Manager
# filebeat.yml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /var/log/ldap-manager/*.log
  json.keys_under_root: true
  json.message_key: message
  fields:
    service: ldap-manager
    environment: production

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "ldap-manager-%{+yyyy.MM.dd}"
```

## Performance Monitoring

### Application Metrics

**Response Time Monitoring:**
```bash
#!/bin/bash
# response-time-check.sh

URL="https://ldap.company.com"
THRESHOLD=2000  # milliseconds

response_time=$(curl -w "%{time_total}" -s -o /dev/null "$URL")
response_time_ms=$(echo "$response_time * 1000" | bc)

if (( $(echo "$response_time_ms > $THRESHOLD" | bc -l) )); then
    echo "WARNING - Response time ${response_time_ms}ms exceeds threshold ${THRESHOLD}ms"
    exit 1
else
    echo "OK - Response time ${response_time_ms}ms"
    exit 0
fi
```

**LDAP Cache Performance:**
```bash
# Monitor cache hit rates in debug logs
tail -f /var/log/ldap-manager.log | grep -E "(cache hit|cache miss)" | \
while read line; do
    echo $line | jq -r '.time + " " + .message'
done
```

### Resource Monitoring

**System Resource Usage:**
```bash
#!/bin/bash
# resource-check.sh

# CPU usage
cpu_usage=$(ps -p $(pidof ldap-manager) -o %cpu= | tr -d ' ')
echo "CPU Usage: ${cpu_usage}%"

# Memory usage
mem_usage=$(ps -p $(pidof ldap-manager) -o %mem= | tr -d ' ')
mem_rss=$(ps -p $(pidof ldap-manager) -o rss= | tr -d ' ')
echo "Memory Usage: ${mem_usage}% (${mem_rss}KB)"

# File descriptors
fd_count=$(lsof -p $(pidof ldap-manager) | wc -l)
echo "Open File Descriptors: $fd_count"

# Network connections
conn_count=$(ss -p | grep ldap-manager | wc -l)
echo "Network Connections: $conn_count"
```

**Docker Resource Monitoring:**
```bash
# Container resource usage
docker stats ldap-manager --no-stream --format \
    "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"

# Container process information
docker exec ldap-manager ps aux
```

### Database Monitoring

**BBolt Session Storage:**
```bash
#!/bin/bash
# session-db-check.sh

SESSION_DB="/opt/ldap-manager/sessions.bbolt"

if [ -f "$SESSION_DB" ]; then
    size=$(du -h "$SESSION_DB" | cut -f1)
    echo "Session database size: $size"
    
    # Check file permissions
    permissions=$(ls -la "$SESSION_DB" | awk '{print $1,$3,$4}')
    echo "Permissions: $permissions"
    
    # Check last modification time
    mtime=$(stat -c %y "$SESSION_DB")
    echo "Last modified: $mtime"
else
    echo "WARNING - Session database not found at $SESSION_DB"
fi
```

## Alerting

### Nagios Integration

```bash
# /etc/nagios/objects/ldap-manager.cfg
define service {
    use                     generic-service
    host_name               ldap-server
    service_description     LDAP Manager Web Service
    check_command           check_http!-H ldap.company.com -S
    normal_check_interval   5
    retry_check_interval    1
}

define service {
    use                     generic-service
    host_name               ldap-server
    service_description     LDAP Manager Response Time
    check_command           check_http!-H ldap.company.com -w 2 -c 5
}
```

### Prometheus Monitoring

**Prometheus Configuration:**
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'ldap-manager'
    static_configs:
      - targets: ['ldap.company.com:3000']
    metrics_path: /metrics  # If custom metrics endpoint implemented
    scrape_interval: 30s
```

**Custom Metrics Collection Script:**
```bash
#!/bin/bash
# metrics-collector.sh
# Custom script to export metrics in Prometheus format

# Active sessions count (from BBolt database)
SESSION_COUNT=$(echo "SELECT COUNT(*) FROM sessions;" | sqlite3 /opt/ldap-manager/sessions.bbolt 2>/dev/null || echo "0")

# Response time check
RESPONSE_TIME=$(curl -w "%{time_total}" -s -o /dev/null http://localhost:3000)

# Process metrics
CPU_PERCENT=$(ps -p $(pidof ldap-manager) -o %cpu= | tr -d ' ')
MEM_PERCENT=$(ps -p $(pidof ldap-manager) -o %mem= | tr -d ' ')

# Output Prometheus format
cat << EOF
# HELP ldap_manager_active_sessions Number of active user sessions
# TYPE ldap_manager_active_sessions gauge
ldap_manager_active_sessions $SESSION_COUNT

# HELP ldap_manager_response_time_seconds Response time in seconds
# TYPE ldap_manager_response_time_seconds gauge
ldap_manager_response_time_seconds $RESPONSE_TIME

# HELP ldap_manager_cpu_percent CPU usage percentage
# TYPE ldap_manager_cpu_percent gauge
ldap_manager_cpu_percent $CPU_PERCENT

# HELP ldap_manager_memory_percent Memory usage percentage  
# TYPE ldap_manager_memory_percent gauge
ldap_manager_memory_percent $MEM_PERCENT
EOF
```

### Alert Rules

**Example Alert Conditions:**
```yaml
# alertmanager.yml
groups:
- name: ldap-manager
  rules:
  - alert: LDAPManagerDown
    expr: up{job="ldap-manager"} == 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "LDAP Manager is down"
      description: "LDAP Manager has been down for more than 2 minutes."

  - alert: LDAPManagerHighResponseTime
    expr: ldap_manager_response_time_seconds > 5
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "LDAP Manager response time is high"
      description: "Response time is {{ $value }}s for 5 minutes."

  - alert: LDAPManagerHighMemoryUsage
    expr: ldap_manager_memory_percent > 80
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "LDAP Manager memory usage is high"
      description: "Memory usage is {{ $value }}% for 5 minutes."
```

## Troubleshooting Guide

### Common Issues and Solutions

#### Application Won't Start

**Symptoms:**
- Container exits immediately
- Process crashes on startup
- No response on port 3000

**Diagnostic Steps:**
```bash
# Check application logs
docker logs ldap-manager --tail 50

# Check configuration
docker exec ldap-manager env | grep LDAP

# Test LDAP connectivity
docker exec ldap-manager nslookup dc1.company.com
docker exec ldap-manager telnet dc1.company.com 636
```

**Common Causes:**
1. **Missing required environment variables**
   ```bash
   # Solution: Verify all required variables are set
   echo $LDAP_SERVER $LDAP_BASE_DN $LDAP_READONLY_USER
   ```

2. **LDAP server unreachable**
   ```bash
   # Solution: Test network connectivity
   ping dc1.company.com
   telnet dc1.company.com 636
   ```

3. **Invalid LDAP credentials**
   ```bash
   # Solution: Test credentials manually
   ldapsearch -H ldaps://dc1.company.com:636 \
     -D "readonly@company.com" -w "password" \
     -b "DC=company,DC=com" -s base
   ```

#### Authentication Failures

**Symptoms:**
- Users cannot log in with correct credentials
- "Invalid credentials" error messages
- Authentication timeouts

**Diagnostic Commands:**
```bash
# Check LDAP connectivity
ldapsearch -H $LDAP_SERVER -D $LDAP_READONLY_USER -w $LDAP_READONLY_PASSWORD \
  -b $LDAP_BASE_DN "(sAMAccountName=testuser)" dn

# Test user authentication
ldapsearch -H $LDAP_SERVER -D "CN=testuser,OU=Users,DC=company,DC=com" \
  -w "userpassword" -b $LDAP_BASE_DN -s base
```

**Common Solutions:**
1. **Base DN scope too narrow**
   ```bash
   # Expand Base DN to cover user locations
   LDAP_BASE_DN=DC=company,DC=com  # Instead of OU=Users,DC=company,DC=com
   ```

2. **Active Directory format issues**
   ```bash
   # Use UPN format for AD
   username: user@company.com  # Instead of CN=user,OU=Users,DC=company,DC=com
   ```

3. **LDAP server certificate issues**
   ```bash
   # Add CA certificate to trust store
   cp company-ca.crt /usr/local/share/ca-certificates/
   update-ca-certificates
   ```

#### Performance Issues

**Symptoms:**
- Slow page loads
- High CPU/memory usage
- LDAP query timeouts

**Performance Analysis:**
```bash
# Enable debug logging
LOG_LEVEL=debug docker restart ldap-manager

# Monitor resource usage
top -p $(pidof ldap-manager)
iostat -x 1

# Check LDAP server performance
time ldapsearch -H $LDAP_SERVER -b $LDAP_BASE_DN "(objectClass=user)" dn | wc -l
```

**Optimization Steps:**
1. **Increase cache refresh interval** (requires code modification)
2. **Use LDAP replica** for read operations
3. **Optimize LDAP queries** with proper indexing
4. **Scale horizontally** with multiple instances

#### Session Issues

**Symptoms:**
- Users logged out unexpectedly
- "Session expired" errors
- Cannot maintain login state

**Session Diagnostics:**
```bash
# Check session storage
ls -la /opt/ldap-manager/sessions.bbolt
du -h /opt/ldap-manager/sessions.bbolt

# Monitor session activity
tail -f /var/log/ldap-manager.log | grep session

# Check session configuration
echo $SESSION_DURATION $PERSIST_SESSIONS $SESSION_PATH
```

**Solutions:**
1. **Extend session duration**
   ```bash
   SESSION_DURATION=2h  # Increase from default 30m
   ```

2. **Enable persistent sessions**
   ```bash
   PERSIST_SESSIONS=true
   SESSION_PATH=/data/sessions.bbolt
   ```

3. **Check file permissions**
   ```bash
   chown ldap-manager:ldap-manager /data/sessions.bbolt
   chmod 600 /data/sessions.bbolt
   ```

### Debug Mode Operation

**Enable Comprehensive Debugging:**
```bash
# Environment variables for debug mode
export LOG_LEVEL=debug
export LDAP_DEBUG=true  # If implemented

# Start with debug logging
./ldap-manager --log-level debug

# Or with Docker
docker run -e LOG_LEVEL=debug ldap-manager:latest
```

**Debug Log Analysis:**
```bash
# LDAP operation timing
grep '"message":"ldap"' /var/log/ldap-manager.log | jq '.duration'

# Session management
grep '"message":"session"' /var/log/ldap-manager.log

# Cache operations
grep '"message":"cache"' /var/log/ldap-manager.log

# HTTP request tracing
grep '"method":"' /var/log/ldap-manager.log | jq '{method, path, duration, status}'
```

### Network Diagnostics

**Connection Testing:**
```bash
# Test LDAP connectivity
nc -zv dc1.company.com 636

# SSL certificate check
echo | openssl s_client -connect dc1.company.com:636 -servername dc1.company.com

# DNS resolution
dig dc1.company.com
nslookup dc1.company.com

# Routing check
traceroute dc1.company.com
```

**Firewall Diagnostics:**
```bash
# Check listening ports
ss -tlnp | grep 3000

# Check iptables rules
iptables -L -n -v

# Check SELinux (if applicable)
sestatus
sealert -a /var/log/audit/audit.log
```

### Emergency Procedures

**Service Recovery:**
```bash
# Restart service
systemctl restart ldap-manager

# Or Docker
docker restart ldap-manager

# Check service status
systemctl status ldap-manager
docker ps ldap-manager
```

**Rollback Procedure:**
```bash
# Stop current version
systemctl stop ldap-manager

# Restore previous version
cp /opt/ldap-manager/ldap-manager.backup /opt/ldap-manager/ldap-manager

# Start service
systemctl start ldap-manager
```

**Data Recovery:**
```bash
# Restore session database
cp /backup/sessions-20240901.bbolt /opt/ldap-manager/sessions.bbolt
chown ldap-manager:ldap-manager /opt/ldap-manager/sessions.bbolt

# Restore configuration
cp /backup/.env.local.backup /opt/ldap-manager/.env.local
```

This monitoring and troubleshooting guide provides comprehensive coverage for maintaining LDAP Manager in production. Regular monitoring and proactive issue resolution will ensure reliable service for your users.