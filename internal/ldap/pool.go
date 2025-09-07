// Package ldap provides LDAP connection pooling capabilities for efficient resource management
// and improved performance when handling concurrent LDAP operations.
package ldap

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

var (
	// ErrPoolClosed indicates the connection pool has been shut down
	ErrPoolClosed = errors.New("connection pool is closed")
	// ErrConnectionTimeout indicates timeout while acquiring connection
	ErrConnectionTimeout = errors.New("timeout acquiring connection from pool")
	// ErrInvalidCredentials indicates authentication failure
	ErrInvalidCredentials = errors.New("invalid LDAP credentials")
)

// PoolConfig contains configuration options for the LDAP connection pool
type PoolConfig struct {
	MaxConnections      int           // Maximum number of connections in pool (default: 10)
	MinConnections      int           // Minimum number of connections to maintain (default: 2)
	MaxIdleTime         time.Duration // Maximum time a connection can be idle (default: 15min)
	MaxLifetime         time.Duration // Maximum lifetime of a connection (default: 1hour)
	HealthCheckInterval time.Duration // Interval for connection health checks (default: 30s)
	AcquireTimeout      time.Duration // Timeout for acquiring a connection (default: 10s)
}

// DefaultPoolConfig returns a default configuration for the connection pool
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConnections:      10,
		MinConnections:      2,
		MaxIdleTime:         15 * time.Minute,
		MaxLifetime:         1 * time.Hour,
		HealthCheckInterval: 30 * time.Second,
		AcquireTimeout:      10 * time.Second,
	}
}

// PooledConnection represents a connection in the pool with metadata
type PooledConnection struct {
	client      *ldap.LDAP
	credentials *ConnectionCredentials
	createdAt   time.Time
	lastUsedAt  time.Time
	inUse       bool
	healthy     bool
	mutex       sync.RWMutex
}

// ConnectionCredentials stores authentication information for a pooled connection
type ConnectionCredentials struct {
	DN       string
	Password string
}

// ConnectionPool manages a pool of LDAP connections for efficient reuse
type ConnectionPool struct {
	config      *PoolConfig
	baseClient  *ldap.LDAP
	connections []*PooledConnection
	available   chan *PooledConnection
	mutex       sync.RWMutex
	closed      int32
	stopChan    chan struct{}
	wg          sync.WaitGroup

	// Metrics
	totalConnections    int32
	activeConnections   int32
	acquiredConnections int64
	failedConnections   int64
}

// NewConnectionPool creates a new LDAP connection pool with the specified configuration
func NewConnectionPool(baseClient *ldap.LDAP, config *PoolConfig) (*ConnectionPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	// Validate configuration
	if config.MaxConnections <= 0 {
		config.MaxConnections = 10
	}
	if config.MinConnections < 0 {
		config.MinConnections = 2
	}
	if config.MinConnections > config.MaxConnections {
		config.MinConnections = config.MaxConnections
	}
	if config.MaxIdleTime <= 0 {
		config.MaxIdleTime = 15 * time.Minute
	}
	if config.MaxLifetime <= 0 {
		config.MaxLifetime = 1 * time.Hour
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.AcquireTimeout <= 0 {
		config.AcquireTimeout = 10 * time.Second
	}

	pool := &ConnectionPool{
		config:      config,
		baseClient:  baseClient,
		connections: make([]*PooledConnection, 0, config.MaxConnections),
		available:   make(chan *PooledConnection, config.MaxConnections),
		stopChan:    make(chan struct{}),
	}

	// Pre-create minimum connections
	if err := pool.warmupPool(); err != nil {
		log.Warn().Err(err).Msg("Failed to warm up connection pool, continuing with empty pool")
	}

	// Start background maintenance
	pool.wg.Add(1)
	go pool.maintenanceLoop()

	log.Info().
		Int("max_connections", config.MaxConnections).
		Int("min_connections", config.MinConnections).
		Dur("max_idle_time", config.MaxIdleTime).
		Dur("max_lifetime", config.MaxLifetime).
		Msg("LDAP connection pool initialized")

	return pool, nil
}

// warmupPool pre-creates minimum connections for faster initial requests
func (p *ConnectionPool) warmupPool() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Create readonly connections for warmup (using base client credentials)
	for i := 0; i < p.config.MinConnections; i++ {
		conn, err := p.createConnection(nil)
		if err != nil {
			log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to create warmup connection")

			continue
		}

		p.connections = append(p.connections, conn)
		select {
		case p.available <- conn:
			atomic.AddInt32(&p.totalConnections, 1)
		default:
			// Channel full, should not happen during warmup
			p.closeConnection(conn)
		}
	}

	log.Debug().Int("warmed_connections", len(p.connections)).Msg("Connection pool warmed up")

	return nil
}

// AcquireConnection gets a connection from the pool with the specified credentials
func (p *ConnectionPool) AcquireConnection(ctx context.Context, dn, password string) (*PooledConnection, error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, ErrPoolClosed
	}

	// Set timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.config.AcquireTimeout)
	defer cancel()

	credentials := &ConnectionCredentials{DN: dn, Password: password}

	// Try to get an existing connection or create a new one
	conn, err := p.getOrCreateConnection(timeoutCtx, credentials)
	if err != nil {
		atomic.AddInt64(&p.failedConnections, 1)

		return nil, err
	}

	// Mark connection as in use
	conn.mutex.Lock()
	conn.inUse = true
	conn.lastUsedAt = time.Now()
	conn.mutex.Unlock()

	atomic.AddInt32(&p.activeConnections, 1)
	atomic.AddInt64(&p.acquiredConnections, 1)

	return conn, nil
}

// getOrCreateConnection attempts to reuse or create a connection
func (p *ConnectionPool) getOrCreateConnection(
	ctx context.Context, creds *ConnectionCredentials,
) (*PooledConnection, error) {
	// Try to get an existing connection with matching credentials
	select {
	case conn := <-p.available:
		if p.canReuseConnection(conn, creds) {
			return conn, nil
		}
		// Connection not reusable, close it and create a new one
		p.closeConnection(conn)
		atomic.AddInt32(&p.totalConnections, -1)
	case <-ctx.Done():
		return nil, ErrConnectionTimeout
	default:
		// No available connections, try to create new one
	}

	// Check if we can create more connections
	maxConn32 := safeIntToInt32(p.config.MaxConnections)
	if atomic.LoadInt32(&p.totalConnections) >= maxConn32 {
		// Wait for an available connection
		select {
		case conn := <-p.available:
			if p.canReuseConnection(conn, creds) {
				return conn, nil
			}
			p.closeConnection(conn)
			atomic.AddInt32(&p.totalConnections, -1)
		case <-ctx.Done():
			return nil, ErrConnectionTimeout
		}
	}

	// Create new connection
	conn, err := p.createConnection(creds)
	if err != nil {
		return nil, err
	}

	atomic.AddInt32(&p.totalConnections, 1)
	return conn, nil
}

// canReuseConnection checks if an existing connection can be reused for the given credentials
func (p *ConnectionPool) canReuseConnection(conn *PooledConnection, creds *ConnectionCredentials) bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()

	// Check if connection is healthy and not expired
	if !conn.healthy {
		return false
	}

	now := time.Now()
	if now.Sub(conn.createdAt) > p.config.MaxLifetime {
		return false
	}

	if now.Sub(conn.lastUsedAt) > p.config.MaxIdleTime {
		return false
	}

	// Check if credentials match (for connection reuse)
	if conn.credentials != nil && creds != nil {
		return conn.credentials.DN == creds.DN && conn.credentials.Password == creds.Password
	}

	// Allow reuse if both are nil (readonly connections)
	return conn.credentials == nil && creds == nil
}

// createConnection creates a new LDAP connection with the specified credentials
func (p *ConnectionPool) createConnection(creds *ConnectionCredentials) (*PooledConnection, error) {
	var client *ldap.LDAP
	var err error

	if creds != nil {
		// Create authenticated connection
		client, err = p.baseClient.WithCredentials(creds.DN, creds.Password)
		if err != nil {
			return nil, ErrInvalidCredentials
		}
	} else {
		// Use base client for readonly operations
		client = p.baseClient
	}

	conn := &PooledConnection{
		client:      client,
		credentials: creds,
		createdAt:   time.Now(),
		lastUsedAt:  time.Now(),
		healthy:     true,
		inUse:       false,
	}

	return conn, nil
}

// ReleaseConnection returns a connection to the pool
func (p *ConnectionPool) ReleaseConnection(conn *PooledConnection) {
	if conn == nil {
		return
	}

	conn.mutex.Lock()
	conn.inUse = false
	conn.lastUsedAt = time.Now()
	conn.mutex.Unlock()

	atomic.AddInt32(&p.activeConnections, -1)

	// Check if connection is still healthy and not expired
	if !p.isConnectionValid(conn) {
		p.closeConnection(conn)
		atomic.AddInt32(&p.totalConnections, -1)
		return
	}

	// Return to pool
	select {
	case p.available <- conn:
		// Successfully returned to pool
	default:
		// Pool is full, close this connection
		p.closeConnection(conn)
		atomic.AddInt32(&p.totalConnections, -1)
	}
}

// isConnectionValid checks if a connection is still valid for pool reuse
func (p *ConnectionPool) isConnectionValid(conn *PooledConnection) bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()

	if !conn.healthy {
		return false
	}

	now := time.Now()
	return now.Sub(conn.createdAt) <= p.config.MaxLifetime
}

// closeConnection properly closes a connection and cleans up resources
func (p *ConnectionPool) closeConnection(conn *PooledConnection) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	if conn.client != nil {
		// Note: simple-ldap-go doesn't expose a Close() method
		// The underlying connection will be cleaned up by GC
		conn.client = nil
	}
	conn.healthy = false
}

// maintenanceLoop runs periodic maintenance tasks
func (p *ConnectionPool) maintenanceLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.performMaintenance()
		}
	}
}

// performMaintenance cleans up expired connections and maintains pool health
func (p *ConnectionPool) performMaintenance() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	var validConnections []*PooledConnection

	// Clean up expired connections
	for _, conn := range p.connections {
		conn.mutex.RLock()
		expired := now.Sub(conn.createdAt) > p.config.MaxLifetime ||
			(now.Sub(conn.lastUsedAt) > p.config.MaxIdleTime && !conn.inUse)
		conn.mutex.RUnlock()

		if expired || !conn.healthy {
			p.closeConnection(conn)
			atomic.AddInt32(&p.totalConnections, -1)
		} else {
			validConnections = append(validConnections, conn)
		}
	}

	p.connections = validConnections

	// Log maintenance stats
	totalConns := atomic.LoadInt32(&p.totalConnections)
	activeConns := atomic.LoadInt32(&p.activeConnections)

	log.Debug().
		Int32("total_connections", totalConns).
		Int32("active_connections", activeConns).
		Int("available_connections", len(p.available)).
		Msg("Connection pool maintenance completed")
}

// GetClient returns the LDAP client from a pooled connection
func (conn *PooledConnection) GetClient() *ldap.LDAP {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.client
}

// GetStats returns current pool statistics for monitoring
func (p *ConnectionPool) GetStats() PoolStats {
	return PoolStats{
		TotalConnections:     atomic.LoadInt32(&p.totalConnections),
		ActiveConnections:    atomic.LoadInt32(&p.activeConnections),
		AvailableConnections: safeIntToInt32(len(p.available)),
		AcquiredCount:        atomic.LoadInt64(&p.acquiredConnections),
		FailedCount:          atomic.LoadInt64(&p.failedConnections),
		MaxConnections:       safeIntToInt32(p.config.MaxConnections),
	}
}

// safeIntToInt32 safely converts int to int32 with overflow protection
func safeIntToInt32(value int) int32 {
	if value > 2147483647 { // int32 max
		return 2147483647
	}
	if value < -2147483648 { // int32 min
		return -2147483648
	}
	return int32(value) // #nosec G115 - safe conversion after bounds check
}

// PoolStats contains statistics about pool usage
type PoolStats struct {
	TotalConnections     int32 `json:"total_connections"`
	ActiveConnections    int32 `json:"active_connections"`
	AvailableConnections int32 `json:"available_connections"`
	AcquiredCount        int64 `json:"acquired_count"`
	FailedCount          int64 `json:"failed_count"`
	MaxConnections       int32 `json:"max_connections"`
}

// Close gracefully shuts down the connection pool
func (p *ConnectionPool) Close() error {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return nil // Already closed
	}

	log.Info().Msg("Shutting down LDAP connection pool")

	// Stop maintenance loop
	close(p.stopChan)
	p.wg.Wait()

	// Close all connections
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, conn := range p.connections {
		p.closeConnection(conn)
	}

	// Drain available channel
	close(p.available)
	for conn := range p.available {
		p.closeConnection(conn)
	}

	p.connections = nil

	log.Info().Msg("LDAP connection pool shutdown complete")
	return nil
}
