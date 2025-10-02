# Connection Pool Bug Report - simple-ldap-go v1.5.1

## Summary
Critical bug in simple-ldap-go v1.5.1: Find methods (FindUsers, FindGroups, FindComputers) close connections instead of returning them to the pool, causing pool exhaustion and 30-second timeouts.

## Root Cause
All Find methods use this pattern:

```go
c, err := l.GetConnectionContext(ctx)
if err != nil {
    return nil, err
}
defer func() {
    if closeErr := c.Close(); closeErr != nil {  // ❌ BUG: Closes connection
        l.logger.Debug("connection_close_error",
            slog.String("operation", "FindComputers"),
            slog.String("error", closeErr.Error()))
    }
}()
```

**Problem:** `c.Close()` destroys the connection instead of returning it to pool via `pool.Put(conn)`

## Impact
- **Initial warmup**: Works because pool.warmPool() properly calls `Put()` (pool.go:633)
- **Refresh cycles**:
  - First operation: Gets connection, uses it, **closes it** (not returned to pool)
  - Subsequent operations: Wait 30s for timeout (`LDAP_POOL_ACQUIRE_TIMEOUT`)
  - Result: Pool gets exhausted, all operations fail with "connection pool exhausted"

## Evidence from Logs

### Initial Warmup (✅ Works)
```
2025/10/02 14:04:35 INFO computer_list_search_completed duration=8.475127ms
2025/10/02 14:04:35 INFO group_list_search_completed duration=10.699983ms
2025/10/02 14:04:35 INFO user_list_search_completed duration=19.037089ms
```

### First Refresh Cycle (❌ Breaks)
```
2025/10/02 14:05:05 INFO user_list_search_completed duration=19.394004ms    ← Fast (got connection)
2025/10/02 14:05:35 INFO computer_list_search_completed duration=30.030s  ← Timeout
2025/10/02 14:05:35 INFO group_list_search_completed duration=30.031s     ← Timeout
```

### Eventually
```
2025/10/02 13:31:02 ERROR pool_connection_failed error="connection pool exhausted" duration=30s
```

## Expected Fix
All Find methods should return connections to pool:

```go
c, err := l.GetConnectionContext(ctx)
if err != nil {
    return nil, err
}
defer func() {
    if l.connPool != nil {
        if putErr := l.connPool.Put(c); putErr != nil {  // ✅ Return to pool
            l.logger.Debug("connection_return_error",
                slog.String("operation", "FindComputers"),
                slog.String("error", putErr.Error()))
        }
    } else {
        // No pool, close directly
        if closeErr := c.Close(); closeErr != nil {
            l.logger.Debug("connection_close_error",
                slog.String("operation", "FindComputers"),
                slog.String("error", closeErr.Error()))
        }
    }
}()
```

Or add a helper method:

```go
func (l *LDAP) ReleaseConnection(conn *ldap.Conn) error {
    if l.connPool != nil {
        return l.connPool.Put(conn)
    }
    return conn.Close()
}
```

Then use:
```go
defer l.ReleaseConnection(c)
```

## Affected Methods
- `FindUsers()` / `FindUsersContext()`
- `FindGroups()` / `FindGroupsContext()`
- `FindComputers()` / `FindComputersContext()`
- `FindUserBySAMAccountName()`
- Likely others that use `GetConnectionContext()`

## Workarounds Attempted
1. **Parallel execution**: Running Find operations in parallel helps but doesn't solve the core issue
2. **Increased pool size**: Delays exhaustion but doesn't prevent it
3. **Direct connections**: Would work but defeats purpose of connection pooling

## Versions
- simple-ldap-go: v1.5.1
- Issue introduced: v1.5.0 (when connection pooling was added)
- Pool warmup fix (PR #46): Present in v1.5.1 but doesn't help with Find methods

## Repository
- Upstream: https://github.com/netresearch/simple-ldap-go
- Affected version: v1.5.1
- Needs fix in: computers.go, users.go, groups.go (and similar files)
