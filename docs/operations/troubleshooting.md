# Troubleshooting Guide

Comprehensive troubleshooting guide for LDAP Manager operational issues, including diagnostic procedures, common problems, and solutions.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Connection Issues](#connection-issues)
- [Authentication Problems](#authentication-problems)
- [Performance Issues](#performance-issues)
- [Cache Problems](#cache-problems)
- [Session Issues](#session-issues)
- [Container and Deployment Issues](#container-and-deployment-issues)
- [LDAP-Specific Problems](#ldap-specific-problems)
- [Monitoring and Logging](#monitoring-and-logging)
- [Emergency Procedures](#emergency-procedures)

---

## Quick Diagnostics

### Health Check Commands

Run these commands for immediate system status:

```bash
# Basic application health
curl http://localhost:3000/health
# Expected: {"status":"healthy","timestamp":"...","version":"..."}

# LDAP connectivity check
curl http://localhost:3000/health/ready
# Expected: {"status":"ready","ldap":"connected","cache":"active"}

# Application liveness
curl http://localhost:3000/health/live
# Expected: {"status":"live","uptime":"...","memory":"..."}

# Container health (if using Docker)
docker ps | grep ldap-manager
docker logs --tail=50 ldap-manager

# Process health (if running directly)
ps aux | grep ldap-manager
netstat -tlnp | grep :3000
```

### System Resource Check

```bash
# Memory usage
free -h
ps aux --sort=-%mem | head -10

# CPU usage
top -bn1 | head -10
iostat 1 3

# Disk space
df -h
du -sh /var/log/ldap-manager/

# Network connectivity
ping -c3 dc.example.com
nc -zv dc.example.com 636
```

### Log Analysis Quick Check

```bash
# Recent errors
tail -50 /var/log/ldap-manager/app.log | grep -i error

# Authentication failures
grep "authentication.*failure" /var/log/ldap-manager/app.log | tail -10

# LDAP connection issues
grep -i "ldap.*error\|connection.*failed" /var/log/ldap-manager/app.log | tail -10

# Performance issues
grep "response_time\|slow\|timeout" /var/log/ldap-manager/app.log | tail -10
```

---

## Connection Issues

### LDAP Server Connection Problems

#### Symptom: "Connection refused" or "Network unreachable"

**Diagnosis:**

```bash
# Test basic connectivity
telnet dc.example.com 636
nc -zv dc.example.com 636

# Test DNS resolution
nslookup dc.example.com
dig dc.example.com

# Test from container (if using Docker)
docker exec ldap-manager nc -zv dc.example.com 636
```

**Common Causes & Solutions:**

1. **Firewall blocking connection**

   ```bash
   # Check firewall rules
   iptables -L | grep 636
   ufw status

   # Solution: Open firewall port
   sudo ufw allow out 636
   iptables -A OUTPUT -p tcp --dport 636 -j ACCEPT
   ```

2. **DNS resolution failure**

   ```bash
   # Check DNS servers
   cat /etc/resolv.conf

   # Solution: Use IP address or fix DNS
   LDAP_SERVER=ldaps://192.168.1.10:636
   ```

3. **Network routing issues**

   ```bash
   # Test routing
   traceroute dc.example.com

   # Solution: Check network configuration
   ip route show
   ```

#### Symptom: "SSL/TLS handshake failure"

**Diagnosis:**

```bash
# Test SSL/TLS connection
openssl s_client -connect dc.example.com:636 -servername dc.example.com

# Check certificate details
openssl s_client -connect dc.example.com:636 -showcerts | openssl x509 -noout -text

# Test cipher compatibility
openssl s_client -connect dc.example.com:636 -cipher 'HIGH:!aNULL:!MD5'
```

**Common Causes & Solutions:**

1. **Certificate validation failure**

   ```bash
   # Check certificate chain
   openssl s_client -connect dc.example.com:636 -CApath /etc/ssl/certs/

   # Solution: Update CA certificates
   sudo apt update && sudo apt install ca-certificates
   sudo update-ca-certificates
   ```

2. **Self-signed certificate**

   ```bash
   # Solution: Add certificate to trust store
   echo | openssl s_client -connect dc.example.com:636 | openssl x509 > dc.crt
   sudo cp dc.crt /usr/local/share/ca-certificates/
   sudo update-ca-certificates
   ```

3. **TLS version mismatch**

   ```bash
   # Test different TLS versions
   openssl s_client -connect dc.example.com:636 -tls1_2
   openssl s_client -connect dc.example.com:636 -tls1_3

   # Solution: Configure TLS settings in LDAP client
   ```

### Application Connection Problems

#### Symptom: "Connection timeout"

**Diagnosis:**

```bash
# Check connection pool status
curl -s -H "Cookie: session=..." http://localhost:3000/debug/ldap-pool | jq '.stats'

# Monitor connection acquisition times
grep "connection.*timeout\|acquire.*timeout" /var/log/ldap-manager/app.log
```

**Solutions:**

1. **Increase connection pool size**

   ```bash
   LDAP_POOL_MAX_CONNECTIONS=20
   LDAP_POOL_MIN_CONNECTIONS=5
   ```

2. **Adjust timeout settings**

   ```bash
   LDAP_POOL_ACQUIRE_TIMEOUT=15s
   LDAP_CONNECT_TIMEOUT=10s
   ```

3. **Check LDAP server load**
   ```bash
   # Monitor LDAP server performance
   ldapsearch -H ldaps://dc.example.com -D "service_account" -W -b "" -s base "objectclass=*" currentTime
   ```

---

## Authentication Problems

### Login Failures

#### Symptom: "Authentication failed" for valid users

**Diagnosis:**

```bash
# Test user authentication directly
ldapwhoami -H ldaps://dc.example.com:636 -D "user@example.com" -W

# Check service account
ldapwhoami -H ldaps://dc.example.com:636 -D "CN=ldap-reader,OU=Service,DC=example,DC=com" -W

# Check application logs
grep "authentication.*failure" /var/log/ldap-manager/app.log | tail -10
```

**Common Causes & Solutions:**

1. **User account locked or disabled**

   ```bash
   # Check user account status
   ldapsearch -H ldaps://dc.example.com -D "admin@example.com" -W \
     -b "DC=example,DC=com" "(sAMAccountName=username)" userAccountControl

   # Solution: Unlock/enable account in LDAP/AD
   ```

2. **Password expired**

   ```bash
   # Check password policy
   ldapsearch -H ldaps://dc.example.com -D "admin@example.com" -W \
     -b "DC=example,DC=com" "(sAMAccountName=username)" pwdLastSet accountExpires
   ```

3. **Incorrect DN format**
   ```bash
   # Test different DN formats
   # UPN format: user@example.com
   # DN format: CN=User,OU=Users,DC=example,DC=com
   # sAMAccountName: DOMAIN\username
   ```

#### Symptom: "Service account authentication failed"

**Diagnosis:**

```bash
# Test service account directly
ldapsearch -H ldaps://dc.example.com:636 \
  -D "$LDAP_READONLY_USER" -w "$LDAP_READONLY_PASSWORD" \
  -b "$LDAP_BASE_DN" -s base "(objectclass=*)"
```

**Solutions:**

1. **Update service account password**

   ```bash
   # Generate new password and update configuration
   LDAP_READONLY_PASSWORD=new_secure_password
   ```

2. **Check service account permissions**
   ```powershell
   # PowerShell command for AD
   Get-ADUser -Identity "ldap-reader" -Properties MemberOf
   dsacls "OU=Users,DC=example,DC=com" | Select-String "ldap-reader"
   ```

### Session Problems

#### Symptom: "Session expired" messages

**Diagnosis:**

```bash
# Check session configuration
env | grep SESSION

# Monitor session activity
grep "session" /var/log/ldap-manager/app.log | tail -20

# Check session storage
ls -la /path/to/sessions.db  # for persistent sessions
```

**Solutions:**

1. **Increase session duration**

   ```bash
   SESSION_DURATION=1h  # or appropriate duration
   ```

2. **Fix session storage issues**

   ```bash
   # Check session file permissions
   chmod 600 /path/to/sessions.db
   chown ldap-manager:ldap-manager /path/to/sessions.db

   # For memory sessions, ensure adequate memory
   free -h
   ```

---

## Performance Issues

### Slow Response Times

#### Symptom: Pages loading slowly

**Diagnosis:**

```bash
# Measure response times
time curl -s http://localhost:3000/users > /dev/null

# Check cache performance
curl -s -H "Cookie: session=..." http://localhost:3000/debug/cache | jq '.hit_ratio'

# Monitor system resources
top -p $(pgrep ldap-manager)
iostat -x 1 5
```

**Solutions:**

1. **Optimize cache configuration**

   ```bash
   # Increase cache sizes if hit ratio is low
   TEMPLATE_CACHE_MAX_SIZE=1000
   TEMPLATE_CACHE_MAX_MEMORY=100MB

   # Adjust LDAP cache refresh interval
   LDAP_CACHE_REFRESH_INTERVAL=60s  # for stable environments
   ```

2. **Optimize connection pool**

   ```bash
   # Increase pool size for high concurrency
   LDAP_POOL_MAX_CONNECTIONS=20
   LDAP_POOL_MIN_CONNECTIONS=8

   # Keep connections alive longer
   LDAP_POOL_MAX_IDLE_TIME=30m
   ```

3. **System resource optimization**

   ```bash
   # Increase available memory
   GOMEMLIMIT=1GiB

   # Adjust garbage collection
   GOGC=200  # less frequent GC
   ```

### High Memory Usage

#### Symptom: Out of memory errors or high memory consumption

**Diagnosis:**

```bash
# Check memory usage
ps aux | grep ldap-manager
docker stats ldap-manager --no-stream

# Monitor memory growth
while true; do
  ps -o pid,vsz,rss,comm -p $(pgrep ldap-manager)
  sleep 30
done

# Check for memory leaks
go tool pprof http://localhost:3000/debug/pprof/heap  # if debug enabled
```

**Solutions:**

1. **Reduce cache sizes**

   ```bash
   TEMPLATE_CACHE_MAX_MEMORY=50MB
   LDAP_POOL_MAX_CONNECTIONS=10
   ```

2. **Optimize garbage collection**

   ```bash
   GOGC=50      # more aggressive GC
   GOMEMLIMIT=512MiB  # hard memory limit
   ```

3. **Check for connection leaks**
   ```bash
   # Monitor connection pool
   watch -n 5 "curl -s -H 'Cookie: session=...' http://localhost:3000/debug/ldap-pool | jq '.stats'"
   ```

### High CPU Usage

#### Symptom: Consistently high CPU usage

**Diagnosis:**

```bash
# Monitor CPU usage
top -p $(pgrep ldap-manager)
htop -p $(pgrep ldap-manager)

# Check for CPU-intensive operations
strace -c -p $(pgrep ldap-manager) 2>&1 | head -20

# Profile CPU usage (if debug enabled)
go tool pprof http://localhost:3000/debug/pprof/profile?seconds=30
```

**Solutions:**

1. **Optimize template caching**

   ```bash
   # Increase template cache to reduce rendering
   TEMPLATE_CACHE_MAX_SIZE=2000
   ```

2. **Reduce LDAP query frequency**

   ```bash
   # Increase cache refresh interval
   LDAP_CACHE_REFRESH_INTERVAL=120s
   ```

3. **Limit concurrent processing**
   ```bash
   GOMAXPROCS=4  # limit to 4 CPU cores
   ```

---

## Cache Problems

### Cache Performance Issues

#### Symptom: Low cache hit ratios

**Diagnosis:**

```bash
# Check cache statistics
curl -s -H "Cookie: session=..." http://localhost:3000/debug/cache | jq '.'

# Monitor cache behavior
grep "cache.*miss\|cache.*hit" /var/log/ldap-manager/app.log | tail -20
```

**Solutions:**

1. **Increase cache sizes**

   ```bash
   TEMPLATE_CACHE_MAX_SIZE=2000
   TEMPLATE_CACHE_MAX_MEMORY=200MB
   ```

2. **Optimize cache keys**

   ```bash
   # Check for cache key conflicts in logs
   grep "cache.*key" /var/log/ldap-manager/app.log | sort | uniq -c
   ```

3. **Adjust cache expiration**
   ```bash
   # Longer LDAP cache interval for stable directories
   LDAP_CACHE_REFRESH_INTERVAL=300s  # 5 minutes
   ```

### Cache Invalidation Problems

#### Symptom: Stale data displayed

**Diagnosis:**

```bash
# Check cache refresh activity
grep "cache.*refresh\|cache.*invalidate" /var/log/ldap-manager/app.log | tail -10

# Force cache refresh (restart application temporarily)
docker restart ldap-manager
```

**Solutions:**

1. **Reduce cache refresh interval**

   ```bash
   LDAP_CACHE_REFRESH_INTERVAL=15s  # more frequent refresh
   ```

2. **Manual cache invalidation**
   ```bash
   # Restart application to clear all caches
   systemctl restart ldap-manager
   # or
   docker restart ldap-manager
   ```

---

## Session Issues

### Session Storage Problems

#### Symptom: Sessions not persisting across restarts

**Diagnosis:**

```bash
# Check session configuration
echo "PERSIST_SESSIONS: $PERSIST_SESSIONS"
echo "SESSION_PATH: $SESSION_PATH"

# Check session file
ls -la "$SESSION_PATH"
file "$SESSION_PATH"
```

**Solutions:**

1. **Enable persistent sessions**

   ```bash
   PERSIST_SESSIONS=true
   SESSION_PATH=/app/data/sessions.db

   # Ensure directory exists and is writable
   mkdir -p /app/data
   chown ldap-manager:ldap-manager /app/data
   ```

2. **Fix session file permissions**
   ```bash
   chmod 600 /app/data/sessions.db
   chown ldap-manager:ldap-manager /app/data/sessions.db
   ```

#### Symptom: Session timeouts too frequent

**Solutions:**

```bash
# Increase session duration
SESSION_DURATION=2h

# For development (not recommended for production)
SESSION_DURATION=8h
```

---

## Container and Deployment Issues

### Docker Container Problems

#### Symptom: Container won't start

**Diagnosis:**

```bash
# Check container logs
docker logs ldap-manager

# Check container configuration
docker inspect ldap-manager

# Verify image
docker images | grep ldap-manager
```

**Common Issues & Solutions:**

1. **Permission denied errors**

   ```bash
   # Ensure non-root user can access mounted volumes
   chown -R 1000:1000 /opt/ldap-manager/data

   # Update Docker run command
   docker run -u 1000:1000 ...
   ```

2. **Environment variable issues**

   ```bash
   # Check environment variables
   docker exec ldap-manager env | grep LDAP

   # Solution: Fix .env file or Docker command
   docker run --env-file .env ...
   ```

3. **Port binding conflicts**

   ```bash
   # Check port usage
   netstat -tlnp | grep :3000

   # Solution: Use different port
   docker run -p 3001:3000 ...
   ```

#### Symptom: Container health checks failing

**Diagnosis:**

```bash
# Check health status
docker ps | grep ldap-manager

# Manual health check
docker exec ldap-manager wget --spider http://localhost:3000/health
```

**Solutions:**

1. **Fix health check endpoint**

   ```bash
   # Test health endpoint manually
   docker exec ldap-manager curl http://localhost:3000/health
   ```

2. **Adjust health check timing**
   ```dockerfile
   HEALTHCHECK --interval=60s --timeout=10s --start-period=30s --retries=3 \
       CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1
   ```

### Kubernetes Deployment Issues

#### Symptom: Pods failing to start

**Diagnosis:**

```bash
# Check pod status
kubectl get pods -l app=ldap-manager

# Check pod logs
kubectl logs -l app=ldap-manager --tail=50

# Describe pod for events
kubectl describe pod ldap-manager-xxx
```

**Common Solutions:**

1. **Resource constraints**

   ```yaml
   resources:
     requests:
       memory: "512Mi"
       cpu: "200m"
     limits:
       memory: "1Gi"
       cpu: "1000m"
   ```

2. **ConfigMap/Secret issues**

   ```bash
   # Check ConfigMap
   kubectl get configmap ldap-manager-config -o yaml

   # Check Secret
   kubectl get secret ldap-manager-secret -o yaml
   ```

3. **Persistent Volume issues**

   ```bash
   # Check PVC status
   kubectl get pvc

   # Check PV binding
   kubectl get pv
   ```

---

## LDAP-Specific Problems

### Active Directory Issues

#### Symptom: "Referral" errors

**Diagnosis:**

```bash
# Check for referrals in logs
grep -i referral /var/log/ldap-manager/app.log

# Test LDAP search with referral following
ldapsearch -H ldaps://dc.example.com -D "user@example.com" -W \
  -b "DC=example,DC=com" "(objectclass=user)" -C
```

**Solutions:**

1. **Configure referral following**

   ```bash
   # Use Global Catalog port
   LDAP_SERVER=ldaps://dc.example.com:3269
   ```

2. **Target specific domain controller**
   ```bash
   # Use specific DC instead of DNS alias
   LDAP_SERVER=ldaps://dc01.example.com:636
   ```

#### Symptom: "Size limit exceeded"

**Solutions:**

```bash
# Request size limit increase from AD admin, or
# Use paged results (automatic in simple-ldap-go)
# Check if service account has appropriate permissions
```

### OpenLDAP Issues

#### Symptom: "Insufficient access rights"

**Diagnosis:**

```bash
# Test service account permissions
ldapsearch -H ldaps://ldap.example.com -D "$LDAP_READONLY_USER" -W \
  -b "$LDAP_BASE_DN" "(objectclass=organizationalPerson)" cn
```

**Solutions:**

1. **Update ACLs**

   ```ldif
   # Add read access for service account
   dn: olcDatabase={1}mdb,cn=config
   changetype: modify
   add: olcAccess
   olcAccess: to * by dn="cn=ldap-manager,ou=System,dc=example,dc=com" read
   ```

2. **Check service account DN format**
   ```bash
   # Ensure correct DN format for OpenLDAP
   LDAP_READONLY_USER=cn=ldap-manager,ou=System,dc=example,dc=com
   ```

---

## Monitoring and Logging

### Log Analysis

#### Enable Debug Logging Temporarily

```bash
# Enable debug logging
export LOG_LEVEL=debug
# or restart with debug level
docker restart ldap-manager

# Analyze logs
tail -f /var/log/ldap-manager/app.log | grep -E "(ERROR|WARN|DEBUG)"
```

#### Common Log Patterns

```bash
# Authentication events
grep -E "authentication.*(success|failure)" /var/log/ldap-manager/app.log

# LDAP connection events
grep -E "ldap.*(connect|disconnect|error)" /var/log/ldap-manager/app.log

# Performance events
grep -E "response_time|slow|timeout" /var/log/ldap-manager/app.log

# Cache events
grep -E "cache.*(hit|miss|refresh)" /var/log/ldap-manager/app.log
```

#### Log Rotation

```bash
# Setup logrotate for application logs
cat > /etc/logrotate.d/ldap-manager << EOF
/var/log/ldap-manager/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
EOF
```

### Monitoring Setup

#### Basic Monitoring Script

```bash
#!/bin/bash
# monitoring.sh - Basic LDAP Manager monitoring

monitor() {
    echo "LDAP Manager Health Check - $(date)"
    echo "=================================="

    # Application health
    if curl -f http://localhost:3000/health > /dev/null 2>&1; then
        echo "✓ Application: Healthy"
    else
        echo "✗ Application: Unhealthy"
        return 1
    fi

    # LDAP connectivity
    if curl -f http://localhost:3000/health/ready > /dev/null 2>&1; then
        echo "✓ LDAP: Connected"
    else
        echo "✗ LDAP: Connection issues"
        return 1
    fi

    # Performance check
    response_time=$(curl -o /dev/null -s -w '%{time_total}' http://localhost:3000/health)
    if (( $(echo "$response_time > 2.0" | bc -l) )); then
        echo "⚠ Performance: Slow (${response_time}s)"
    else
        echo "✓ Performance: Good (${response_time}s)"
    fi

    # Resource usage
    memory=$(ps -o rss= -p $(pgrep ldap-manager) | awk '{print $1/1024 "MB"}')
    echo "ℹ Memory Usage: $memory"

    return 0
}

# Run monitoring
if monitor; then
    echo "All checks passed"
    exit 0
else
    echo "Health check failed - investigate immediately"
    exit 1
fi
```

---

## Emergency Procedures

### Service Recovery

#### Immediate Recovery Steps

```bash
# 1. Check if service is running
systemctl status ldap-manager
# or for Docker
docker ps | grep ldap-manager

# 2. If stopped, attempt restart
systemctl restart ldap-manager
# or for Docker
docker restart ldap-manager

# 3. Monitor startup
tail -f /var/log/ldap-manager/app.log
# or for Docker
docker logs -f ldap-manager

# 4. Test functionality
curl http://localhost:3000/health
curl http://localhost:3000/health/ready
```

#### Service Won't Start

```bash
# Check configuration
ldap-manager --help  # if available
env | grep LDAP

# Check dependencies
systemctl status network
systemctl status docker  # if using Docker

# Check resources
df -h
free -h
ulimit -n

# Check logs for startup errors
journalctl -u ldap-manager -f
# or
docker logs ldap-manager 2>&1 | grep -i error
```

### Data Recovery

#### Session Data Recovery

```bash
# If sessions.db is corrupted
# 1. Stop service
systemctl stop ldap-manager

# 2. Backup corrupted file
cp /app/data/sessions.db /app/data/sessions.db.corrupt.$(date +%Y%m%d)

# 3. Remove corrupted file
rm /app/data/sessions.db

# 4. Restart service (will create new session file)
systemctl start ldap-manager

# Note: All users will need to log in again
```

#### Configuration Recovery

```bash
# Restore from backup
cp /backup/.env.$(date +%Y%m%d) .env

# Or recreate minimal configuration
cat > .env << EOF
LDAP_SERVER=ldaps://dc.example.com:636
LDAP_BASE_DN=DC=example,DC=com
LDAP_IS_AD=true
LDAP_READONLY_USER=CN=service,OU=Service,DC=example,DC=com
LDAP_READONLY_PASSWORD=service_password
LOG_LEVEL=info
EOF
```

### Rollback Procedures

#### Application Rollback

```bash
# Docker rollback to previous version
docker stop ldap-manager
docker run -d --name ldap-manager-new \
  --env-file .env \
  -p 3000:3000 \
  ldap-manager:previous-version

# Test rollback
curl http://localhost:3000/health

# If successful, remove old container
docker rm ldap-manager
docker rename ldap-manager-new ldap-manager
```

#### Configuration Rollback

```bash
# Restore previous configuration
git checkout HEAD~1 .env  # if using git
# or
cp .env.backup .env

# Restart with previous configuration
systemctl restart ldap-manager
```

### Emergency Contacts and Escalation

#### Emergency Checklist

1. **Immediate Response (0-5 minutes)**
   - Check service status
   - Attempt service restart
   - Verify basic connectivity

2. **Initial Investigation (5-15 minutes)**
   - Check logs for errors
   - Verify LDAP server connectivity
   - Check system resources

3. **Escalation (15+ minutes)**
   - Contact system administrator
   - Contact LDAP/AD administrator
   - Consider rollback procedures

4. **Communication**
   - Notify affected users
   - Update incident tracking system
   - Document resolution steps

---

This troubleshooting guide covers the most common operational issues with LDAP Manager. For additional support, consult the [Configuration Reference](../user-guide/configuration.md), [Performance Guide](performance-optimization.md), and [Security Guide](security-configuration.md).

Remember to always test solutions in a development environment before applying to production systems.
