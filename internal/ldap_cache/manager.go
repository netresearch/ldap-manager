// Package ldap_cache provides efficient caching of LDAP directory data with automatic refresh capabilities.
// It maintains synchronized in-memory caches for users, groups, and computers with concurrent-safe operations.
//
// Package name uses underscore for LDAP domain clarity (ldap_cache vs ldapcache).
// nolint:revive
package ldap_cache

import (
	"context"
	"sync"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	"github.com/netresearch/ldap-manager/internal/retry"
)

// LDAPClient interface defines the LDAP operations needed by the cache manager.
// This allows for easier testing with mock implementations.
type LDAPClient interface {
	FindUsers() ([]ldap.User, error)
	FindGroups() ([]ldap.Group, error)
	FindComputers() ([]ldap.Computer, error)
	CheckPasswordForSAMAccountName(samAccountName, password string) (*ldap.User, error)
	WithCredentials(dn, password string) (*ldap.LDAP, error)
}

// Manager coordinates LDAP data caching with automatic background refresh.
// It maintains separate caches for users, groups, and computers with configurable refresh intervals.
// All operations are concurrent-safe and provide immediate access to cached data.
// Includes comprehensive metrics tracking for performance monitoring and observability.
// Supports cache warming for faster startup and configurable refresh strategies.
// LDAP operations are automatically retried with exponential backoff for resilience.
type Manager struct {
	stop     chan struct{} // Channel for graceful shutdown signaling
	stopOnce sync.Once     // Ensures Stop() is idempotent
	ctx      context.Context

	client          LDAPClient    // LDAP client for directory operations
	metrics         *Metrics      // Performance metrics and health monitoring
	refreshInterval time.Duration // Configurable refresh interval (default 30s)
	warmupComplete  bool          // Tracks if initial cache warming is complete
	retryConfig     retry.Config  // Retry configuration for LDAP operations

	Users     Cache[ldap.User]     // Cached user entries with O(1) indexed lookups
	Groups    Cache[ldap.Group]    // Cached group entries with O(1) indexed lookups
	Computers Cache[ldap.Computer] // Cached computer entries with O(1) indexed lookups
}

// FullLDAPUser represents a user with populated group memberships.
// This provides a complete view of user data including all associated groups.
type FullLDAPUser struct {
	ldap.User
	Groups []ldap.Group // All groups this user belongs to
}

// FullLDAPGroup represents a group with populated member list.
// This provides a complete view of group data including all member users.
type FullLDAPGroup struct {
	ldap.Group
	Members      []ldap.User  // All users that belong to this group
	ParentGroups []ldap.Group // All groups this group belongs to (from MemberOf)
}

// FullLDAPComputer represents a computer with populated group memberships.
// This provides a complete view of computer data including all associated groups.
type FullLDAPComputer struct {
	ldap.Computer
	Groups []ldap.Group // All groups this computer belongs to
}

// New creates a new LDAP cache manager with the provided LDAP client.
// The manager is initialized with empty caches for users, groups, and computers.
// Uses default 30-second refresh interval. Call NewWithConfig() for custom settings.
// Includes comprehensive metrics tracking for performance monitoring.
// Call Run() to start the background refresh goroutine.
func New(client LDAPClient) *Manager {
	return NewWithConfig(client, 30*time.Second)
}

// NewWithConfig creates a new LDAP cache manager with configurable refresh interval.
// The manager is initialized with empty caches and custom refresh timing.
// Includes comprehensive metrics tracking for performance monitoring.
// LDAP operations are automatically retried with exponential backoff for resilience.
// Call Run() to start the background refresh goroutine.
func NewWithConfig(client LDAPClient, refreshInterval time.Duration) *Manager {
	metrics := NewMetrics()

	return &Manager{
		stop: make(chan struct{}),
		// ctx is initialized with Background and replaced in Run() before any operations
		ctx:             context.Background(),
		client:          client,
		metrics:         metrics,
		refreshInterval: refreshInterval,
		warmupComplete:  false,
		retryConfig:     retry.LDAPConfig(),
		Users:           NewCachedWithMetrics[ldap.User](metrics),
		Groups:          NewCachedWithMetrics[ldap.Group](metrics),
		Computers:       NewCachedWithMetrics[ldap.Computer](metrics),
	}
}

// Run starts the background cache refresh loop.
// It performs initial cache warming, then continues refreshing at the configured interval.
// The context is used for graceful shutdown - when canceled, the loop terminates.
// This method blocks until the context is canceled or Stop() is called.
// Should be run in a separate goroutine.
func (m *Manager) Run(ctx context.Context) {
	// Store context for use in refresh operations.
	// Safe: Run() is called once from Listen() before any cache operations.
	m.ctx = ctx

	// Use configurable refresh interval
	t := time.NewTicker(m.refreshInterval)
	defer t.Stop()

	// Perform initial cache warming with logging
	log.Info().Msg("Starting LDAP cache warming...")
	m.WarmupCache()

	log.Info().Dur("interval", m.refreshInterval).Msg("LDAP cache warmed up, starting refresh loop")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("LDAP cache stopping (context canceled)")

			return
		case <-m.stop:
			log.Info().Msg("LDAP cache stopped")

			return
		case <-t.C:
			m.Refresh()
		}
	}
}

// Stop gracefully shuts down the background refresh loop.
// It closes the stop channel to signal Run() to terminate the refresh cycle.
// Safe to call multiple times; subsequent calls are no-ops due to sync.Once.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stop)
	})
}

// WarmupCache performs initial cache population with enhanced logging and error handling.
// This ensures the cache is fully populated before serving requests.
// Sets warmupComplete flag to indicate readiness for normal operations.
func (m *Manager) WarmupCache() {
	log.Info().Msg("Starting cache warmup process...")
	startTime := time.Now()

	// Parallel cache warming for faster startup
	type warmupResult struct {
		name  string
		count int
		err   error
	}

	results := make(chan warmupResult, 3)

	// Warm up users cache
	go func() {
		if err := m.RefreshUsers(); err != nil {
			results <- warmupResult{"users", 0, err}
		} else {
			results <- warmupResult{"users", m.Users.Count(), nil}
		}
	}()

	// Warm up groups cache
	go func() {
		if err := m.RefreshGroups(); err != nil {
			results <- warmupResult{"groups", 0, err}
		} else {
			results <- warmupResult{"groups", m.Groups.Count(), nil}
		}
	}()

	// Warm up computers cache
	go func() {
		if err := m.RefreshComputers(); err != nil {
			results <- warmupResult{"computers", 0, err}
		} else {
			results <- warmupResult{"computers", m.Computers.Count(), nil}
		}
	}()

	// Collect results
	totalEntities := 0
	hasErrors := false
	for i := 0; i < 3; i++ {
		result := <-results
		if result.err != nil {
			log.Error().Err(result.err).Str("cache", result.name).Msg("Failed to warm up cache")
			m.metrics.RecordRefreshError()
			hasErrors = true
		} else {
			log.Debug().Str("cache", result.name).Int("count", result.count).Msg("Cache warmed up successfully")
			totalEntities += result.count
		}
	}

	duration := time.Since(startTime)

	if !hasErrors {
		m.warmupComplete = true
		m.metrics.RecordRefreshComplete(startTime, m.Users.Count(), m.Groups.Count(), m.Computers.Count())
		log.Info().
			Int("total_entities", totalEntities).
			Dur("duration", duration).
			Msg("Cache warmup completed successfully")
	} else {
		log.Warn().
			Int("total_entities", totalEntities).
			Dur("duration", duration).
			Msg("Cache warmup completed with errors")
	}
}

// IsWarmedUp returns true if the initial cache warming process has completed successfully.
// Used to determine if the cache is ready to serve requests optimally.
func (m *Manager) IsWarmedUp() bool {
	return m.warmupComplete
}

// RefreshUsers fetches all users from LDAP and updates the user cache.
// Uses retry logic with exponential backoff for resilience against transient failures.
// Returns an error if all retry attempts fail, otherwise replaces the entire user cache.
func (m *Manager) RefreshUsers() error {
	users, err := retry.DoWithResultConfig(m.ctx, m.retryConfig, func() ([]ldap.User, error) {
		return m.client.FindUsers()
	})
	if err != nil {
		return err
	}

	m.Users.setAll(users)

	return nil
}

// RefreshGroups fetches all groups from LDAP and updates the group cache.
// Uses retry logic with exponential backoff for resilience against transient failures.
// Returns an error if all retry attempts fail, otherwise replaces the entire group cache.
func (m *Manager) RefreshGroups() error {
	groups, err := retry.DoWithResultConfig(m.ctx, m.retryConfig, func() ([]ldap.Group, error) {
		return m.client.FindGroups()
	})
	if err != nil {
		return err
	}

	m.Groups.setAll(groups)

	return nil
}

// RefreshComputers fetches all computers from LDAP and updates the computer cache.
// Uses retry logic with exponential backoff for resilience against transient failures.
// Returns an error if all retry attempts fail, otherwise replaces the entire computer cache.
func (m *Manager) RefreshComputers() error {
	computers, err := retry.DoWithResultConfig(m.ctx, m.retryConfig, func() ([]ldap.Computer, error) {
		return m.client.FindComputers()
	})
	if err != nil {
		return err
	}

	m.Computers.setAll(computers)

	return nil
}

// Refresh updates all caches (users, groups, computers) from LDAP.
// Individual failures are logged as errors but don't stop other cache updates.
// This method is called automatically by the background refresh loop.
// Records comprehensive metrics for monitoring and observability.
func (m *Manager) Refresh() {
	startTime := m.metrics.RecordRefreshStart()
	hasErrors := false

	if err := m.RefreshUsers(); err != nil {
		log.Error().Err(err).Msg("Failed to refresh users cache")
		m.metrics.RecordRefreshError()
		hasErrors = true
	}

	if err := m.RefreshGroups(); err != nil {
		log.Error().Err(err).Msg("Failed to refresh groups cache")
		m.metrics.RecordRefreshError()
		hasErrors = true
	}

	if err := m.RefreshComputers(); err != nil {
		log.Error().Err(err).Msg("Failed to refresh computers cache")
		m.metrics.RecordRefreshError()
		hasErrors = true
	}

	// Record successful completion metrics
	if !hasErrors {
		m.metrics.RecordRefreshComplete(startTime,
			m.Users.Count(), m.Groups.Count(), m.Computers.Count())
	}

	log.Debug().Msgf("Refreshed LDAP cache with %d users, %d groups and %d computers",
		m.Users.Count(), m.Groups.Count(), m.Computers.Count())
}

// FindUsers returns all cached users, optionally filtering out disabled users.
// When showDisabled is true, returns all users including disabled ones.
// When false, returns only enabled users. Uses efficient filtering on cached data.
func (m *Manager) FindUsers(showDisabled bool) []ldap.User {
	if !showDisabled {
		return m.Users.Filter(func(u ldap.User) bool {
			return u.Enabled
		})
	}

	return m.Users.Get()
}

// FindUserByDN finds a user by their Distinguished Name (DN) in the cache.
// Returns the user if found, or ErrUserNotFound if no matching user exists.
// This is now an O(1) operation using hash-based index lookup for optimal performance.
func (m *Manager) FindUserByDN(dn string) (*ldap.User, error) {
	user, found := m.Users.FindByDN(dn)
	if !found {
		return nil, ldap.ErrUserNotFound
	}

	return user, nil
}

// FindUserBySAMAccountName finds a user by their SAMAccountName in the cache.
// Returns the user if found, or ErrUserNotFound if no matching user exists.
// This is now an O(1) operation using hash-based index lookup for optimal performance.
// Provides significant performance improvement over the previous O(n) linear search.
func (m *Manager) FindUserBySAMAccountName(samAccountName string) (*ldap.User, error) {
	user, found := m.Users.FindBySAMAccountName(samAccountName)
	if !found {
		return nil, ldap.ErrUserNotFound
	}

	return user, nil
}

// FindGroups returns all cached groups from the LDAP directory.
// Provides immediate access to group data without LDAP queries.
func (m *Manager) FindGroups() []ldap.Group {
	return m.Groups.Get()
}

// FindGroupByDN finds a group by its Distinguished Name (DN) in the cache.
// Returns the group if found, or ErrGroupNotFound if no matching group exists.
// This is now an O(1) operation using hash-based index lookup for optimal performance.
func (m *Manager) FindGroupByDN(dn string) (*ldap.Group, error) {
	group, found := m.Groups.FindByDN(dn)

	if !found {
		return nil, ldap.ErrGroupNotFound
	}

	return group, nil
}

// FindComputers returns all cached computers, optionally filtering out disabled computers.
// When showDisabled is true, returns all computers including disabled ones.
// When false, returns only enabled computers. Uses efficient filtering on cached data.
func (m *Manager) FindComputers(showDisabled bool) []ldap.Computer {
	if !showDisabled {
		return m.Computers.Filter(func(c ldap.Computer) bool {
			return c.Enabled
		})
	}

	return m.Computers.Get()
}

// FindComputerByDN finds a computer by its Distinguished Name (DN) in the cache.
// Returns the computer if found, or ErrComputerNotFound if no matching computer exists.
// This is now an O(1) operation using hash-based index lookup for optimal performance.
func (m *Manager) FindComputerByDN(dn string) (*ldap.Computer, error) {
	computer, found := m.Computers.FindByDN(dn)
	if !found {
		return nil, ldap.ErrComputerNotFound
	}

	return computer, nil
}

// FindComputerBySAMAccountName finds a computer by its SAMAccountName in the cache.
// Returns the computer if found, or ErrComputerNotFound if no matching computer exists.
// This is an O(1) operation using hash-based index lookup for optimal performance.
// Provides significant performance improvement for computer lookups by account name.
func (m *Manager) FindComputerBySAMAccountName(samAccountName string) (*ldap.Computer, error) {
	computer, found := m.Computers.FindBySAMAccountName(samAccountName)
	if !found {
		return nil, ldap.ErrComputerNotFound
	}

	return computer, nil
}

// PopulateGroupsForUser creates a FullLDAPUser with populated group memberships.
// Takes a user and finds all groups where the user is a member.
// This implementation iterates through all cached groups to find membership,
// which works correctly even when OpenLDAP's memberOf overlay is not enabled.
// Returns a complete user object with expanded group information.
func (m *Manager) PopulateGroupsForUser(user *ldap.User) *FullLDAPUser {
	return PopulateGroupsForUserFromData(user, m.Groups.Get())
}

// PopulateUsersForGroup creates a FullLDAPGroup with populated member list.
// Takes a group and resolves all member DNs to full user objects from the cache.
// Also resolves parent groups from the MemberOf field.
// When showDisabled is false, filters out disabled users from membership.
// Returns a complete group object with expanded member and parent group information.
func (m *Manager) PopulateUsersForGroup(group *ldap.Group, showDisabled bool) *FullLDAPGroup {
	return PopulateUsersForGroupFromData(group, m.Users.Get(), m.Groups.Get(), showDisabled)
}

// PopulateGroupsForComputer creates a FullLDAPComputer with populated group memberships.
// Takes a computer and finds all groups where the computer is a member.
// This implementation iterates through all cached groups to find membership,
// which works correctly even when OpenLDAP's memberOf overlay is not enabled.
// Returns a complete computer object with expanded group information.
func (m *Manager) PopulateGroupsForComputer(computer *ldap.Computer) *FullLDAPComputer {
	return PopulateGroupsForComputerFromData(computer, m.Groups.Get())
}

// OnAddUserToGroup updates cache when a user is added to a group.
// Synchronizes both user and group cache entries to maintain consistency.
// Should be called after successful LDAP operations to keep cache current.
func (m *Manager) OnAddUserToGroup(userDN, groupDN string) {
	m.Users.update(func(user *ldap.User) {
		if user.DN() != userDN {
			return
		}

		user.Groups = append(user.Groups, groupDN)
	})

	m.Groups.update(func(group *ldap.Group) {
		if group.DN() != groupDN {
			return
		}

		group.Members = append(group.Members, userDN)
	})
}

// OnRemoveUserFromGroup updates cache when a user is removed from a group.
// Synchronizes both user and group cache entries to maintain consistency.
// Should be called after successful LDAP operations to keep cache current.
func (m *Manager) OnRemoveUserFromGroup(userDN, groupDN string) {
	m.Users.update(func(user *ldap.User) {
		if user.DN() != userDN {
			return
		}

		for idx, group := range user.Groups {
			if group == groupDN {
				user.Groups = append(user.Groups[:idx], user.Groups[idx+1:]...)
			}
		}
	})

	m.Groups.update(func(group *ldap.Group) {
		if group.DN() != groupDN {
			return
		}

		for idx, member := range group.Members {
			if member == userDN {
				group.Members = append(group.Members[:idx], group.Members[idx+1:]...)
			}
		}
	})
}

// PopulateGroupsForUserFromData creates a FullLDAPUser with populated group memberships
// using provided data instead of cache. Works identically to PopulateGroupsForUser
// but operates on explicit slices rather than the cache.
func PopulateGroupsForUserFromData(user *ldap.User, allGroups []ldap.Group) *FullLDAPUser {
	full := &FullLDAPUser{
		User:   *user,
		Groups: make([]ldap.Group, 0),
	}

	userDN := user.DN()

	for _, group := range allGroups {
		for _, memberDN := range group.Members {
			if memberDN == userDN {
				full.Groups = append(full.Groups, group)

				break
			}
		}
	}

	return full
}

// PopulateUsersForGroupFromData creates a FullLDAPGroup with populated member list
// using provided data instead of cache. Works identically to PopulateUsersForGroup
// but operates on explicit slices rather than the cache.
func PopulateUsersForGroupFromData(
	group *ldap.Group, allUsers []ldap.User, allGroups []ldap.Group, showDisabled bool,
) *FullLDAPGroup {
	full := &FullLDAPGroup{
		Group:        *group,
		Members:      make([]ldap.User, 0),
		ParentGroups: make([]ldap.Group, 0),
	}

	// Build a map for O(1) user lookups by DN
	usersByDN := make(map[string]*ldap.User, len(allUsers))
	for i := range allUsers {
		usersByDN[allUsers[i].DN()] = &allUsers[i]
	}

	for _, memberDN := range group.Members {
		if user, ok := usersByDN[memberDN]; ok {
			if !showDisabled && !user.Enabled {
				continue
			}

			full.Members = append(full.Members, *user)
		}
	}

	// Build a map for O(1) group lookups by DN
	groupsByDN := make(map[string]*ldap.Group, len(allGroups))
	for i := range allGroups {
		groupsByDN[allGroups[i].DN()] = &allGroups[i]
	}

	for _, parentDN := range group.MemberOf {
		if parentGroup, ok := groupsByDN[parentDN]; ok {
			full.ParentGroups = append(full.ParentGroups, *parentGroup)
		}
	}

	return full
}

// PopulateGroupsForComputerFromData creates a FullLDAPComputer with populated group memberships
// using provided data instead of cache. Works identically to PopulateGroupsForComputer
// but operates on explicit slices rather than the cache.
func PopulateGroupsForComputerFromData(computer *ldap.Computer, allGroups []ldap.Group) *FullLDAPComputer {
	full := &FullLDAPComputer{
		Computer: *computer,
		Groups:   make([]ldap.Group, 0),
	}

	computerDN := computer.DN()

	for _, group := range allGroups {
		for _, memberDN := range group.Members {
			if memberDN == computerDN {
				full.Groups = append(full.Groups, group)

				break
			}
		}
	}

	return full
}

// GetMetrics returns the current cache metrics for monitoring and observability.
// Provides comprehensive statistics about cache performance, health, and operations.
func (m *Manager) GetMetrics() *Metrics {
	return m.metrics
}

// GetHealthCheck returns a summary of cache health and performance metrics.
// Useful for health check endpoints and monitoring dashboards.
func (m *Manager) GetHealthCheck() SummaryStats {
	return m.metrics.GetSummaryStats()
}

// IsHealthy returns true if the cache system is operating normally.
// Checks for recent successful refreshes and low error rates.
func (m *Manager) IsHealthy() bool {
	return m.metrics.GetHealthStatus() == HealthHealthy
}
