// internal/web/table_sort_test.go — unit coverage for the Table-view
// server-side sort helpers. Stays in the web package so the test sees
// the unexported sortUsersTable / sortGroupsTable / sortComputersTable
// directly.
package web

import (
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/ldap_cache/cachetest"
)

func TestNormaliseSortDir(t *testing.T) {
	require.Equal(t, "asc", normaliseSortDir(""))
	require.Equal(t, "asc", normaliseSortDir("foo"))
	require.Equal(t, "asc", normaliseSortDir("ASC")) // case-sensitive: only literal "desc" wins
	require.Equal(t, "desc", normaliseSortDir("desc"))
}

func TestSortUsersTable_ByCNAscDesc(t *testing.T) {
	users := []ldap.User{
		cachetest.NewUserWithDN("cn=charlie,dc=ex,dc=com", "charlie", "csam", true, nil),
		cachetest.NewUserWithDN("cn=alice,dc=ex,dc=com", "alice", "asam", true, nil),
		cachetest.NewUserWithDN("cn=bob,dc=ex,dc=com", "bob", "bsam", false, nil),
	}

	sortUsersTable(users, "cn", "asc")
	require.Equal(t, "alice", users[0].CN())
	require.Equal(t, "bob", users[1].CN())
	require.Equal(t, "charlie", users[2].CN())

	sortUsersTable(users, "cn", "desc")
	require.Equal(t, "charlie", users[0].CN())
	require.Equal(t, "bob", users[1].CN())
	require.Equal(t, "alice", users[2].CN())
}

func TestSortUsersTable_ByStatusEnabledFirst(t *testing.T) {
	users := []ldap.User{
		cachetest.NewUserWithDN("cn=disabled,dc=ex,dc=com", "disabled", "d", false, nil),
		cachetest.NewUserWithDN("cn=enabled,dc=ex,dc=com", "enabled", "e", true, nil),
	}
	sortUsersTable(users, "status", "asc")
	// "Enabled" (key "a") < "Disabled" (key "b") so enabled rows come
	// first under asc — the documented contract for the status sort.
	require.True(t, users[0].Enabled, "first row should be enabled under asc status sort")
	require.False(t, users[1].Enabled)
}

func TestSortUsersTable_UnknownKeyFallsBackToCN(t *testing.T) {
	users := []ldap.User{
		cachetest.NewUserWithDN("cn=zed,dc=ex,dc=com", "zed", "z", true, nil),
		cachetest.NewUserWithDN("cn=ann,dc=ex,dc=com", "ann", "a", true, nil),
	}
	sortUsersTable(users, "lol-not-a-column", "asc")
	require.Equal(t, "ann", users[0].CN(), "unknown sort key should fall back to CN")
}

func TestSortGroupsTable_ByMembersDesc(t *testing.T) {
	groups := []ldap.Group{
		cachetest.NewGroupWithDN("cn=small,dc=ex,dc=com", "small", []string{"a"}),
		cachetest.NewGroupWithDN("cn=big,dc=ex,dc=com", "big", []string{"a", "b", "c"}),
		cachetest.NewGroupWithDN("cn=mid,dc=ex,dc=com", "mid", []string{"a", "b"}),
	}
	sortGroupsTable(groups, "members", "desc")
	require.Equal(t, "big", groups[0].CN())
	require.Equal(t, "mid", groups[1].CN())
	require.Equal(t, "small", groups[2].CN())
}

func TestSortComputersTable_BySAM(t *testing.T) {
	computers := []ldap.Computer{
		cachetest.NewComputerWithDN("cn=ws03,dc=ex,dc=com", "ws03", "ws03$", true, nil),
		cachetest.NewComputerWithDN("cn=ws01,dc=ex,dc=com", "ws01", "ws01$", true, nil),
	}
	sortComputersTable(computers, "sam", "asc")
	require.Equal(t, "ws01$", computers[0].SAMAccountName)
	require.Equal(t, "ws03$", computers[1].SAMAccountName)
}
