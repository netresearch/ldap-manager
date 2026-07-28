// Package cachetest provides exported test helpers for seeding the
// ldap_cache.Manager from external test packages. It uses reflection +
// unsafe to set the unexported dn/cn fields on simple-ldap-go's Object
// type. DO NOT import from production code — this exists for tests only.
package cachetest

import (
	"reflect"
	"unsafe"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// NewUserWithDN builds a ldap.User with the unexported dn/cn fields set
// via reflection. Use only in tests.
func NewUserWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.User {
	u := ldap.User{SAMAccountName: sam, Enabled: enabled, Groups: groups}
	setObjectFields(reflect.ValueOf(&u).Elem().FieldByName("Object"), dn, cn)

	return u
}

// NewGroupWithDN builds a ldap.Group with unexported dn/cn populated.
func NewGroupWithDN(dn, cn string, members []string) ldap.Group {
	g := ldap.Group{Members: members}
	setObjectFields(reflect.ValueOf(&g).Elem().FieldByName("Object"), dn, cn)

	return g
}

// NewComputerWithDN builds a ldap.Computer with unexported dn/cn populated.
func NewComputerWithDN(dn, cn, sam string, enabled bool, groups []string) ldap.Computer {
	c := ldap.Computer{SAMAccountName: sam, Enabled: enabled, Groups: groups}
	setObjectFields(reflect.ValueOf(&c).Elem().FieldByName("Object"), dn, cn)

	return c
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

func setObjectFields(obj reflect.Value, dn, cn string) {
	dnField := obj.FieldByName("dn")
	cnField := obj.FieldByName("cn")
	// Test-only writers for unexported simple-ldap-go fields. This package is
	// imported by the tests of two other packages (internal/web and
	// internal/ldap_cache), and Go cannot share helpers declared in _test.go
	// files across package boundaries — so it has to be an ordinary package and
	// is compiled into the binary, unreachable from any production call path.
	// The unsafe write exists because simple-ldap-go sets Object.dn/cn only in
	// objectFromEntry when decoding a directory response and exposes no
	// constructor taking them; see netresearch/simple-ldap-go#191.
	dnPtr := unsafe.Pointer(dnField.UnsafeAddr()) // #nosec G103 -- test fixture writer, see comment above
	cnPtr := unsafe.Pointer(cnField.UnsafeAddr()) // #nosec G103 -- test fixture writer, see comment above
	reflect.NewAt(dnField.Type(), dnPtr).Elem().SetString(dn)
	reflect.NewAt(cnField.Type(), cnPtr).Elem().SetString(cn)
}
