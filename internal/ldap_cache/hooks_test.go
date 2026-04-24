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

// TestManagerOnDeleteComputer_ScrubGroupMembership verifies that
// deleting a computer also removes its DN from every cached group's
// Members list. Computers can be group members in AD (machine accounts
// in security groups), and the UI derives computer group memberships
// by scanning group Members, so the scrub is necessary for the group
// member count to stay accurate after the delete.
func TestManagerOnDeleteComputer_ScrubGroupMembership(t *testing.T) {
	const computerDN = "cn=workstation-01,ou=computers,dc=example,dc=com"

	mockClient := &mockLDAPClient{
		computers: []ldap.Computer{
			NewMockComputer(computerDN, "workstation-01$", true, nil),
		},
		groups: []ldap.Group{
			{Members: []string{computerDN, "cn=other,dc=example,dc=com"}},
		},
	}
	manager := New(mockClient)
	if err := manager.RefreshComputers(); err != nil {
		t.Fatalf("refresh computers: %v", err)
	}
	if err := manager.RefreshGroups(); err != nil {
		t.Fatalf("refresh groups: %v", err)
	}

	manager.OnDeleteComputer(computerDN)

	groups := manager.Groups.Get()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	for _, m := range groups[0].Members {
		if m == computerDN {
			t.Errorf("group still references deleted computer %q", computerDN)
		}
	}
	if want := "cn=other,dc=example,dc=com"; len(groups[0].Members) != 1 || groups[0].Members[0] != want {
		t.Errorf("expected remaining member %q, got %v", want, groups[0].Members)
	}
}

// testMutable is a cacheable with both a DN and a mutable bool, used
// to exercise Cache.updateByDN under a controlled DN surface. The
// approach mirrors testEntity above; simple-ldap-go's unexported dn
// field makes it impractical to seed real DNs on ldap.User /
// ldap.Computer from outside that package.
type testMutable struct {
	dn      string
	enabled bool
}

func (m testMutable) DN() string { return m.dn }

func TestCacheUpdateByDN(t *testing.T) {
	t.Run("mutates the matching entry and returns true", func(t *testing.T) {
		c := NewCached[testMutable]()
		c.setAll([]testMutable{
			{dn: "cn=alice,dc=x", enabled: true},
			{dn: "cn=bob,dc=x", enabled: true},
		})

		ok := c.updateByDN("cn=bob,dc=x", func(m *testMutable) { m.enabled = false })
		if !ok {
			t.Fatal("updateByDN returned false for an existing DN")
		}

		got, found := c.FindByDN("cn=bob,dc=x")
		if !found {
			t.Fatal("bob missing after updateByDN")
		}
		if got.enabled {
			t.Error("bob still enabled after updateByDN")
		}

		alice, _ := c.FindByDN("cn=alice,dc=x")
		if !alice.enabled {
			t.Error("alice got mutated — update leaked past the DN filter")
		}
	})

	t.Run("returns false and does nothing for an unknown DN", func(t *testing.T) {
		c := NewCached[testMutable]()
		c.setAll([]testMutable{
			{dn: "cn=alice,dc=x", enabled: true},
		})

		called := false
		ok := c.updateByDN("cn=ghost,dc=x", func(_ *testMutable) { called = true })
		if ok {
			t.Error("updateByDN returned true for a missing DN")
		}
		if called {
			t.Error("fn was invoked for a missing DN")
		}
	})
}

// TestManagerOnDisable_NoOpOnAbsentDN verifies that calling
// OnDisableUser / OnDisableComputer with a DN that isn't cached is a
// safe no-op. The happy-path ("a known DN gets Enabled flipped") is
// covered structurally by TestCacheUpdateByDN above — seeding an
// ldap.User with a real DN requires reaching into simple-ldap-go's
// unexported Object fields.
func TestManagerOnDisable_NoOpOnAbsentDN(t *testing.T) {
	mockClient := &mockLDAPClient{
		users:     createMockUsers(),
		computers: createMockComputers(),
	}
	manager := New(mockClient)
	manager.Refresh()

	usersBefore := manager.Users.Count()
	computersBefore := manager.Computers.Count()

	manager.OnDisableUser("cn=ghost-user,dc=example,dc=com")
	manager.OnDisableComputer("cn=ghost-computer,dc=example,dc=com")

	if got := manager.Users.Count(); got != usersBefore {
		t.Errorf("user count changed: before=%d after=%d", usersBefore, got)
	}
	if got := manager.Computers.Count(); got != computersBefore {
		t.Errorf("computer count changed: before=%d after=%d", computersBefore, got)
	}
}
