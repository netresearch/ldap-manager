// Package ldap_cache provides efficient caching of LDAP directory data with automatic refresh capabilities.
// It maintains synchronized in-memory caches for users, groups, and computers with concurrent-safe operations.
package ldap_cache

import (
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
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
// It maintains separate caches for users, groups, and computers, refreshing every 30 seconds.
// All operations are concurrent-safe and provide immediate access to cached data.
type Manager struct {
	stop chan struct{} // Channel for graceful shutdown signaling

	client LDAPClient // LDAP client for directory operations

	Users     Cache[ldap.User]     // Cached user entries
	Groups    Cache[ldap.Group]    // Cached group entries
	Computers Cache[ldap.Computer] // Cached computer entries
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
	Members []ldap.User // All users that belong to this group
}

// FullLDAPComputer represents a computer with populated group memberships.
// This provides a complete view of computer data including all associated groups.
type FullLDAPComputer struct {
	ldap.Computer
	Groups []ldap.Group // All groups this computer belongs to
}

// New creates a new LDAP cache manager with the provided LDAP client.
// The manager is initialized with empty caches for users, groups, and computers.
// Call Run() to start the background refresh goroutine.
func New(client LDAPClient) *Manager {
	return &Manager{
		stop:      make(chan struct{}),
		client:    client,
		Users:     NewCached[ldap.User](),
		Groups:    NewCached[ldap.Group](),
		Computers: NewCached[ldap.Computer](),
	}
}

// Run starts the background cache refresh loop.
// It performs an initial cache refresh, then continues refreshing every 30 seconds.
// This method blocks until Stop() is called. Should be run in a separate goroutine.
func (m *Manager) Run() {
	t := time.NewTicker(30 * time.Second)

	// Perform initial cache population
	m.Refresh()

	for {
		select {
		case <-m.stop:
			t.Stop()
			log.Info().Msg("LDAP cache stopped")
			return
		case <-t.C:
			m.Refresh()
		}
	}
}

// Stop gracefully shuts down the background refresh loop.
// It sends a signal to the Run() method to terminate the refresh cycle.
func (m *Manager) Stop() {
	m.stop <- struct{}{}
}

// RefreshUsers fetches all users from LDAP and updates the user cache.
// Returns an error if the LDAP query fails, otherwise replaces the entire user cache.
func (m *Manager) RefreshUsers() error {
	users, err := m.client.FindUsers()
	if err != nil {
		return err
	}

	m.Users.setAll(users)
	return nil
}

// RefreshGroups fetches all groups from LDAP and updates the group cache.
// Returns an error if the LDAP query fails, otherwise replaces the entire group cache.
func (m *Manager) RefreshGroups() error {
	groups, err := m.client.FindGroups()
	if err != nil {
		return err
	}

	m.Groups.setAll(groups)
	return nil
}

// RefreshComputers fetches all computers from LDAP and updates the computer cache.
// Returns an error if the LDAP query fails, otherwise replaces the entire computer cache.
func (m *Manager) RefreshComputers() error {
	computers, err := m.client.FindComputers()
	if err != nil {
		return err
	}

	m.Computers.setAll(computers)
	return nil
}

// Refresh updates all caches (users, groups, computers) from LDAP.
// Individual failures are logged as errors but don't stop other cache updates.
// This method is called automatically by the background refresh loop.
func (m *Manager) Refresh() {
	if err := m.RefreshUsers(); err != nil {
		log.Error().Err(err).Msg("Failed to refresh users cache")
	}

	if err := m.RefreshGroups(); err != nil {
		log.Error().Err(err).Msg("Failed to refresh groups cache")
	}

	if err := m.RefreshComputers(); err != nil {
		log.Error().Err(err).Msg("Failed to refresh computers cache")
	}

	log.Debug().Msgf("Refreshed LDAP cache with %d users, %d groups and %d computers", 
		m.Users.Count(), m.Groups.Count(), m.Computers.Count())
}

func (m *Manager) FindUsers(showDisabled bool) []ldap.User {
	if !showDisabled {
		return m.Users.Filter(func(t ldap.User) bool {
			return t.Enabled
		})
	}

	return m.Users.Get()
}

func (m *Manager) FindUserByDN(dn string) (*ldap.User, error) {
	user, found := m.Users.FindByDN(dn)
	if !found {
		return nil, ldap.ErrUserNotFound
	}

	return user, nil
}

func (m *Manager) FindUserBySAMAccountName(samAccountName string) (*ldap.User, error) {
	user, found := m.Users.Find(func(user ldap.User) bool {
		return user.SAMAccountName == samAccountName
	})
	if !found {
		return nil, ldap.ErrUserNotFound
	}

	return user, nil
}

func (m *Manager) FindGroups() []ldap.Group {
	return m.Groups.Get()
}

func (m *Manager) FindGroupByDN(dn string) (*ldap.Group, error) {
	group, found := m.Groups.FindByDN(dn)

	if !found {
		return nil, ldap.ErrGroupNotFound
	}

	return group, nil
}

func (m *Manager) FindComputers(showDisabled bool) []ldap.Computer {
	if !showDisabled {
		return m.Computers.Filter(func(t ldap.Computer) bool {
			return t.Enabled
		})
	}

	return m.Computers.Get()
}

func (m *Manager) FindComputerByDN(dn string) (*ldap.Computer, error) {
	computer, found := m.Computers.FindByDN(dn)
	if !found {
		return nil, ldap.ErrComputerNotFound
	}

	return computer, nil
}

func (m *Manager) PopulateGroupsForUser(user *ldap.User) *FullLDAPUser {
	full := &FullLDAPUser{
		User:   *user,
		Groups: make([]ldap.Group, 0),
	}

	for _, groupDN := range user.Groups {
		group, err := m.FindGroupByDN(groupDN)
		if err == nil {
			full.Groups = append(full.Groups, *group)
		}
	}

	return full
}

func (m *Manager) PopulateUsersForGroup(group *ldap.Group, showDisabled bool) *FullLDAPGroup {
	full := &FullLDAPGroup{
		Group:   *group,
		Members: make([]ldap.User, 0),
	}

	for _, userDN := range group.Members {
		user, err := m.FindUserByDN(userDN)
		if err == nil {
			if showDisabled && !user.Enabled {
				continue
			}

			full.Members = append(full.Members, *user)
		}
	}

	return full
}

func (m *Manager) PopulateGroupsForComputer(computer *ldap.Computer) *FullLDAPComputer {
	full := &FullLDAPComputer{
		Computer: *computer,
		Groups:   make([]ldap.Group, 0),
	}

	for _, groupDN := range computer.Groups {
		group, err := m.FindGroupByDN(groupDN)
		if err == nil {
			full.Groups = append(full.Groups, *group)
		}
	}

	return full
}

func (m *Manager) OnAddUserToGroup(userDN string, groupDN string) {
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

func (m *Manager) OnRemoveUserFromGroup(userDN string, groupDN string) {
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
