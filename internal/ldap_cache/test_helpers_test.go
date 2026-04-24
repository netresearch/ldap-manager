// Package ldap_cache provides thread-safe caching for LDAP entities with test helpers.
// nolint:revive
package ldap_cache

import (
	"errors"
	"reflect"
	"sync"
	"unsafe"

	ldap "github.com/netresearch/simple-ldap-go"
)

// mockLDAPClient implements LDAPClient for testing
type mockLDAPClient struct {
	mu        sync.Mutex
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
	m.mu.Lock()
	m.callCounts.findUsers++
	m.mu.Unlock()

	if m.findUsersError != nil {
		return nil, m.findUsersError
	}

	return m.users, nil
}

func (m *mockLDAPClient) FindGroups() ([]ldap.Group, error) {
	m.mu.Lock()
	m.callCounts.findGroups++
	m.mu.Unlock()

	if m.findGroupsError != nil {
		return nil, m.findGroupsError
	}

	return m.groups, nil
}

func (m *mockLDAPClient) FindComputers() ([]ldap.Computer, error) {
	m.mu.Lock()
	m.callCounts.findComputers++
	m.mu.Unlock()

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

// Test-only helpers: seed ldap.User / Group / Computer with a real DN by
// poking simple-ldap-go's unexported Object fields. Production code builds
// these via objectFromEntry internal to that package.

// newUserWithDN creates a ldap.User with the DN and CN fields populated via reflection.
func newUserWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.User {
	u := ldap.User{SAMAccountName: sam, Enabled: enabled, Groups: groups}
	setObjectFields(reflect.ValueOf(&u).Elem().FieldByName("Object"), dn, cn)

	return u
}

// newGroupWithDN creates a ldap.Group with the DN and CN fields populated via reflection.
func newGroupWithDN(dn, cn string, members []string) ldap.Group {
	g := ldap.Group{Members: members}
	setObjectFields(reflect.ValueOf(&g).Elem().FieldByName("Object"), dn, cn)

	return g
}

// newComputerWithDN creates a ldap.Computer with the DN and CN fields populated via reflection.
func newComputerWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.Computer { //nolint:unused
	c := ldap.Computer{SAMAccountName: sam, Enabled: enabled, Groups: groups}
	setObjectFields(reflect.ValueOf(&c).Elem().FieldByName("Object"), dn, cn)

	return c
}

// setObjectFields uses unsafe pointer arithmetic to write the unexported dn and cn
// fields of simple-ldap-go's Object struct. This is only valid in test code where
// we need to seed deterministic DNs without going through LDAP entry parsing.
func setObjectFields(obj reflect.Value, dn, cn string) {
	dnField := obj.FieldByName("dn")
	cnField := obj.FieldByName("cn")
	reflect.NewAt(dnField.Type(), unsafe.Pointer(dnField.UnsafeAddr())).Elem().SetString(dn) //nolint:gosec
	reflect.NewAt(cnField.Type(), unsafe.Pointer(cnField.UnsafeAddr())).Elem().SetString(cn) //nolint:gosec
}
