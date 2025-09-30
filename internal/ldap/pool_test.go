package ldap

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	assert.Equal(t, 10, config.MaxConnections)
	assert.Equal(t, 2, config.MinConnections)
	assert.Equal(t, 15*time.Minute, config.MaxIdleTime)
	assert.Equal(t, 1*time.Hour, config.MaxLifetime)
	assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
	assert.Equal(t, 10*time.Second, config.AcquireTimeout)
}

func TestPoolConfigValidation(t *testing.T) {
	// Test config with zero MaxConnections
	config := &PoolConfig{
		MaxConnections: 0,
		MinConnections: 1,
	}

	pool, err := NewConnectionPool(nil, config)
	if err == nil && pool != nil {
		// Should default to 10
		assert.Equal(t, 10, pool.config.MaxConnections)
		if err := pool.Close(); err != nil {
			t.Logf("Pool close error: %v", err)
		}
	}

	// Test config with MinConnections > MaxConnections
	config2 := &PoolConfig{
		MaxConnections: 5,
		MinConnections: 10,
	}

	pool2, err2 := NewConnectionPool(nil, config2)
	if err2 == nil && pool2 != nil {
		// MinConnections should be adjusted to MaxConnections
		assert.Equal(t, 5, pool2.config.MinConnections)
		assert.Equal(t, 5, pool2.config.MaxConnections)
		if err := pool2.Close(); err != nil {
			t.Logf("Pool close error: %v", err)
		}
	}
}

func TestPoolStatsStructure(t *testing.T) {
	stats := PoolStats{
		TotalConnections:     5,
		ActiveConnections:    3,
		AvailableConnections: 2,
		AcquiredCount:        100,
		FailedCount:          2,
		MaxConnections:       10,
	}

	assert.Equal(t, int32(5), stats.TotalConnections)
	assert.Equal(t, int32(3), stats.ActiveConnections)
	assert.Equal(t, int32(2), stats.AvailableConnections)
	assert.Equal(t, int64(100), stats.AcquiredCount)
	assert.Equal(t, int64(2), stats.FailedCount)
	assert.Equal(t, int32(10), stats.MaxConnections)
}

func TestConnectionCredentials(t *testing.T) {
	creds := &ConnectionCredentials{
		DN:       "cn=test,dc=example,dc=com",
		Password: "testpass",
	}

	assert.Equal(t, "cn=test,dc=example,dc=com", creds.DN)
	assert.Equal(t, "testpass", creds.Password)
}

func TestPooledConnectionBasicOps(t *testing.T) {
	// Test basic pooled connection structure
	conn := &PooledConnection{
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
		healthy:    true,
		inUse:      false,
	}

	assert.True(t, conn.healthy)
	assert.False(t, conn.inUse)
	assert.WithinDuration(t, time.Now(), conn.createdAt, time.Second)
}

// TestPoolGetStatsZeroConnections tests GetStats with a fresh pool (zero connections scenario)
func TestPoolGetStatsZeroConnections(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0, // No warmup connections
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	stats := pool.GetStats()

	assert.Equal(t, int32(0), stats.TotalConnections)
	assert.Equal(t, int32(0), stats.ActiveConnections)
	assert.Equal(t, int32(0), stats.AvailableConnections)
	assert.Equal(t, int64(0), stats.AcquiredCount)
	assert.Equal(t, int64(0), stats.FailedCount)
	assert.Equal(t, int32(5), stats.MaxConnections)
}

// TestPoolGetStatsAfterOperations tests GetStats after simulating pool operations
func TestPoolGetStatsAfterOperations(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      10,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	// Simulate some pool operations by directly manipulating internal counters
	atomic.StoreInt32(&pool.totalConnections, 5)
	atomic.StoreInt32(&pool.activeConnections, 3)
	atomic.StoreInt64(&pool.acquiredConnections, 100)
	atomic.StoreInt64(&pool.failedConnections, 2)

	stats := pool.GetStats()

	assert.Equal(t, int32(5), stats.TotalConnections)
	assert.Equal(t, int32(3), stats.ActiveConnections)
	assert.Equal(t, int64(100), stats.AcquiredCount)
	assert.Equal(t, int64(2), stats.FailedCount)
	assert.Equal(t, int32(10), stats.MaxConnections)
}

// TestPoolCloseIdempotency tests that closing a pool multiple times is safe
func TestPoolCloseIdempotency(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)

	// Close the pool
	err = pool.Close()
	assert.NoError(t, err)

	// Close again - should be safe
	err = pool.Close()
	assert.NoError(t, err)

	// Verify pool is marked as closed
	assert.Equal(t, int32(1), atomic.LoadInt32(&pool.closed))
}

// TestPoolCloseWithConnections tests pool closure with existing connections
func TestPoolCloseWithConnections(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)

	// Add some connections to the pool manually
	conn1 := &PooledConnection{
		client:      nil,
		credentials: nil,
		createdAt:   time.Now(),
		lastUsedAt:  time.Now(),
		healthy:     true,
		inUse:       false,
	}

	pool.mutex.Lock()
	pool.connections = append(pool.connections, conn1)
	atomic.StoreInt32(&pool.totalConnections, 1)
	pool.mutex.Unlock()

	// Close the pool
	err = pool.Close()
	assert.NoError(t, err)

	// Verify connections are cleaned up
	pool.mutex.RLock()
	assert.Nil(t, pool.connections)
	assert.False(t, conn1.healthy)
	pool.mutex.RUnlock()
}

// TestPoolAcquireAfterClose tests that acquiring connections after close fails
func TestPoolAcquireAfterClose(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)

	// Close the pool
	err = pool.Close()
	assert.NoError(t, err)

	// Try to acquire a connection - should fail
	ctx := context.Background()
	conn, err := pool.AcquireConnection(ctx, "cn=test", "password")

	assert.Error(t, err)
	assert.Equal(t, ErrPoolClosed, err)
	assert.Nil(t, conn)
}

// TestPooledConnectionGetClient tests GetClient method
func TestPooledConnectionGetClient(t *testing.T) {
	conn := &PooledConnection{
		client:      nil, // No real client needed for test
		createdAt:   time.Now(),
		lastUsedAt:  time.Now(),
		healthy:     true,
		inUse:       false,
		credentials: nil,
	}

	client := conn.GetClient()
	assert.Nil(t, client) // Expected since we set it to nil
}

// TestSafeIntToInt32 tests the safe conversion function with various inputs
func TestSafeIntToInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int32
	}{
		{
			name:     "normal positive value",
			input:    100,
			expected: 100,
		},
		{
			name:     "normal negative value",
			input:    -100,
			expected: -100,
		},
		{
			name:     "zero value",
			input:    0,
			expected: 0,
		},
		{
			name:     "max int32 value",
			input:    2147483647,
			expected: 2147483647,
		},
		{
			name:     "min int32 value",
			input:    -2147483648,
			expected: -2147483648,
		},
		{
			name:     "overflow - above max int32",
			input:    2147483648,
			expected: 2147483647,
		},
		{
			name:     "overflow - below min int32",
			input:    -2147483649,
			expected: -2147483648,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeIntToInt32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsConnectionValid tests connection validation logic
func TestIsConnectionValid(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	t.Run("valid connection", func(t *testing.T) {
		conn := &PooledConnection{
			createdAt:  time.Now(),
			lastUsedAt: time.Now(),
			healthy:    true,
			inUse:      false,
		}

		valid := pool.isConnectionValid(conn)
		assert.True(t, valid)
	})

	t.Run("unhealthy connection", func(t *testing.T) {
		conn := &PooledConnection{
			createdAt:  time.Now(),
			lastUsedAt: time.Now(),
			healthy:    false,
			inUse:      false,
		}

		valid := pool.isConnectionValid(conn)
		assert.False(t, valid)
	})

	t.Run("expired connection - max lifetime exceeded", func(t *testing.T) {
		conn := &PooledConnection{
			createdAt:  time.Now().Add(-2 * time.Hour), // Created 2 hours ago
			lastUsedAt: time.Now(),
			healthy:    true,
			inUse:      false,
		}

		valid := pool.isConnectionValid(conn)
		assert.False(t, valid)
	})

	t.Run("connection at lifetime boundary", func(t *testing.T) {
		conn := &PooledConnection{
			createdAt:  time.Now().Add(-59 * time.Minute), // Just under 1 hour
			lastUsedAt: time.Now(),
			healthy:    true,
			inUse:      false,
		}

		valid := pool.isConnectionValid(conn)
		assert.True(t, valid)
	})
}

// TestCanReuseConnection tests connection reusability logic
func TestCanReuseConnection(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	t.Run("can reuse - matching credentials", func(t *testing.T) {
		creds := &ConnectionCredentials{
			DN:       "cn=test,dc=example,dc=com",
			Password: "password",
		}

		conn := &PooledConnection{
			credentials: creds,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       false,
		}

		canReuse := pool.canReuseConnection(conn, creds)
		assert.True(t, canReuse)
	})

	t.Run("cannot reuse - different credentials", func(t *testing.T) {
		connCreds := &ConnectionCredentials{
			DN:       "cn=user1,dc=example,dc=com",
			Password: "password1",
		}

		newCreds := &ConnectionCredentials{
			DN:       "cn=user2,dc=example,dc=com",
			Password: "password2",
		}

		conn := &PooledConnection{
			credentials: connCreds,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       false,
		}

		canReuse := pool.canReuseConnection(conn, newCreds)
		assert.False(t, canReuse)
	})

	t.Run("cannot reuse - unhealthy connection", func(t *testing.T) {
		creds := &ConnectionCredentials{
			DN:       "cn=test,dc=example,dc=com",
			Password: "password",
		}

		conn := &PooledConnection{
			credentials: creds,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     false,
			inUse:       false,
		}

		canReuse := pool.canReuseConnection(conn, creds)
		assert.False(t, canReuse)
	})

	t.Run("cannot reuse - max lifetime exceeded", func(t *testing.T) {
		creds := &ConnectionCredentials{
			DN:       "cn=test,dc=example,dc=com",
			Password: "password",
		}

		conn := &PooledConnection{
			credentials: creds,
			createdAt:   time.Now().Add(-2 * time.Hour),
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       false,
		}

		canReuse := pool.canReuseConnection(conn, creds)
		assert.False(t, canReuse)
	})

	t.Run("cannot reuse - max idle time exceeded", func(t *testing.T) {
		creds := &ConnectionCredentials{
			DN:       "cn=test,dc=example,dc=com",
			Password: "password",
		}

		conn := &PooledConnection{
			credentials: creds,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now().Add(-20 * time.Minute), // Idle for 20 minutes
			healthy:     true,
			inUse:       false,
		}

		canReuse := pool.canReuseConnection(conn, creds)
		assert.False(t, canReuse)
	})

	t.Run("can reuse - both nil credentials (readonly)", func(t *testing.T) {
		conn := &PooledConnection{
			credentials: nil,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       false,
		}

		canReuse := pool.canReuseConnection(conn, nil)
		assert.True(t, canReuse)
	})
}

// TestReleaseConnection tests connection release logic
func TestReleaseConnection(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	t.Run("release nil connection - should be safe", func(_ *testing.T) {
		pool.ReleaseConnection(nil)
		// Should not panic
	})

	t.Run("release valid connection", func(t *testing.T) {
		conn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       true,
		}

		// Set active connections counter
		atomic.StoreInt32(&pool.activeConnections, 1)
		atomic.StoreInt32(&pool.totalConnections, 1)

		pool.ReleaseConnection(conn)

		// Verify connection is no longer in use
		conn.mutex.RLock()
		assert.False(t, conn.inUse)
		conn.mutex.RUnlock()

		// Verify active connections decremented
		assert.Equal(t, int32(0), atomic.LoadInt32(&pool.activeConnections))
	})

	t.Run("release expired connection - should be closed", func(t *testing.T) {
		conn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now().Add(-2 * time.Hour), // Expired
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       true,
		}

		atomic.StoreInt32(&pool.activeConnections, 1)
		atomic.StoreInt32(&pool.totalConnections, 1)

		pool.ReleaseConnection(conn)

		// Connection should be marked unhealthy
		conn.mutex.RLock()
		assert.False(t, conn.healthy)
		conn.mutex.RUnlock()

		// Total connections should be decremented
		assert.Equal(t, int32(0), atomic.LoadInt32(&pool.totalConnections))
	})
}

// TestCloseConnection tests connection cleanup
func TestCloseConnection(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	conn := &PooledConnection{
		client:      nil, // Simulate a client
		credentials: nil,
		createdAt:   time.Now(),
		lastUsedAt:  time.Now(),
		healthy:     true,
		inUse:       false,
	}

	pool.closeConnection(conn)

	// Verify connection is marked unhealthy
	conn.mutex.RLock()
	assert.False(t, conn.healthy)
	assert.Nil(t, conn.client)
	conn.mutex.RUnlock()
}

// TestPerformMaintenance tests the maintenance cleanup logic
func TestPerformMaintenance(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      5,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	t.Run("cleanup expired connections", func(t *testing.T) {
		// Add expired and valid connections
		expiredConn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now().Add(-2 * time.Hour), // Expired
			lastUsedAt:  time.Now().Add(-2 * time.Hour),
			healthy:     true,
			inUse:       false,
		}

		validConn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     true,
			inUse:       false,
		}

		pool.mutex.Lock()
		pool.connections = []*PooledConnection{expiredConn, validConn}
		atomic.StoreInt32(&pool.totalConnections, 2)
		pool.mutex.Unlock()

		// Run maintenance
		pool.performMaintenance()

		// Verify only valid connection remains
		pool.mutex.RLock()
		assert.Equal(t, 1, len(pool.connections))
		assert.Equal(t, validConn, pool.connections[0])
		pool.mutex.RUnlock()

		assert.Equal(t, int32(1), atomic.LoadInt32(&pool.totalConnections))
	})

	t.Run("cleanup idle connections", func(t *testing.T) {
		idleConn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now().Add(-20 * time.Minute), // Idle > 15 minutes
			healthy:     true,
			inUse:       false,
		}

		pool.mutex.Lock()
		pool.connections = []*PooledConnection{idleConn}
		atomic.StoreInt32(&pool.totalConnections, 1)
		pool.mutex.Unlock()

		pool.performMaintenance()

		// Idle connection should be removed
		pool.mutex.RLock()
		assert.Equal(t, 0, len(pool.connections))
		pool.mutex.RUnlock()

		assert.Equal(t, int32(0), atomic.LoadInt32(&pool.totalConnections))
	})

	t.Run("keep in-use connections even if idle", func(t *testing.T) {
		inUseConn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now().Add(-20 * time.Minute), // Idle but in use
			healthy:     true,
			inUse:       true, // In use
		}

		pool.mutex.Lock()
		pool.connections = []*PooledConnection{inUseConn}
		atomic.StoreInt32(&pool.totalConnections, 1)
		pool.mutex.Unlock()

		pool.performMaintenance()

		// In-use connection should be kept
		pool.mutex.RLock()
		assert.Equal(t, 1, len(pool.connections))
		pool.mutex.RUnlock()

		assert.Equal(t, int32(1), atomic.LoadInt32(&pool.totalConnections))
	})

	t.Run("cleanup unhealthy connections", func(t *testing.T) {
		unhealthyConn := &PooledConnection{
			client:      nil,
			credentials: nil,
			createdAt:   time.Now(),
			lastUsedAt:  time.Now(),
			healthy:     false, // Unhealthy
			inUse:       false,
		}

		pool.mutex.Lock()
		pool.connections = []*PooledConnection{unhealthyConn}
		atomic.StoreInt32(&pool.totalConnections, 1)
		pool.mutex.Unlock()

		pool.performMaintenance()

		// Unhealthy connection should be removed
		pool.mutex.RLock()
		assert.Equal(t, 0, len(pool.connections))
		pool.mutex.RUnlock()

		assert.Equal(t, int32(0), atomic.LoadInt32(&pool.totalConnections))
	})
}

// TestConcurrentPoolAccess tests thread-safety of pool operations
func TestConcurrentPoolAccess(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      10,
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	t.Run("concurrent GetStats calls", func(t *testing.T) {
		var wg sync.WaitGroup
		goroutines := 50

		for range goroutines {
			wg.Go(func() {
				stats := pool.GetStats()
				assert.NotNil(t, stats)
			})
		}

		wg.Wait()
	})

	t.Run("concurrent counter updates", func(t *testing.T) {
		var wg sync.WaitGroup
		goroutines := 100

		for range goroutines {
			wg.Go(func() {
				atomic.AddInt32(&pool.totalConnections, 1)
				atomic.AddInt32(&pool.activeConnections, 1)
				atomic.AddInt64(&pool.acquiredConnections, 1)
			})
		}

		wg.Wait()

		// Verify all increments were applied
		assert.Equal(t, int32(goroutines), atomic.LoadInt32(&pool.totalConnections))
		assert.Equal(t, int32(goroutines), atomic.LoadInt32(&pool.activeConnections))
		assert.Equal(t, int64(goroutines), atomic.LoadInt64(&pool.acquiredConnections))
	})
}

// TestPoolConfigBoundaryConditions tests edge cases in pool configuration
func TestPoolConfigBoundaryConditions(t *testing.T) {
	tests := []struct {
		name           string
		config         *PoolConfig
		expectedMaxMin func(*PoolConfig) bool
	}{
		{
			name: "negative MinConnections should be adjusted",
			config: &PoolConfig{
				MaxConnections: 10,
				MinConnections: -5,
			},
			expectedMaxMin: func(cfg *PoolConfig) bool {
				return cfg.MinConnections >= 0
			},
		},
		{
			name: "zero MaxIdleTime should get default",
			config: &PoolConfig{
				MaxConnections: 10,
				MinConnections: 2,
				MaxIdleTime:    0,
			},
			expectedMaxMin: func(cfg *PoolConfig) bool {
				return cfg.MaxIdleTime > 0
			},
		},
		{
			name: "zero MaxLifetime should get default",
			config: &PoolConfig{
				MaxConnections: 10,
				MinConnections: 2,
				MaxLifetime:    0,
			},
			expectedMaxMin: func(cfg *PoolConfig) bool {
				return cfg.MaxLifetime > 0
			},
		},
		{
			name: "zero HealthCheckInterval should get default",
			config: &PoolConfig{
				MaxConnections:      10,
				MinConnections:      2,
				HealthCheckInterval: 0,
			},
			expectedMaxMin: func(cfg *PoolConfig) bool {
				return cfg.HealthCheckInterval > 0
			},
		},
		{
			name: "zero AcquireTimeout should get default",
			config: &PoolConfig{
				MaxConnections: 10,
				MinConnections: 2,
				AcquireTimeout: 0,
			},
			expectedMaxMin: func(cfg *PoolConfig) bool {
				return cfg.AcquireTimeout > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewConnectionPool(nil, tt.config)
			if err == nil && pool != nil {
				defer func() { _ = pool.Close() }()
				assert.True(t, tt.expectedMaxMin(pool.config))
			}
		})
	}
}

// TestAcquireConnectionTimeout tests timeout behavior
func TestAcquireConnectionTimeout(t *testing.T) {
	config := &PoolConfig{
		MaxConnections:      1, // Very small pool
		MinConnections:      0,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      100 * time.Millisecond, // Very short timeout
	}

	pool, err := NewConnectionPool(nil, config)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	// Fill the pool with a mock connection
	conn := &PooledConnection{
		client:      nil,
		credentials: nil,
		createdAt:   time.Now(),
		lastUsedAt:  time.Now(),
		healthy:     true,
		inUse:       true,
	}

	pool.mutex.Lock()
	pool.connections = append(pool.connections, conn)
	atomic.StoreInt32(&pool.totalConnections, 1)
	pool.mutex.Unlock()

	// Try to acquire with a context that will timeout
	ctx := context.Background()
	acquiredConn, err := pool.AcquireConnection(ctx, "cn=test", "password")

	// Should timeout since pool is at capacity and no connections available
	assert.Error(t, err)
	assert.Nil(t, acquiredConn)
}

// TestConfigNilDefault tests that nil config uses defaults
func TestConfigNilDefault(t *testing.T) {
	pool, err := NewConnectionPool(nil, nil)
	require.NoError(t, err)
	defer func() { _ = pool.Close() }()

	// Verify default config was applied
	assert.Equal(t, 10, pool.config.MaxConnections)
	assert.Equal(t, 2, pool.config.MinConnections)
	assert.Equal(t, 15*time.Minute, pool.config.MaxIdleTime)
	assert.Equal(t, 1*time.Hour, pool.config.MaxLifetime)
	assert.Equal(t, 30*time.Second, pool.config.HealthCheckInterval)
	assert.Equal(t, 10*time.Second, pool.config.AcquireTimeout)
}
