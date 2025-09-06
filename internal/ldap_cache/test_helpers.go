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

func (m *mockLDAPClient) CheckPasswordForSAMAccountName(samAccountName, password string) (*ldap.User, error) {
	return nil, errors.New("not implemented in mock")
}

func (m *mockLDAPClient) WithCredentials(dn, password string) (*ldap.LDAP, error) {
	return nil, errors.New("not implemented in mock")
}

// Helper functions to create test data
func NewMockUser(dn, samAccountName string, enabled bool, groups []string) ldap.User {
	return ldap.User{
		SAMAccountName: samAccountName,
		Enabled:        enabled,
		Groups:         groups,
	}
}

func NewMockGroup(dn, name string, members []string) ldap.Group {
	return ldap.Group{
		Members: members,
	}
}

func NewMockComputer(dn, samAccountName string, enabled bool, groups []string) ldap.Computer {
	return ldap.Computer{
		SAMAccountName: samAccountName,
		Enabled:        enabled,
		Groups:         groups,
	}
}