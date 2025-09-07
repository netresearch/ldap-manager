// Package ldap provides connection pool management for LDAP operations
package ldap

import (
	"context"
	"fmt"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

// PoolManager provides a high-level interface for LDAP connection pool operations
// It wraps the connection pool and provides convenient methods for common LDAP tasks
type PoolManager struct {
	pool       *ConnectionPool
	baseClient *ldap.LDAP
}

// NewPoolManager creates a new pool manager with the specified base client and configuration
func NewPoolManager(baseClient *ldap.LDAP, config *PoolConfig) (*PoolManager, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool, err := NewConnectionPool(baseClient, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	manager := &PoolManager{
		pool:       pool,
		baseClient: baseClient,
	}

	log.Info().Msg("LDAP pool manager initialized")

	return manager, nil
}

// WithCredentials gets an authenticated LDAP client from the connection pool
// This replaces the simple-ldap-go WithCredentials method with pooled connections
func (pm *PoolManager) WithCredentials(ctx context.Context, dn, password string) (*PooledLDAPClient, error) {
	conn, err := pm.pool.AcquireConnection(ctx, dn, password)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	return &PooledLDAPClient{
		client: conn.GetClient(),
		conn:   conn,
		pool:   pm.pool,
	}, nil
}

// GetReadOnlyClient gets a read-only LDAP client from the connection pool
// This is useful for operations that don't require specific user credentials
func (pm *PoolManager) GetReadOnlyClient(ctx context.Context) (*PooledLDAPClient, error) {
	conn, err := pm.pool.AcquireConnection(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to acquire readonly connection: %w", err)
	}

	return &PooledLDAPClient{
		client: conn.GetClient(),
		conn:   conn,
		pool:   pm.pool,
	}, nil
}

// GetStats returns connection pool statistics
func (pm *PoolManager) GetStats() PoolStats {
	return pm.pool.GetStats()
}

// GetHealthStatus returns the health status of the connection pool
func (pm *PoolManager) GetHealthStatus() map[string]interface{} {
	stats := pm.pool.GetStats()

	// Determine health based on available connections and error rates
	healthy := true
	if stats.TotalConnections == 0 {
		healthy = false
	}

	// Consider unhealthy if high error rate (>10% failures)
	if stats.AcquiredCount > 0 {
		errorRate := float64(stats.FailedCount) / float64(stats.AcquiredCount)
		if errorRate > 0.1 {
			healthy = false
		}
	}

	return map[string]interface{}{
		"healthy":               healthy,
		"total_connections":     stats.TotalConnections,
		"active_connections":    stats.ActiveConnections,
		"available_connections": stats.AvailableConnections,
		"acquired_count":        stats.AcquiredCount,
		"failed_count":          stats.FailedCount,
		"max_connections":       stats.MaxConnections,
	}
}

// Close gracefully shuts down the pool manager
func (pm *PoolManager) Close() error {
	log.Info().Msg("Closing LDAP pool manager")

	return pm.pool.Close()
}

// PooledLDAPClient represents an LDAP client obtained from the connection pool
// It automatically returns the connection to the pool when closed
type PooledLDAPClient struct {
	client *ldap.LDAP
	conn   *PooledConnection
	pool   *ConnectionPool
	closed bool
}

// AddUserToGroup adds a user to a group using the pooled connection
func (plc *PooledLDAPClient) AddUserToGroup(userDN, groupDN string) error {
	if plc.closed {
		return fmt.Errorf("pooled client is closed")
	}

	return plc.client.AddUserToGroup(userDN, groupDN)
}

// RemoveUserFromGroup removes a user from a group using the pooled connection
func (plc *PooledLDAPClient) RemoveUserFromGroup(userDN, groupDN string) error {
	if plc.closed {
		return fmt.Errorf("pooled client is closed")
	}

	return plc.client.RemoveUserFromGroup(userDN, groupDN)
}

// FindUsers finds all users using the pooled connection
func (plc *PooledLDAPClient) FindUsers() ([]ldap.User, error) {
	if plc.closed {
		return nil, fmt.Errorf("pooled client is closed")
	}

	return plc.client.FindUsers()
}

// FindGroups finds all groups using the pooled connection
func (plc *PooledLDAPClient) FindGroups() ([]ldap.Group, error) {
	if plc.closed {
		return nil, fmt.Errorf("pooled client is closed")
	}

	return plc.client.FindGroups()
}

// FindComputers finds all computers using the pooled connection
func (plc *PooledLDAPClient) FindComputers() ([]ldap.Computer, error) {
	if plc.closed {
		return nil, fmt.Errorf("pooled client is closed")
	}

	return plc.client.FindComputers()
}

// CheckPasswordForSAMAccountName checks password for a SAM account using the pooled connection
func (plc *PooledLDAPClient) CheckPasswordForSAMAccountName(samAccountName, password string) (*ldap.User, error) {
	if plc.closed {
		return nil, fmt.Errorf("pooled client is closed")
	}

	return plc.client.CheckPasswordForSAMAccountName(samAccountName, password)
}

// GetUnderlyingClient returns the underlying LDAP client for advanced operations
// Use this method with caution as it bypasses pool management
func (plc *PooledLDAPClient) GetUnderlyingClient() *ldap.LDAP {
	return plc.client
}

// Close returns the connection to the pool
// This method is idempotent and safe to call multiple times
func (plc *PooledLDAPClient) Close() {
	if plc.closed {
		return
	}

	plc.closed = true
	plc.pool.ReleaseConnection(plc.conn)

	// Clear references to prevent accidental usage
	plc.client = nil
	plc.conn = nil
}

// WithPooledLDAPClient is a utility function that automatically handles connection lifecycle
// It acquires a connection, executes the provided function, and ensures the connection is returned
func WithPooledLDAPClient(
	ctx context.Context, pm *PoolManager, dn, password string, fn func(*PooledLDAPClient) error,
) error {
	client, err := pm.WithCredentials(ctx, dn, password)
	if err != nil {
		return err
	}
	defer client.Close()

	return fn(client)
}
