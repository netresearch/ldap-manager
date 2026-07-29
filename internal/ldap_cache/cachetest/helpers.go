// Package cachetest provides exported test helpers for seeding the
// ldap_cache.Manager from external test packages. DO NOT import from
// production code — this exists for tests only.
package cachetest

import (
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// NewUserWithDN builds a ldap.User with dn/cn populated. Use only in tests.
func NewUserWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.User {
	return ldap.User{
		Object:         ldap.NewObject(cn, dn),
		SAMAccountName: sam,
		Enabled:        enabled,
		Groups:         groups,
	}
}

// NewGroupWithDN builds a ldap.Group with dn/cn populated.
func NewGroupWithDN(dn, cn string, members []string) ldap.Group {
	return ldap.Group{
		Object:  ldap.NewObject(cn, dn),
		Members: members,
	}
}

// NewComputerWithDN builds a ldap.Computer with dn/cn populated.
func NewComputerWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.Computer {
	return ldap.Computer{
		Object:         ldap.NewObject(cn, dn),
		SAMAccountName: sam,
		Enabled:        enabled,
		Groups:         groups,
	}
}

// Seed replaces all three caches in one call. Pass nil for any kind you
// don't need to set.
func Seed(m *ldap_cache.Manager, users []ldap.User, groups []ldap.Group, computers []ldap.Computer) {
	if users != nil {
		m.SetUsersForTesting(users)
	}
	if groups != nil {
		m.SetGroupsForTesting(groups)
	}
	if computers != nil {
		m.SetComputersForTesting(computers)
	}
}
