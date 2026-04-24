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
}
