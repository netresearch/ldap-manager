// internal/ldap_cache/hooks_test.go
package ldap_cache

import (
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
)

// testEntity is a minimal cacheable used to exercise Cache.remove under
// a controlled DN surface. The ldap.User / ldap.Computer structs embed
// simple-ldap-go's Object struct with unexported dn/cn fields, so there
// is no public way to seed a real DN from outside that package. An
// in-test type sidesteps that without resorting to unsafe pointer
// tricks, and lets us assert DN-indexed behaviour properly.
type testEntity struct {
	dn string
	v  string
}

func (e testEntity) DN() string { return e.dn }

func TestCacheRemove(t *testing.T) {
	seed := []testEntity{
		{dn: "cn=alice,dc=x", v: "A"},
		{dn: "cn=bob,dc=x", v: "B"},
		{dn: "cn=carol,dc=x", v: "C"},
	}

	t.Run("removes the matching entry", func(t *testing.T) {
		c := NewCached[testEntity]()
		c.setAll(append([]testEntity(nil), seed...))

		c.remove("cn=bob,dc=x")

		if got := c.Count(); got != 2 {
			t.Fatalf("expected 2 items after remove, got %d", got)
		}
		if _, ok := c.FindByDN("cn=bob,dc=x"); ok {
			t.Error("bob still indexed after remove")
		}
		if _, ok := c.FindByDN("cn=alice,dc=x"); !ok {
			t.Error("alice missing after remove")
		}
		if _, ok := c.FindByDN("cn=carol,dc=x"); !ok {
			t.Error("carol missing after remove")
		}
	})

	t.Run("no-op when DN is not present", func(t *testing.T) {
		c := NewCached[testEntity]()
		c.setAll(append([]testEntity(nil), seed...))

		c.remove("cn=does-not-exist,dc=x")

		if got := c.Count(); got != 3 {
			t.Fatalf("expected 3 items, got %d", got)
		}
	})

	t.Run("tolerates empty cache", func(t *testing.T) {
		c := NewCached[testEntity]()
		c.remove("cn=whatever")

		if got := c.Count(); got != 0 {
			t.Fatalf("expected 0 items, got %d", got)
		}
	})
}

// TestManagerOnDeleteUser_ScrubGroupMembership relies on the fact that
// every group's Members is a []string of user DNs — so even though the
// mock users synthesise DN() == "", we can assert that a user DN we
// seed into a group's member list gets scrubbed by OnDeleteUser.
func TestManagerOnDeleteUser_ScrubGroupMembership(t *testing.T) {
	const userDN = "cn=john.doe,ou=users,dc=example,dc=com"

	mockClient := &mockLDAPClient{
		users: []ldap.User{
			NewMockUser(userDN, "john.doe", true, nil),
		},
		groups: []ldap.Group{
			{Members: []string{userDN, "cn=other,dc=example,dc=com"}},
		},
	}
	manager := New(mockClient)

	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("refresh users: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("refresh groups: %v", err)
	}

	manager.OnDeleteUser(userDN)

	groups := manager.Groups.Get()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	for _, m := range groups[0].Members {
		if m == userDN {
			t.Errorf("group still references deleted user %q", userDN)
		}
	}
	if want := "cn=other,dc=example,dc=com"; len(groups[0].Members) != 1 || groups[0].Members[0] != want {
		t.Errorf("expected remaining member %q, got %v", want, groups[0].Members)
	}
}

// TestManagerOnDeleteGroup_ScrubMemberOf verifies that deleting a group
// removes it from every user's Groups slice and every computer's Groups
// slice.
func TestManagerOnDeleteGroup_ScrubMemberOf(t *testing.T) {
	const groupDN = "cn=old-group,ou=groups,dc=example,dc=com"

	mockClient := &mockLDAPClient{
		users: []ldap.User{
			NewMockUser("cn=john.doe,ou=users,dc=example,dc=com", "john.doe", true,
				[]string{groupDN, "cn=keep,dc=example,dc=com"}),
		},
		groups: []ldap.Group{
			{Members: nil},
		},
		computers: []ldap.Computer{
			NewMockComputer("cn=workstation-01,ou=computers,dc=example,dc=com", "workstation-01$", true,
				[]string{groupDN, "cn=keep,dc=example,dc=com"}),
		},
	}
	manager := New(mockClient)
	manager.Refresh()

	manager.OnDeleteGroup(groupDN)

	users := manager.Users.Get()
	for _, u := range users {
		for _, g := range u.Groups {
			if g == groupDN {
				t.Errorf("user %q still references deleted group %q", u.SAMAccountName, groupDN)
			}
		}
	}

	computers := manager.Computers.Get()
	for _, c := range computers {
		for _, g := range c.Groups {
			if g == groupDN {
				t.Errorf("computer %q still references deleted group %q", c.SAMAccountName, groupDN)
			}
		}
	}
}

// TestManagerOnDeleteComputer_NoOpOnAbsent confirms that deleting a
// computer DN that isn't cached is a safe no-op (the hook is called
// before bulk_handlers has a chance to check cache presence).
func TestManagerOnDeleteComputer_NoOpOnAbsent(t *testing.T) {
	mockClient := &mockLDAPClient{
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("refresh computers: %v", err)
	}

	before := manager.Computers.Count()
	manager.OnDeleteComputer("cn=ghost,ou=computers,dc=example,dc=com")

	if after := manager.Computers.Count(); after != before {
		t.Errorf("computer count changed: before=%d after=%d", before, after)
	}
}

// TestManagerOnDisableUser_FlipsEnabled uses the `update` code path
// which iterates all users. Because mock users' DN() returns "", we
// exploit the empty-string equality to flip every cached user's
// Enabled bit and assert the write-back took effect. This is the same
// pragmatic approach the existing OnAdd/OnRemove tests use.
func TestManagerOnDisableUser_FlipsEnabled(t *testing.T) {
	mockClient := &mockLDAPClient{
		users: []ldap.User{
			NewMockUser("cn=john.doe,…", "john.doe", true, nil),
			NewMockUser("cn=jane.smith,…", "jane.smith", true, nil),
		},
	}
	manager := New(mockClient)
	if err := manager.RefreshUsers(); err != nil {
		t.Fatalf("refresh users: %v", err)
	}

	manager.OnDisableUser("") // mock DN()=="" matches all

	for _, u := range manager.Users.Get() {
		if u.Enabled {
			t.Errorf("user %q still enabled after OnDisableUser", u.SAMAccountName)
		}
	}
}

// TestManagerOnDisableComputer_FlipsEnabled mirrors
// TestManagerOnDisableUser_FlipsEnabled for machine accounts.
func TestManagerOnDisableComputer_FlipsEnabled(t *testing.T) {
	mockClient := &mockLDAPClient{
		computers: []ldap.Computer{
			NewMockComputer("cn=ws01,…", "ws01$", true, nil),
		},
	}
	manager := New(mockClient)
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("refresh computers: %v", err)
	}

	manager.OnDisableComputer("")

	for _, cmp := range manager.Computers.Get() {
		if cmp.Enabled {
			t.Errorf("computer %q still enabled after OnDisableComputer", cmp.SAMAccountName)
		}
	}
}
