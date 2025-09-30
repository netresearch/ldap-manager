# Code Examples: Credential-Aware Connection Pooling

This document provides comprehensive code examples demonstrating the usage of credential-aware connection pooling for various scenarios.

## Table of Contents

1. [Basic Single-User Pooling (Existing API)](#basic-single-user-pooling)
2. [Multi-User Web Application](#multi-user-web-application)
3. [HTTP Handler with Per-Request User](#http-handler-with-per-request-user)
4. [Multi-Tenant System](#multi-tenant-system)
5. [Connection Pool Monitoring](#connection-pool-monitoring)
6. [Error Handling Best Practices](#error-handling-best-practices)

---

## Basic Single-User Pooling

### Existing API (Unchanged)

This example shows that existing code continues to work without any modifications.

```go
package main

import (
    "fmt"
    "log"
    "time"

    ldap "github.com/netresearch/simple-ldap-go"
)

func main() {
    // Configure connection pool
    poolConfig := &ldap.PoolConfig{
        MaxConnections:      10,
        MinConnections:      2,
        MaxIdleTime:         15 * time.Minute,
        MaxLifetime:         1 * time.Hour,
        HealthCheckInterval: 30 * time.Second,
        GetTimeout:          10 * time.Second,
    }

    // Configure LDAP connection
    ldapConfig := ldap.Config{
        Host:     "ldap.example.com",
        Port:     389,
        BaseDN:   "dc=example,dc=com",
        UserDN:   "cn=admin,dc=example,dc=com",
        Password: "admin_password",
    }

    // Create connection pool
    pool := ldap.NewConnectionPool(poolConfig, ldapConfig)
    defer pool.Close()

    // Use existing Get() method - works exactly as before
    conn, err := pool.Get()
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Put(conn)

    // Perform LDAP operations
    user, err := conn.FindUserBySAMAccountName("jdoe")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found user: %s\n", user.DisplayName)
}
```

**Key Points:**
- ✅ No code changes required
- ✅ Existing Get() method works as before
- ✅ 100% backward compatible

---

## Multi-User Web Application

### HTTP Handler with Credential-Aware Pooling

This example shows a web application where each user authenticates with their own LDAP credentials.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    ldap "github.com/netresearch/simple-ldap-go"
)

// Global connection pool (initialized at startup)
var ldapPool *ldap.ConnectionPool

func init() {
    poolConfig := &ldap.PoolConfig{
        MaxConnections:      50,  // Higher limit for web app
        MinConnections:      5,
        MaxIdleTime:         15 * time.Minute,
        MaxLifetime:         1 * time.Hour,
        HealthCheckInterval: 30 * time.Second,
        GetTimeout:          5 * time.Second,
    }

    ldapConfig := ldap.Config{
        Host:   "ldap.example.com",
        Port:   389,
        BaseDN: "dc=example,dc=com",
        // Note: No UserDN/Password here - using per-user credentials
    }

    ldapPool = ldap.NewConnectionPool(poolConfig, ldapConfig)
}

// User credentials from session/JWT
type UserContext struct {
    DN       string
    Password string
}

// HTTP handler for user profile
func getUserProfileHandler(w http.ResponseWriter, r *http.Request) {
    // Extract user credentials from session/JWT
    userCtx, ok := r.Context().Value("user").(UserContext)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Get connection with user's credentials
    conn, err := ldapPool.GetWithCredentials(userCtx.DN, userCtx.Password)
    if err != nil {
        log.Printf("Failed to get LDAP connection: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    defer ldapPool.Put(conn)

    // Perform user-specific operation
    username := r.URL.Query().Get("username")
    user, err := conn.FindUserBySAMAccountName(username)
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    fmt.Fprintf(w, "User: %s (%s)\n", user.DisplayName, user.Email)
}

// HTTP handler for group management
func updateUserGroupHandler(w http.ResponseWriter, r *http.Request) {
    userCtx, ok := r.Context().Value("user").(UserContext)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Each request gets a connection with appropriate credentials
    // Connections are efficiently reused when the same user makes multiple requests
    conn, err := ldapPool.GetWithCredentials(userCtx.DN, userCtx.Password)
    if err != nil {
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    defer ldapPool.Put(conn)

    // Perform authenticated operation
    // User's permissions are enforced by LDAP server based on their credentials
    // ...
}

func main() {
    http.HandleFunc("/api/user/profile", getUserProfileHandler)
    http.HandleFunc("/api/user/groups", updateUserGroupHandler)

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**Key Benefits:**
- ✅ Each user's operations use their own credentials
- ✅ LDAP permissions enforced per user
- ✅ Efficient connection reuse for same user across requests
- ✅ Prevents credential mixing security issues

---

## HTTP Handler with Per-Request User

### Middleware Pattern

```go
package main

import (
    "context"
    "net/http"

    ldap "github.com/netresearch/simple-ldap-go"
)

// Middleware to attach LDAP connection to request context
func ldapConnectionMiddleware(pool *ldap.ConnectionPool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract credentials from authentication token
            userDN, password, ok := extractUserCredentials(r)
            if !ok {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            // Get connection with user's credentials
            conn, err := pool.GetWithCredentials(userDN, password)
            if err != nil {
                http.Error(w, "Authentication failed", http.StatusUnauthorized)
                return
            }

            // Add connection to request context
            ctx := context.WithValue(r.Context(), "ldapConn", conn)

            // Ensure connection is returned to pool
            defer pool.Put(conn)

            // Call next handler
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Helper to extract user credentials from request (JWT, session, etc.)
func extractUserCredentials(r *http.Request) (dn, password string, ok bool) {
    // Example: Extract from JWT token
    // token := r.Header.Get("Authorization")
    // claims := parseJWT(token)
    // return claims.DN, claims.Password, true

    return "cn=user,dc=example,dc=com", "password", true
}

// Handler that uses LDAP connection from context
func protectedHandler(w http.ResponseWriter, r *http.Request) {
    conn, ok := r.Context().Value("ldapConn").(*ldap.Conn)
    if !ok {
        http.Error(w, "Internal error", http.StatusInternalServerError)
        return
    }

    // Use connection - already authenticated with user's credentials
    user, err := conn.FindUserBySAMAccountName("jdoe")
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    // ... handle user data
}
```

---

## Multi-Tenant System

### Tenant-Specific LDAP Operations

```go
package main

import (
    "fmt"
    "log"

    ldap "github.com/netresearch/simple-ldap-go"
)

type TenantService struct {
    pool *ldap.ConnectionPool
}

func NewTenantService(pool *ldap.ConnectionPool) *TenantService {
    return &TenantService{pool: pool}
}

// Perform operation on behalf of a specific tenant
func (s *TenantService) GetTenantUsers(tenantDN, tenantPassword string) ([]string, error) {
    // Get connection authenticated as tenant service account
    conn, err := s.pool.GetWithCredentials(tenantDN, tenantPassword)
    if err != nil {
        return nil, fmt.Errorf("tenant authentication failed: %w", err)
    }
    defer s.pool.Put(conn)

    // Perform tenant-specific query
    // Each tenant sees only their own users based on their credentials
    users, err := conn.Search(&ldap.SearchRequest{
        BaseDN: tenantDN,
        Filter: "(objectClass=person)",
        Scope:  ldap.ScopeWholeSubtree,
    })

    if err != nil {
        return nil, err
    }

    var usernames []string
    for _, user := range users {
        usernames = append(usernames, user.DN)
    }

    return usernames, nil
}

// Example: Multi-tenant application
func main() {
    pool := ldap.NewConnectionPool(poolConfig, ldapConfig)
    defer pool.Close()

    service := NewTenantService(pool)

    // Tenant A operations
    tenantAUsers, err := service.GetTenantUsers(
        "cn=tenantA-service,ou=tenants,dc=example,dc=com",
        "tenantA_password",
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Tenant A users: %v\n", tenantAUsers)

    // Tenant B operations (uses different connection)
    tenantBUsers, err := service.GetTenantUsers(
        "cn=tenantB-service,ou=tenants,dc=example,dc=com",
        "tenantB_password",
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Tenant B users: %v\n", tenantBUsers)

    // Tenant A operations again (efficiently reuses first connection)
    tenantAUsers2, err := service.GetTenantUsers(
        "cn=tenantA-service,ou=tenants,dc=example,dc=com",
        "tenantA_password",
    )
    // ... same connection as first tenantA call
}
```

**Key Benefits:**
- ✅ Tenant isolation enforced by LDAP credentials
- ✅ Connection pooling per tenant
- ✅ Efficient reuse for repeated tenant operations

---

## Connection Pool Monitoring

### Observability and Metrics

```go
package main

import (
    "fmt"
    "log"
    "time"

    ldap "github.com/netresearch/simple-ldap-go"
)

func monitorPool(pool *ldap.ConnectionPool) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        stats := pool.GetStats()

        log.Printf("Pool Statistics:")
        log.Printf("  Total Connections: %d", stats.TotalConnections)
        log.Printf("  Active Connections: %d", stats.ActiveConnections)
        log.Printf("  Available Connections: %d", stats.AvailableConnections)
        log.Printf("  Total Acquired: %d", stats.AcquiredCount)
        log.Printf("  Failed Acquisitions: %d", stats.FailedCount)
        log.Printf("  Max Connections: %d", stats.MaxConnections)

        // Calculate metrics
        utilizationRate := float64(stats.ActiveConnections) / float64(stats.MaxConnections) * 100
        failureRate := float64(stats.FailedCount) / float64(stats.AcquiredCount) * 100

        log.Printf("  Utilization: %.2f%%", utilizationRate)
        log.Printf("  Failure Rate: %.2f%%", failureRate)

        // Alert if utilization is high
        if utilizationRate > 80 {
            log.Printf("WARNING: High pool utilization!")
        }
    }
}

// Prometheus metrics integration example
func exportPrometheusMetrics(pool *ldap.ConnectionPool) {
    stats := pool.GetStats()

    // Export to Prometheus (pseudo-code)
    // prometheus.Set("ldap_pool_total_connections", stats.TotalConnections)
    // prometheus.Set("ldap_pool_active_connections", stats.ActiveConnections)
    // prometheus.Counter("ldap_pool_acquired_total", stats.AcquiredCount)
    // prometheus.Counter("ldap_pool_failed_total", stats.FailedCount)
}
```

---

## Error Handling Best Practices

### Graceful Degradation

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"

    ldap "github.com/netresearch/simple-ldap-go"
)

func performLDAPOperation(pool *ldap.ConnectionPool, userDN, password string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Acquire connection with timeout
    conn, err := pool.GetWithCredentials(userDN, password)
    if err != nil {
        // Handle specific errors
        if errors.Is(err, ldap.ErrPoolClosed) {
            return fmt.Errorf("service unavailable: pool is closed")
        }
        if errors.Is(err, ldap.ErrConnectionTimeout) {
            return fmt.Errorf("service busy: timeout acquiring connection")
        }
        if errors.Is(err, ldap.ErrInvalidCredentials) {
            return fmt.Errorf("authentication failed: invalid credentials")
        }
        return fmt.Errorf("ldap error: %w", err)
    }
    defer pool.Put(conn)

    // Perform operation with context
    user, err := conn.FindUserBySAMAccountNameContext(ctx, "jdoe")
    if err != nil {
        if ctx.Err() != nil {
            return fmt.Errorf("operation timeout: %w", ctx.Err())
        }
        return fmt.Errorf("search failed: %w", err)
    }

    log.Printf("Found user: %s", user.DisplayName)
    return nil
}

// Retry logic with exponential backoff
func performLDAPOperationWithRetry(pool *ldap.ConnectionPool, userDN, password string, maxRetries int) error {
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := performLDAPOperation(pool, userDN, password)
        if err == nil {
            return nil // Success
        }

        lastErr = err

        // Don't retry authentication failures
        if errors.Is(err, ldap.ErrInvalidCredentials) {
            return err
        }

        // Exponential backoff
        backoff := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
        log.Printf("Attempt %d failed: %v. Retrying in %v", attempt+1, err, backoff)
        time.Sleep(backoff)
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

---

## Summary

These examples demonstrate:

1. **Backward Compatibility**: Existing Get() continues to work
2. **Multi-User Support**: GetWithCredentials() enables per-user pooling
3. **Web Applications**: Efficient connection reuse per user in HTTP handlers
4. **Multi-Tenancy**: Tenant isolation through credential-aware pooling
5. **Monitoring**: Pool statistics for observability
6. **Error Handling**: Graceful degradation and retry logic

**Common Patterns:**

```go
// Pattern 1: Single-user (existing)
conn, err := pool.Get()
defer pool.Put(conn)

// Pattern 2: Multi-user (new)
conn, err := pool.GetWithCredentials(userDN, password)
defer pool.Put(conn)

// Pattern 3: Context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
conn, err := pool.GetWithCredentials(userDN, password)
defer pool.Put(conn)
```

All patterns ensure:
- ✅ Connections are returned to pool via defer
- ✅ Errors are handled appropriately
- ✅ Context timeouts are respected
- ✅ Credentials are protected and not mixed