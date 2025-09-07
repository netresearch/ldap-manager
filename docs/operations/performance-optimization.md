# Performance Optimization Guide

Comprehensive guide for optimizing LDAP Manager performance in production environments.

## Table of Contents

- [Performance Overview](#performance-overview)
- [Connection Pool Optimization](#connection-pool-optimization)
- [Cache Configuration](#cache-configuration)
- [Template Performance](#template-performance)
- [Memory Management](#memory-management)
- [Network Optimization](#network-optimization)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Scaling Strategies](#scaling-strategies)
- [Troubleshooting](#troubleshooting)

---

## Performance Overview

### Performance Characteristics

LDAP Manager is designed for high-performance directory operations with the following targets:

- **Response Times**: <100ms for cached operations, <500ms for LDAP queries
- **Throughput**: 1000+ concurrent users with optimal configuration
- **Memory Usage**: <512MB for typical deployment (10,000 users)
- **CPU Usage**: <10% on modern hardware for normal load

### Key Performance Features

- **Connection Pooling**: Reuse expensive LDAP connections
- **Multi-Level Caching**: LDAP data, rendered templates, static assets
- **Compiled Templates**: Zero runtime parsing overhead
- **Efficient Data Structures**: O(1) lookups with indexed caches
- **Concurrent Operations**: Thread-safe parallel processing

---

## Connection Pool Optimization

### Default Configuration

```bash
# Production-optimized connection pool settings
LDAP_POOL_MAX_CONNECTIONS=20        # Maximum concurrent connections
LDAP_POOL_MIN_CONNECTIONS=5         # Always-available connections
LDAP_POOL_MAX_IDLE_TIME=15m         # Close idle connections after 15 minutes
LDAP_POOL_MAX_LIFETIME=1h           # Rotate connections every hour
LDAP_POOL_HEALTH_CHECK_INTERVAL=30s # Check connection health every 30 seconds
LDAP_POOL_ACQUIRE_TIMEOUT=10s       # Timeout for connection acquisition
```

### Sizing Guidelines

#### Small Deployment (< 100 users)

```bash
LDAP_POOL_MAX_CONNECTIONS=5
LDAP_POOL_MIN_CONNECTIONS=2
LDAP_POOL_MAX_IDLE_TIME=30m
```

#### Medium Deployment (100-1000 users)

```bash
LDAP_POOL_MAX_CONNECTIONS=10
LDAP_POOL_MIN_CONNECTIONS=3
LDAP_POOL_MAX_IDLE_TIME=15m
```

#### Large Deployment (1000+ users)

```bash
LDAP_POOL_MAX_CONNECTIONS=20
LDAP_POOL_MIN_CONNECTIONS=5
LDAP_POOL_MAX_IDLE_TIME=10m
```

### Connection Pool Monitoring

Monitor pool performance with debug endpoints:

```bash
# Get connection pool statistics
curl -H "Cookie: session=..." http://localhost:3000/debug/ldap-pool

# Example response
{
  "stats": {
    "total_connections": 8,
    "active_connections": 3,
    "available_connections": 5,
    "acquired_count": 1247,
    "failed_count": 2,
    "max_connections": 10
  },
  "health": {
    "healthy": true,
    "total_connections": 8,
    "error_rate": 0.0016
  }
}
```

### Performance Tuning

#### High-Load Optimization

```bash
# Increase pool size for high concurrent load
LDAP_POOL_MAX_CONNECTIONS=30
LDAP_POOL_MIN_CONNECTIONS=10

# Reduce connection lifetime for better load distribution
LDAP_POOL_MAX_LIFETIME=30m

# More frequent health checks for reliability
LDAP_POOL_HEALTH_CHECK_INTERVAL=15s
```

#### Low-Latency Optimization

```bash
# Maintain more warm connections
LDAP_POOL_MIN_CONNECTIONS=8

# Keep connections longer to avoid reconnection overhead
LDAP_POOL_MAX_IDLE_TIME=45m
LDAP_POOL_MAX_LIFETIME=2h

# Faster acquisition timeout
LDAP_POOL_ACQUIRE_TIMEOUT=5s
```

---

## Cache Configuration

### LDAP Data Cache

The LDAP data cache dramatically reduces directory server load:

```bash
# Cache refresh interval (default: 30s)
# Lower values = fresher data but higher LDAP load
# Higher values = better performance but staler data
LDAP_CACHE_REFRESH_INTERVAL=30s

# For high-change environments
LDAP_CACHE_REFRESH_INTERVAL=15s

# For stable environments
LDAP_CACHE_REFRESH_INTERVAL=60s
```

### Template Cache

Template caching is automatically configured but can be monitored:

```bash
# Get template cache statistics
curl -H "Cookie: session=..." http://localhost:3000/debug/cache

# Example response
{
  "hits": 8432,
  "misses": 1247,
  "hit_ratio": 0.871,
  "entries": 156,
  "memory_usage": "12.4MB",
  "evictions": 23
}
```

### Cache Performance Metrics

Monitor these key metrics for optimal cache performance:

- **Hit Ratio**: Should be >80% for template cache, >95% for LDAP cache
- **Eviction Rate**: High evictions indicate undersized cache
- **Memory Usage**: Monitor for memory pressure
- **Refresh Times**: LDAP cache refresh should complete in <5s

### Cache Tuning

#### High-Traffic Optimization

```bash
# Increase cache memory limits
TEMPLATE_CACHE_MAX_SIZE=1000        # Number of cached templates
TEMPLATE_CACHE_MAX_MEMORY=100MB     # Maximum memory usage

# Reduce LDAP refresh interval for fresher data
LDAP_CACHE_REFRESH_INTERVAL=20s
```

#### Memory-Constrained Environment

```bash
# Reduce cache sizes
TEMPLATE_CACHE_MAX_SIZE=200
TEMPLATE_CACHE_MAX_MEMORY=25MB

# Longer LDAP refresh to reduce memory churn
LDAP_CACHE_REFRESH_INTERVAL=60s
```

---

## Template Performance

### Compiled Template System

LDAP Manager uses [templ](https://templ.guide/) for zero-runtime-overhead templates:

- **Compile-Time Generation**: Templates compiled to Go code
- **Type Safety**: No runtime template parsing errors
- **Automatic Escaping**: XSS protection built-in
- **Optimal Performance**: Direct Go function calls

### Template Caching Strategy

Templates are cached with intelligent cache keys:

```
Cache Key Format:
[path]:[method]:[user_context]:[query_params]:[additional_data]

Examples:
/users:GET:CN=admin,OU=Users,DC=example,DC=com::
/users:GET:CN=admin,OU=Users,DC=example,DC=com:show-disabled=1:
/users/CN=John%20Doe,OU=Users,DC=example,DC=com:GET:::userDN:CN=John Doe,OU=Users,DC=example,DC=com
```

### Cache Invalidation

Smart cache invalidation ensures data consistency:

- **Path-Based Invalidation**: Clear caches for affected URLs
- **User-Context Invalidation**: Clear user-specific cached content
- **Automatic Invalidation**: After any data modification

### Performance Best Practices

1. **Minimize Template Complexity**: Keep templates simple for faster rendering
2. **Efficient Data Structures**: Use indexed lookups in template data
3. **Batch Operations**: Group related template operations
4. **Monitor Cache Ratios**: Maintain >80% hit ratio for optimal performance

---

## Memory Management

### Memory Usage Patterns

```
Typical Memory Distribution:
├── Application Code: ~50MB (fixed)
├── LDAP Data Cache: ~100MB (10,000 users)
├── Template Cache: ~50MB (500 templates)
├── Connection Pool: ~20MB (10 connections)
├── Session Storage: ~10MB (1000 sessions)
└── Go Runtime: ~30MB (GC overhead)
Total: ~260MB
```

### Memory Configuration

Control memory usage with environment variables:

```bash
# Go runtime memory settings
GOGC=100                    # GC target percentage (default: 100)
GOMEMLIMIT=512MiB          # Soft memory limit
GOMAXPROCS=0               # Use all available CPUs

# Application memory limits
TEMPLATE_CACHE_MAX_MEMORY=100MB
SESSION_CACHE_MAX_SIZE=10000
```

### Memory Monitoring

Monitor memory usage patterns:

```bash
# Application metrics endpoint
curl http://localhost:3000/debug/metrics

# System memory monitoring
# Monitor RSS (Resident Set Size) for actual memory usage
ps aux | grep ldap-manager

# Container memory monitoring
docker stats ldap-manager
```

### Memory Optimization

#### Low-Memory Environment (<256MB)

```bash
# Reduce cache sizes
TEMPLATE_CACHE_MAX_MEMORY=25MB
LDAP_POOL_MAX_CONNECTIONS=5

# Aggressive garbage collection
GOGC=50

# Smaller session limits
SESSION_CACHE_MAX_SIZE=1000
```

#### High-Memory Environment (>1GB)

```bash
# Increase cache sizes for better performance
TEMPLATE_CACHE_MAX_MEMORY=200MB
LDAP_POOL_MAX_CONNECTIONS=30

# Relaxed garbage collection
GOGC=200

# Larger session cache
SESSION_CACHE_MAX_SIZE=50000
```

---

## Network Optimization

### LDAP Network Configuration

Optimize LDAP network performance:

```bash
# LDAP connection settings
LDAP_SERVER=ldaps://dc.example.com:636    # Use LDAPS for encryption + performance
LDAP_CONNECT_TIMEOUT=10s                  # Connection timeout
LDAP_READ_TIMEOUT=30s                     # Read timeout
LDAP_WRITE_TIMEOUT=30s                    # Write timeout
```

### Network Tuning

#### Low-Latency Network

```bash
# Faster timeouts for responsive network
LDAP_CONNECT_TIMEOUT=5s
LDAP_READ_TIMEOUT=15s
LDAP_POOL_ACQUIRE_TIMEOUT=5s
```

#### High-Latency Network

```bash
# Longer timeouts for unreliable network
LDAP_CONNECT_TIMEOUT=30s
LDAP_READ_TIMEOUT=60s
LDAP_POOL_ACQUIRE_TIMEOUT=15s

# Keep connections alive longer
LDAP_POOL_MAX_IDLE_TIME=45m
```

### HTTP Performance

Optimize HTTP layer performance:

```bash
# Fiber configuration (environment variables)
FIBER_PREFORK=false           # Use default single-process mode
FIBER_BODY_LIMIT=4096        # 4KB body limit (sufficient for forms)
FIBER_READ_TIMEOUT=60s       # HTTP read timeout
FIBER_WRITE_TIMEOUT=60s      # HTTP write timeout
```

---

## Monitoring and Metrics

### Key Performance Indicators (KPIs)

Monitor these critical metrics:

#### Response Time Metrics

- **P50 Response Time**: <100ms for cached requests
- **P95 Response Time**: <500ms for LDAP queries
- **P99 Response Time**: <1000ms under normal load

#### Throughput Metrics

- **Requests Per Second**: Baseline throughput capability
- **Concurrent Users**: Number of active sessions
- **LDAP Operations/sec**: Directory operation rate

#### Resource Metrics

- **CPU Usage**: <50% under normal load
- **Memory Usage**: <80% of allocated memory
- **Connection Pool Utilization**: <80% of max connections
- **Cache Hit Ratios**: >80% template cache, >95% LDAP cache

### Monitoring Setup

#### Built-in Debug Endpoints

```bash
# Template cache statistics
curl -H "Cookie: session=..." http://localhost:3000/debug/cache

# LDAP connection pool statistics
curl -H "Cookie: session=..." http://localhost:3000/debug/ldap-pool

# Health check endpoints (no auth required)
curl http://localhost:3000/health        # Basic health
curl http://localhost:3000/health/ready  # Readiness probe
curl http://localhost:3000/health/live   # Liveness probe
```

#### Application Metrics

Enable detailed logging for performance analysis:

```bash
# Enable debug logging
LOG_LEVEL=debug

# Log analysis examples
# Monitor response times
grep "response_time" logs/app.log | jq '.response_time'

# Monitor cache performance
grep "cache_hit" logs/app.log | jq '.cache_hit'

# Monitor LDAP operations
grep "ldap_query" logs/app.log | jq '.query_time'
```

#### External Monitoring

Integrate with monitoring systems:

```bash
# Prometheus metrics (if enabled)
curl http://localhost:3000/metrics

# Health check monitoring
while true; do
  curl -f http://localhost:3000/health/ready || echo "UNHEALTHY"
  sleep 30
done
```

### Performance Alerting

Set up alerts for critical thresholds:

- **Response Time**: Alert if P95 > 1000ms for 5 minutes
- **Error Rate**: Alert if error rate > 5% for 2 minutes
- **Cache Hit Ratio**: Alert if LDAP cache < 90% for 10 minutes
- **Connection Pool**: Alert if pool utilization > 90% for 5 minutes
- **Memory Usage**: Alert if memory > 90% of limit for 5 minutes

---

## Scaling Strategies

### Vertical Scaling

#### CPU Scaling

- **2 CPUs**: Suitable for <500 users
- **4 CPUs**: Suitable for <2000 users
- **8+ CPUs**: Required for >2000 users

#### Memory Scaling

- **512MB**: Minimum for production
- **1GB**: Recommended for 1000+ users
- **2GB+**: Required for large deployments with extensive caching

#### Network Scaling

- **100Mbps**: Sufficient for most deployments
- **1Gbps**: Required for high-traffic or geographically distributed deployments

### Horizontal Scaling

LDAP Manager supports horizontal scaling with these considerations:

#### Stateless Design

- **No Inter-Instance Dependencies**: Each instance operates independently
- **Shared Session Storage**: Use persistent session storage (BoltDB file on shared storage)
- **Cache Independence**: Each instance maintains its own LDAP and template caches

#### Load Balancing Configuration

```nginx
# Nginx load balancer configuration
upstream ldap-manager {
    least_conn;                    # Use least connections algorithm
    server ldap-manager-1:3000;
    server ldap-manager-2:3000;
    server ldap-manager-3:3000;
}

server {
    listen 443 ssl;
    server_name ldap-manager.example.com;

    location / {
        proxy_pass http://ldap-manager;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Session affinity for better cache performance (optional)
        ip_hash;
    }

    location /health {
        proxy_pass http://ldap-manager;
        proxy_set_header Host $host;
    }
}
```

#### Container Orchestration

```yaml
# Kubernetes deployment example
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-manager
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
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "1000m"
              memory: "1Gi"
          env:
            - name: LDAP_POOL_MAX_CONNECTIONS
              value: "15" # Slightly less per instance
            - name: PERSIST_SESSIONS
              value: "true"
            - name: SESSION_PATH
              value: "/shared/sessions.db" # Shared persistent storage
          livenessProbe:
            httpGet:
              path: /health/live
              port: 3000
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 5
```

### Auto-Scaling

Configure auto-scaling based on performance metrics:

#### CPU-Based Scaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ldap-manager-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ldap-manager
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

#### Custom Metrics Scaling

- **Response Time**: Scale up if P95 response time > 500ms
- **Request Rate**: Scale up if requests/sec > 100 per instance
- **Connection Pool**: Scale up if pool utilization > 80%

---

## Troubleshooting

### Common Performance Issues

#### High Response Times

**Symptoms**:

- Slow page loads
- User complaints about responsiveness
- High P95/P99 response times in metrics

**Diagnosis**:

```bash
# Check cache hit ratios
curl -H "Cookie: session=..." http://localhost:3000/debug/cache

# Check LDAP connection pool utilization
curl -H "Cookie: session=..." http://localhost:3000/debug/ldap-pool

# Monitor LDAP query times in logs
grep "ldap_query" logs/app.log | jq '.query_time' | sort -n
```

**Solutions**:

1. **Low Cache Hit Ratio**: Increase cache sizes, reduce refresh intervals
2. **High Pool Utilization**: Increase connection pool size
3. **Slow LDAP Queries**: Optimize LDAP server, check network latency
4. **High Memory Usage**: Increase memory limits or reduce cache sizes

#### Memory Issues

**Symptoms**:

- Out of memory errors
- Frequent garbage collection
- Container restarts due to memory limits

**Diagnosis**:

```bash
# Monitor memory usage
docker stats ldap-manager

# Check cache memory usage
curl -H "Cookie: session=..." http://localhost:3000/debug/cache | jq '.memory_usage'

# Enable memory profiling (development only)
go tool pprof http://localhost:3000/debug/pprof/heap
```

**Solutions**:

1. **High Cache Memory**: Reduce cache sizes, implement more aggressive eviction
2. **Memory Leaks**: Check for unclosed LDAP connections, review recent code changes
3. **Too Small Limits**: Increase memory limits for container/process

#### Connection Pool Issues

**Symptoms**:

- Connection timeout errors
- High connection acquisition times
- LDAP authentication failures

**Diagnosis**:

```bash
# Check pool health and statistics
curl -H "Cookie: session=..." http://localhost:3000/debug/ldap-pool

# Monitor connection errors in logs
grep "connection" logs/app.log | grep "error"

# Test LDAP connectivity directly
ldapsearch -H ldaps://dc.example.com -D "service_account" -W -b "dc=example,dc=com" "(objectclass=user)" cn
```

**Solutions**:

1. **Pool Exhaustion**: Increase max connections, reduce idle time
2. **LDAP Server Overload**: Implement connection throttling, distribute load
3. **Network Issues**: Check firewall rules, DNS resolution, certificate validity
4. **Authentication Issues**: Verify service account credentials and permissions

### Performance Testing

#### Load Testing Setup

```bash
# Install Apache Bench
apt-get install apache2-utils

# Basic load test (authenticated endpoint requires session cookie)
ab -n 1000 -c 10 http://localhost:3000/health

# Load test with session cookie
ab -n 1000 -c 10 -C "session=your_session_cookie_here" http://localhost:3000/users
```

#### Stress Testing

```bash
# Install wrk for more advanced testing
wget https://github.com/wg/wrk/releases/latest
make && sudo cp wrk /usr/local/bin/

# Stress test with custom script
wrk -t12 -c400 -d30s --script=auth_test.lua http://localhost:3000/
```

#### Performance Baselines

Establish performance baselines for comparison:

```bash
# Baseline test script
#!/bin/bash
echo "Starting baseline performance test..."

# Test health endpoint
echo "Health endpoint (no auth):"
ab -n 100 -c 5 http://localhost:3000/health

# Test authenticated endpoint (replace with valid session)
echo "Users endpoint (authenticated):"
ab -n 100 -c 5 -C "session=valid_session_here" http://localhost:3000/users

# Test static assets
echo "Static assets:"
ab -n 100 -c 5 http://localhost:3000/static/styles.css

echo "Baseline test complete."
```

#### Regression Testing

Set up automated performance regression testing:

```bash
# Performance regression test
#!/bin/bash
BASELINE_P95=500  # 500ms baseline P95 response time

# Run load test and capture P95
P95=$(wrk -t4 -c20 -d30s --latency http://localhost:3000/health | grep "99.00%" | awk '{print $2}')

# Convert to milliseconds and compare
P95_MS=$(echo "$P95" | sed 's/ms//')
if (( $(echo "$P95_MS > $BASELINE_P95" | bc -l) )); then
    echo "PERFORMANCE REGRESSION: P95 ($P95_MS ms) exceeds baseline ($BASELINE_P95 ms)"
    exit 1
else
    echo "Performance test passed: P95 = $P95_MS ms"
fi
```

---

This performance optimization guide provides comprehensive strategies for maximizing LDAP Manager performance. Regular monitoring and tuning based on actual usage patterns will ensure optimal performance in your specific environment.

For additional performance insights, see the [Monitoring Guide](monitoring.md) and [Architecture Documentation](../development/architecture-detailed.md).
