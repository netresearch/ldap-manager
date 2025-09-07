package ldap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
