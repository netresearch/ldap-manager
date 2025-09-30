# simple-ldap-go v1.3.0 Analysis & Upstream Contribution Opportunities

**Date:** 2025-09-30
**Repository:** https://github.com/netresearch/simple-ldap-go
**Current Version Used:** v1.3.0
**Analysis Scope:** New features, usage opportunities, upstream PR candidates

---

## Executive Summary

**✅ Status:** We updated to simple-ldap-go v1.3.0 but haven't leveraged its new batch lookup features yet.

**🎯 Opportunities:**

1. **Use new batch features** - Replace sequential lookups with batch operations
2. **Upstream our pool implementation** - simple-ldap-go has basic pooling, we have advanced features
3. **Contribute metrics** - Our pool stats could enhance their monitoring

---

## simple-ldap-go v1.3.0 New Features

### 1. Batch User Lookups (NEW in v1.3.0)

```go
// New batch lookup method - sequential but returns partial results
func (l *LDAP) FindUsersBySAMAccountNames(sAMAccountNames []string) ([]*User, error)
```

**Features:**

- Looks up multiple users by SAMAccountName
- Context-aware with cancellation support
- Returns partial results if some users not found
- Structured logging for batch operations
- **Sequential execution** (not parallel)

**Usage Example:**

```go
usernames := []string{"user1", "user2", "user3"}
users, err := ldapClient.FindUsersBySAMAccountNames(usernames)
// Returns found users, omits missing ones
```

### 2. Bulk Operations (STUB - Not Implemented)

```go
// BulkFindUsersBySAMAccountName - Currently returns stub data
func (l *LDAP) BulkFindUsersBySAMAccountName(ctx context.Context,
    samAccountNames []string, options *BulkSearchOptions) (map[string]*User, error)
```

**Status:** 🚧 Stub implementation only (lines 588-615 in client.go)

**Planned Features (commented):**

- Batch splitting based on `options.BatchSize`
- Concurrent execution up to `options.MaxConcurrency`
- Caching support via `options.UseCache`
- Error handling with `options.ContinueOnError`
- Retry logic with `options.RetryAttempts`

**Current Behavior:** Returns mock users for testing only

### 3. Connection Pooling

simple-ldap-go **already has** connection pooling in `pool.go`:

**Their Pool Features:**

- Basic connection pooling with min/max connections
- Health checking every 30s
- Connection lifecycle management
- Pool statistics (hits, misses, health checks)
- Thread-safe with RWMutex

**Their Pool Config:**

```go
type PoolConfig struct {
    MaxConnections      int           // default: 10
    MinConnections      int           // default: 2
    MaxIdleTime         time.Duration // default: 5min
    HealthCheckInterval time.Duration // default: 30s
    ConnectionTimeout   time.Duration // default: 30s
    GetTimeout          time.Duration // default: 10s
}
```

---

## Our Connection Pool vs simple-ldap-go Pool

### Feature Comparison

| Feature                    | simple-ldap-go Pool  | ldap-manager Pool            | Winner |
| -------------------------- | -------------------- | ---------------------------- | ------ |
| **Basic Pooling**          | ✅ Yes               | ✅ Yes                       | Tie    |
| **Health Checks**          | ✅ Every 30s         | ✅ Configurable              | Tie    |
| **Connection Reuse**       | ✅ Basic             | ✅ **Credential-aware**      | 🏆 Us  |
| **Lifecycle Management**   | ✅ MaxIdleTime       | ✅ MaxIdleTime + MaxLifetime | 🏆 Us  |
| **Statistics**             | ✅ Basic             | ✅ Comprehensive             | 🏆 Us  |
| **Credential Handling**    | ❌ Single user       | ✅ **Per-user pooling**      | 🏆 Us  |
| **Background Maintenance** | ⚠️ Basic             | ✅ Advanced cleanup          | 🏆 Us  |
| **Testing**                | ✅ Integration tests | ✅ Unit + integration        | Tie    |

### Key Differentiators in Our Implementation

#### 1. **Credential-Aware Connection Reuse**

**Our Innovation:**

```go
// We track credentials per connection and only reuse matching ones
type PooledConnection struct {
    client      *ldap.LDAP
    credentials *ConnectionCredentials // 🔑 Key feature
    createdAt   time.Time
    lastUsedAt  time.Time
    inUse       bool
    healthy     bool
}

func (p *ConnectionPool) canReuseConnection(conn *PooledConnection, creds *ConnectionCredentials) bool {
    // Match credentials before reuse
    if conn.credentials != nil && creds != nil {
        return conn.credentials.DN == creds.DN &&
               conn.credentials.Password == creds.Password
    }
    return conn.credentials == nil && creds == nil // readonly
}
```

**Why This Matters:**

- simple-ldap-go pools connections for a single service account
- We pool connections **per user** for multi-user scenarios
- Enables connection reuse in web apps with user-specific LDAP operations

#### 2. **Dual Lifecycle Management**

**Our Approach:**

```go
// Both MaxIdleTime AND MaxLifetime
type PoolConfig struct {
    MaxIdleTime  time.Duration // 15min - idle connection expiry
    MaxLifetime  time.Duration // 1hour - absolute connection lifetime
}
```

**Benefit:** Prevents long-lived connections from accumulating issues (memory leaks, stale state)

#### 3. **Advanced Statistics**

**Our Stats:**

```go
type PoolStats struct {
    TotalConnections     int32
    ActiveConnections    int32
    AvailableConnections int32
    AcquiredCount        int64 // Total acquisitions
    FailedCount          int64 // Failed acquisitions
    MaxConnections       int32
}
```

**Use Case:** Monitoring, alerting, capacity planning

---

## What We Didn't Use from v1.3.0

### 1. Batch User Lookups ❌

**Current ldap-manager Code:**

```go
// We do NOT use FindUsersBySAMAccountNames
// Instead, we use FindUserBySAMAccountName individually
user, err := ldapClient.FindUserBySAMAccountName("username")
```

**Opportunity:**
If we ever need to look up multiple users at once, we should use:

```go
users, err := ldapClient.FindUsersBySAMAccountNames([]string{"user1", "user2", "user3"})
```

**Impact:** Minor - we don't currently have batch lookup requirements

### 2. Bulk Operations API ❌

**Status:** Stub implementation in simple-ldap-go v1.3.0, not production-ready

**Our Assessment:** Wait for full implementation before adoption

---

## Upstream Contribution Opportunities

### 🏆 HIGH VALUE: Credential-Aware Connection Pooling

**What to Contribute:**

- Our `ConnectionCredentials` struct
- `canReuseConnection()` credential matching logic
- Per-user connection pooling design pattern

**Why simple-ldap-go Needs This:**

- Their pool assumes single service account
- Many applications need per-user LDAP operations
- Web apps with user authentication benefit significantly

**PR Scope:**

1. Add `ConnectionCredentials` tracking to `pooledConnection`
2. Implement credential matching in connection reuse logic
3. Add tests for multi-credential scenarios
4. Document use case: web apps with user-specific operations

**Estimated Effort:** 2-4 hours (clean, well-tested code already exists)

### 🥈 MEDIUM VALUE: Dual Lifecycle Management

**What to Contribute:**

- `MaxLifetime` in addition to `MaxIdleTime`
- Maintenance logic for both expiry types

**Why simple-ldap-go Needs This:**

- Long-lived connections can accumulate issues
- Industry best practice (used by database pools)

**PR Scope:**

1. Add `MaxLifetime` to `PoolConfig`
2. Update maintenance to check both expiry types
3. Add tests for lifetime-based expiry
4. Document best practices for values

**Estimated Effort:** 1-2 hours

### 🥉 LOW VALUE: Enhanced Pool Statistics

**What to Contribute:**

- Additional counters: `AcquiredCount`, `FailedCount`
- Monitoring-friendly stats structure

**Why simple-ldap-go Needs This:**

- Current stats focus on pool internals
- Missing operational metrics for monitoring
- No failure tracking

**PR Scope:**

1. Extend `PoolStats` with operational metrics
2. Add atomic counters for thread safety
3. Add example metrics integration (Prometheus)
4. Document monitoring use cases

**Estimated Effort:** 2-3 hours

---

## Recommended Action Plan

### Phase 1: Assessment (Now)

✅ **Done:** Analyzed simple-ldap-go v1.3.0 features
✅ **Done:** Compared our pool implementation
⏭️ **Next:** Discuss upstream contribution with team

### Phase 2: Quick Wins (Optional, Low Priority)

If we have batch lookup needs:

```go
// Replace sequential lookups with batch API
users, err := ldapClient.FindUsersBySAMAccountNames(usernames)
```

**Estimated Effort:** 1 hour if needed
**Impact:** Minimal (no current batch requirements)

### Phase 3: Upstream Contribution (Recommended)

**Priority 1: Credential-Aware Pooling**

- Extract credential matching logic
- Create comprehensive PR with tests
- Document multi-user use case
- **Timeline:** 1 week (includes PR review process)

**Priority 2: Dual Lifecycle Management**

- Extract MaxLifetime logic
- Add to existing pool implementation
- **Timeline:** 3-4 days

**Priority 3: Enhanced Statistics**

- Extract monitoring metrics
- Add Prometheus integration example
- **Timeline:** 3-4 days

---

## Benefits of Upstreaming

### For simple-ldap-go Community

1. **Multi-User Support:** Web apps can use per-user connection pooling
2. **Better Reliability:** Dual lifecycle management prevents stale connections
3. **Improved Observability:** Monitoring-friendly statistics

### For ldap-manager

1. **Reduced Maintenance:** Upstream handles pool implementation
2. **Community Testing:** Broader testing coverage
3. **Standardization:** Industry-standard pool implementation
4. **Potential Removal:** Could eventually replace our custom pool with upstream

---

## Technical Requirements for Upstream PR

### Code Quality

- ✅ Well-tested (we have 59.8% coverage on pool.go)
- ✅ Well-documented (comprehensive inline docs)
- ✅ Thread-safe (using sync primitives correctly)
- ✅ Modern Go (using Go 1.25 features like WaitGroup.Go)

### PR Checklist

- [ ] Extract code into standalone branch
- [ ] Add comprehensive tests
- [ ] Add benchmark comparisons
- [ ] Write detailed PR description with use cases
- [ ] Include example code
- [ ] Update README with new features
- [ ] Add CHANGELOG entry

---

## Conclusion

**Should we use v1.3.0 batch features?**
🟡 **Optional** - Only if we develop batch lookup requirements

**Should we contribute our pool upstream?**
🟢 **Yes, Recommended** - Our credential-aware pooling solves real multi-user scenarios

**Priority:**
Medium - Not urgent, but valuable for both projects

**Next Steps:**

1. Discuss with team: upstream contribution strategy
2. If approved: Create upstream PR for credential-aware pooling
3. Monitor simple-ldap-go v1.4.0 for bulk operations completion
4. Consider replacing our pool with upstream after contribution accepted

---

**Author:** Claude Code
**Review:** Sebastian
**Status:** Ready for team discussion
