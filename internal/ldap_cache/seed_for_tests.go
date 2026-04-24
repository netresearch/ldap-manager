// Package ldap_cache — seed_for_tests.go: exposes test-only seeding hooks
// that bypass LDAP I/O. NOT for production use; named with the
// `ForTesting` suffix per the Go convention so calls stand out at review.
//
//nolint:revive // package name intentionally uses underscore
package ldap_cache

import (
	ldap "github.com/netresearch/simple-ldap-go"
)

// SetUsersForTesting replaces the user cache with the given slice. Test
// code only — production code should use Refresh().
func (m *Manager) SetUsersForTesting(users []ldap.User) {
	m.Users.setAll(users)
}

// SetGroupsForTesting replaces the group cache. See SetUsersForTesting.
func (m *Manager) SetGroupsForTesting(groups []ldap.Group) {
	m.Groups.setAll(groups)
}

// SetComputersForTesting replaces the computer cache. See SetUsersForTesting.
func (m *Manager) SetComputersForTesting(computers []ldap.Computer) {
	m.Computers.setAll(computers)
}
