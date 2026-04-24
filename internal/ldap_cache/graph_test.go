package ldap_cache

import (
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
)

func graphFixture(t *testing.T) *Manager {
	t.Helper()
	m := &mockLDAPClient{}
	manager := New(m)
	// Seed directly, bypassing the mockLDAPClient path.
	manager.Users.setAll([]ldap.User{
		newUserWithDN("cn=bob,ou=Engineering,dc=ex,dc=com", "bob", "bob", true, []string{
			"cn=admins,ou=Groups,dc=ex,dc=com", "cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
		newUserWithDN("cn=carol,ou=Sales,dc=ex,dc=com", "carol", "carol", true, []string{
			"cn=admins,ou=Groups,dc=ex,dc=com",
		}),
		newUserWithDN("cn=dave,ou=Engineering,dc=ex,dc=com", "dave", "dave", true, []string{
			"cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
		newUserWithDN("cn=alice,ou=Engineering,dc=ex,dc=com", "alice", "alice", true, []string{
			"cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
	})
	manager.Groups.setAll([]ldap.Group{
		newGroupWithDN("cn=admins,ou=Groups,dc=ex,dc=com", "admins", []string{
			"cn=bob,ou=Engineering,dc=ex,dc=com", "cn=carol,ou=Sales,dc=ex,dc=com",
		}),
		newGroupWithDN("cn=engineers,ou=Groups,dc=ex,dc=com", "engineers", []string{
			"cn=bob,ou=Engineering,dc=ex,dc=com", "cn=dave,ou=Engineering,dc=ex,dc=com", "cn=alice,ou=Engineering,dc=ex,dc=com",
		}),
		newGroupWithDN("cn=all-staff,ou=Groups,dc=ex,dc=com", "all-staff", []string{
			"cn=engineers,ou=Groups,dc=ex,dc=com",
		}),
	})

	return manager
}

func TestBuildGraph_UserFocus_Depth1(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("cn=bob,ou=Engineering,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	if data.Focus != "cn=bob,ou=Engineering,dc=ex,dc=com" {
		t.Errorf("focus: %q", data.Focus)
	}
	if data.Depth != 1 {
		t.Errorf("depth: %d", data.Depth)
	}
	// Expect: bob (ring 0) + 2 groups (ring 1) + 1 OU (ring 1) = 4 nodes
	if got := len(data.Nodes); got != 4 {
		t.Errorf("node count at depth 1: got %d, want 4", got)
	}
	// Expect edges: bob→admins (memberOf), bob→engineers (memberOf),
	// ou=Engineering→bob (contains)
	if got := len(data.Edges); got != 3 {
		t.Errorf("edge count at depth 1: got %d, want 3", got)
	}

	// Edge kind correctness — catches a regression where direction is
	// right but kind is swapped.
	bobDN := "cn=bob,ou=Engineering,dc=ex,dc=com"
	ouDN := "ou=Engineering,dc=ex,dc=com"
	for _, e := range data.Edges {
		switch {
		case e.Source == bobDN && e.Kind != EdgeMemberOf:
			t.Errorf("edge from bob expected EdgeMemberOf, got %q: %+v", e.Kind, e)
		case e.Source == ouDN && e.Kind != EdgeContains:
			t.Errorf("edge from OU expected EdgeContains, got %q: %+v", e.Kind, e)
		}
	}

	// No self-loops — a user/group/OU shouldn't appear as both source
	// and target of the same edge.
	for _, e := range data.Edges {
		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}
}

func TestBuildGraph_GroupFocus_Depth1(t *testing.T) {
	m := graphFixture(t)
	data, err := m.BuildGraph("cn=engineers,ou=Groups,dc=ex,dc=com", 1)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}
	// engineers has 3 user members + is a member of all-staff
	// Expect: engineers (ring 0) + 3 users (ring 1) + all-staff (ring 1) = 5 nodes
	if got := len(data.Nodes); got != 5 {
		t.Errorf("node count: got %d, want 5", got)
	}

	// Expect edges: bob→engineers, dave→engineers, alice→engineers
	// (memberOf) + engineers→all-staff (memberOf) = 4 edges.
	if got := len(data.Edges); got != 4 {
		t.Errorf("edge count: got %d, want 4", got)
	}

	// Edge kind + direction correctness on the parent edge — the
	// non-obvious branch of the group-focus builder.
	engineersDN := "cn=engineers,ou=Groups,dc=ex,dc=com"
	allStaffDN := "cn=all-staff,ou=Groups,dc=ex,dc=com"

	var foundParent bool

	for _, e := range data.Edges {
		if e.Source == engineersDN && e.Target == allStaffDN {
			if e.Kind != EdgeMemberOf {
				t.Errorf("engineers→all-staff edge: kind %q, want %q", e.Kind, EdgeMemberOf)
			}

			foundParent = true
		}

		if e.Source == e.Target {
			t.Errorf("self-loop edge: %+v", e)
		}
	}

	if !foundParent {
		t.Errorf("missing engineers→all-staff memberOf edge")
	}
}
