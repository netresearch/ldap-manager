# Upstream PR Merged: simple-ldap-go v1.4.0 Migration Status

**Date:** 2025-09-30
**PR:** https://github.com/netresearch/simple-ldap-go/pull/44
**Status:** ✅ Merged and released as v1.4.0

---

## Summary

Our credential-aware connection pooling enhancement has been successfully merged upstream into simple-ldap-go and released as v1.4.0! This means the custom pool implementation in ldap-manager can now be replaced with the upstream version.

---

## What Was Accomplished

### 1. Upstream PR Success ✅

**PR #44** successfully merged with:
- **ConnectionCredentials** struct for per-connection credential tracking
- **GetWithCredentials(ctx, dn, password)** method for multi-user pooling
- **canReuseConnection()** credential matching logic
- Comprehensive tests validating functionality
- Performance benchmarks showing <5% overhead

### 2. Migration Started (Partial)

**Completed:**
- ✅ Updated go.mod to simple-ldap-go v1.4.0
- ✅ Deleted `internal/ldap/pool.go` (now upstream)
- ✅ Deleted `internal/ldap/pool_test.go` (now upstream)
- ✅ Started updating `internal/ldap/manager.go` to use upstream pool

**In Progress:**
- ⏳ Complete manager.go integration with upstream API
- ⏳ Fix NewPoolManager signature to match upstream requirements
- ⏳ Update server.go to pass correct parameters

---

## API Differences

### Our Custom Implementation (OLD)

```go
// internal/ldap/pool.go - DELETED
type ConnectionPool struct {
    baseClient  *ldap.LDAP
    // ...
}

func NewConnectionPool(baseClient *ldap.LDAP, config *PoolConfig) (*ConnectionPool, error)

func (p *ConnectionPool) AcquireConnection(ctx context.Context, dn, password string) (*PooledConnection, error)
func (p *ConnectionPool) ReleaseConnection(conn *PooledConnection)
```

### Upstream simple-ldap-go v1.4.0 (NEW)

```go
// github.com/netresearch/simple-ldap-go
type ConnectionPool struct {
    config     *PoolConfig
    ldapConfig Config
    // ...
}

func NewConnectionPool(poolConfig *PoolConfig, ldapConfig Config, user, password string, logger *slog.Logger) (*ConnectionPool, error)

func (p *ConnectionPool) GetWithCredentials(ctx context.Context, dn, password string) (*ldap.Conn, error)
func (p *ConnectionPool) Get(ctx context.Context) (*ldap.Conn, error)
func (p *ConnectionPool) Put(conn *ldap.Conn) error
```

### Key Differences

| Aspect | Old (Custom) | New (Upstream) |
|--------|--------------|----------------|
| **Constructor params** | `(baseClient, config)` | `(poolConfig, ldapConfig, user, password, logger)` |
| **Get with creds** | `AcquireConnection(ctx, dn, pwd)` | `GetWithCredentials(ctx, dn, pwd)` |
| **Return to pool** | `ReleaseConnection(conn)` | `Put(conn) error` |
| **Return type** | `*PooledConnection` (our wrapper) | `*ldap.Conn` (go-ldap raw conn) |
| **Stats** | `PoolStats` (our struct) | `PoolStats` (upstream struct with more fields) |

---

## Migration Tasks Remaining

### 1. Fix NewPoolManager Signature

**Current (broken):**
```go
func NewPoolManager(baseClient *ldap.LDAP, config *PoolConfig) (*PoolManager, error)
```

**Needs to be:**
```go
func NewPoolManager(ldapConfig ldap.Config, user, password string) (*PoolManager, error)
```

**Why:** Upstream pool needs `ldap.Config` + credentials, not a pre-created client

### 2. Update server.go Call Site

**File:** `internal/web/server.go:96`

**Current (broken):**
```go
ldapPool, err := ldappool.NewPoolManager(ldapClient, createPoolConfig(opts))
```

**Needs to be:**
```go
ldapPool, err := ldappool.NewPoolManager(opts.LDAP, opts.ReadonlyUser, opts.ReadonlyPassword)
```

### 3. Handle Connection Wrapping

**Challenge:** Upstream pool returns `*ldap.Conn` (raw go-ldap connection), but we need `*ldap.LDAP` (simple-ldap-go client wrapper).

**Options:**
1. **Simple-ldap-go might provide wrapper method** - check if they have `ldap.WrapConnection(conn)` or similar
2. **Use pool at higher level** - Instead of wrapping in PoolManager, use pool directly in handlers
3. **Re-authenticate** - Create new LDAP client from returned connection (inefficient)

### 4. Update PooledLDAPClient

**Current issue:** PooledLDAPClient expects `*ldap.LDAP` but pool returns `*ldap.Conn`

**Solution TBD** - Depends on how connection wrapping is handled

### 5. Stats Struct Updates

Update `GetHealthStatus()` to use upstream `PoolStats` fields:
- `PoolHits` / `PoolMisses` (new)
- `HealthChecksPassed` / `HealthChecksFailed` (new)
- `ConnectionsCreated` / `ConnectionsClosed` (new)
- Remove: `AcquiredCount`, `FailedCount`, `AvailableConnections`

---

## Testing Strategy

Once migration is complete:

1. **Unit Tests**
   - Test WithCredentials flow
   - Test GetReadOnlyClient flow
   - Test connection return (Put)
   - Test stats reporting

2. **Integration Tests**
   - Run full test suite
   - Verify credential isolation
   - Verify connection reuse
   - Check for connection leaks

3. **Manual Testing**
   - Start application
   - Login with different users
   - Verify operations work
   - Check pool stats endpoint

---

## Benefits After Migration

✅ **Less Code to Maintain** - No custom pool implementation
✅ **Community Tested** - Upstream pool is tested by multiple users
✅ **Bug Fixes** - Upstream improvements benefit us automatically
✅ **Consistency** - Using standard library approach
✅ **Performance** - Upstream pool is optimized and benchmarked

---

## Next Steps for Sebastian

### Option 1: Complete Migration Now

Continue the migration:
1. Fix NewPoolManager signature
2. Update server.go call site
3. Resolve connection wrapping issue
4. Run tests and fix any issues
5. Commit migration

**Estimated time:** 2-3 hours

### Option 2: Revert and Migrate Later

Revert the partial migration:
```bash
git restore internal/ldap/pool.go internal/ldap/pool_test.go internal/ldap/manager.go
```

Keep using custom pool for now, migrate when more time available.

### Option 3: Hybrid Approach

Keep the current partial state:
- Custom pool deleted
- manager.go updated to use upstream types
- Fix remaining issues incrementally
- Deploy with upstream pool when ready

---

## Files Modified

```
modified:   go.mod                      # Updated to v1.4.0
modified:   go.sum                      # Updated checksums
modified:   internal/ldap/manager.go    # Partially migrated to upstream
deleted:    internal/ldap/pool.go       # Now upstream
deleted:    internal/ldap/pool_test.go  # Now upstream
```

---

## Additional Resources

- **Upstream PR:** https://github.com/netresearch/simple-ldap-go/pull/44
- **PR Materials:** `/srv/www/sme/ldap-manager/claudedocs/upstream-pr-simple-ldap-go/`
- **Analysis Document:** `/srv/www/sme/ldap-manager/claudedocs/simple-ldap-go-analysis-and-upstream-opportunities.md`

---

**Status:** Migration paused - requires connection wrapping strategy decision