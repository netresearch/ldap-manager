// Package ldap_cache provides thread-safe caching for LDAP entities with test helpers.
// nolint:revive
package ldap_cache

import (
	"errors"

	ldap "github.com/netresearch/simple-ldap-go"
)

// mockLDAPClient implements LDAPClient for testing
type mockLDAPClient struct {
	users     []ldap.User
	groups    []ldap.Group
	computers []ldap.Computer

	findUsersError     error
	findGroupsError    error
	findComputersError error

	callCounts struct {
		findUsers     int
		findGroups    int
		findComputers int
	}
}

func (m *mockLDAPClient) FindUsers() ([]ldap.User, error) {
	m.callCounts.findUsers++
	if m.findUsersError != nil {
		return nil, m.findUsersError
	}

	return m.users, nil
}

func (m *mockLDAPClient) FindGroups() ([]ldap.Group, error) {
	m.callCounts.findGroups++
	if m.findGroupsError != nil {
		return nil, m.findGroupsError
	}

	return m.groups, nil
}

func (m *mockLDAPClient) FindComputers() ([]ldap.Computer, error) {
	m.callCounts.findComputers++
	if m.findComputersError != nil {
		return nil, m.findComputersError
	}

	return m.computers, nil
}

func (m *mockLDAPClient) CheckPasswordForSAMAccountName(_, _ string) (*ldap.User, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockLDAPClient) WithCredentials(_, _ string) (*ldap.LDAP, error) {
	return nil, errors.New("not implemented in mock")
}

// NewMockUser creates a mock LDAP user for testing purposes.
// Parameters: dn (Distinguished Name), samAccountName, enabled status, and group DNs.
// Returns a properly configured ldap.User instance for use in unit tests.
func NewMockUser(_, samAccountName string, enabled bool, groups []string) ldap.User {
	return ldap.User{
		SAMAccountName: samAccountName,
		Enabled:        enabled,
		Groups:         groups,
	}
}

// NewMockGroup creates a mock LDAP group for testing purposes.
// Parameters: dn (Distinguished Name), group name, and member user DNs.
// Returns a properly configured ldap.Group instance for use in unit tests.
func NewMockGroup(_, _ string, members []string) ldap.Group {
	return ldap.Group{
		Members: members,
	}
}

// NewMockComputer creates a mock LDAP computer for testing purposes.
// Parameters: dn (Distinguished Name), samAccountName, enabled status, and group DNs.
// Returns a properly configured ldap.Computer instance for use in unit tests.
func NewMockComputer(_, samAccountName string, enabled bool, groups []string) ldap.Computer {
	return ldap.Computer{
		SAMAccountName: samAccountName,
		Enabled:        enabled,
		Groups:         groups,
	}
}
