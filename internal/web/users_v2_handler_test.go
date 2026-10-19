// internal/web/users_v2_handler_test.go — unit tests for the V2 user
// list/detail handlers and their filter helpers. Uses the directory
// fixture from groups_v2_handler_test.go.
package web

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/ldap_cache/cachetest"
)

const disableButton = `action="/users/bulk?action=disable"`

func userWithLastLogon(dn string, lastLogon int64) ldap.User {
	u := cachetest.NewUserWithDN(dn, dn, dn, true, nil)
	u.LastLogon = lastLogon

	return u
}

func TestFilterUsersByLastLogon(t *testing.T) {
	now := time.Now()
	users := []ldap.User{
		userWithLastLogon("cn=hour,dc=x", now.Add(-time.Hour).Unix()),
		userWithLastLogon("cn=3days,dc=x", now.Add(-3*24*time.Hour).Unix()),
		userWithLastLogon("cn=20days,dc=x", now.Add(-20*24*time.Hour).Unix()),
		userWithLastLogon("cn=90days,dc=x", now.Add(-90*24*time.Hour).Unix()),
		userWithLastLogon("cn=never,dc=x", 0),
	}

	tests := []struct {
		window string
		want   []string
	}{
		{"24h", []string{"cn=hour,dc=x"}},
		{"7d", []string{"cn=hour,dc=x", "cn=3days,dc=x"}},
		{"30d", []string{"cn=hour,dc=x", "cn=3days,dc=x", "cn=20days,dc=x"}},
		{"never", []string{"cn=never,dc=x"}},
		{"", userDNs(users)},
		{"bogus", userDNs(users)},
	}

	for _, tc := range tests {
		t.Run("window="+tc.window, func(t *testing.T) {
			assert.Equal(t, tc.want, userDNs(filterUsersByLastLogon(users, tc.window)))
		})
	}
}

func TestFilterUsersByOU(t *testing.T) {
	users := fixtureUsers()

	assert.Equal(t, []string{fxAliceDN, fxCarolDN}, userDNs(filterUsersByOU(users, "ou=people")))
	assert.Equal(t, []string{fxBobDN}, userDNs(filterUsersByOU(users, "ou=staff")))
	assert.Empty(t, filterUsersByOU(users, "ou=nowhere"))
	assert.Len(t, filterUsersByOU(users, ""), len(users), "empty OU is a no-op")
}

func TestFilterUsersByMemberOf(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedDirectoryFixture(t, app)

	users := fixtureUsers()

	assert.Equal(t, []string{fxAliceDN}, userDNs(filterUsersByMemberOf(users, fxMidDN, app.ldapCache)))
	assert.Equal(t, []string{fxBobDN}, userDNs(filterUsersByMemberOf(users, fxLeafDN, app.ldapCache)))
	assert.Empty(t, filterUsersByMemberOf(users, fxOtherDN, app.ldapCache), "known group without members")
	assert.Len(t, filterUsersByMemberOf(users, "cn=ghost,dc=test,dc=local", app.ldapCache), len(users),
		"unknown group is a no-op rather than an empty list")
	assert.Len(t, filterUsersByMemberOf(users, "", app.ldapCache), len(users))
	assert.Len(t, filterUsersByMemberOf(users, fxMidDN, nil), len(users), "no cache is a no-op")
}

func TestLookupGroupCN(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedDirectoryFixture(t, app)

	assert.Equal(t, "grp-leaf", lookupGroupCN(fxLeafDN, app.ldapCache))
	assert.Empty(t, lookupGroupCN("cn=ghost,dc=test,dc=local", app.ldapCache))
	assert.Empty(t, lookupGroupCN("", app.ldapCache))
	assert.Empty(t, lookupGroupCN(fxLeafDN, nil))
}

func TestSortUsersByCN_CaseInsensitive(t *testing.T) {
	users := []ldap.User{
		cachetest.NewUserWithDN("cn=b,dc=x", "bravo", "b", true, nil),
		cachetest.NewUserWithDN("cn=a,dc=x", "Alpha", "a", true, nil),
		cachetest.NewUserWithDN("cn=c,dc=x", "charlie", "c", true, nil),
	}
	sortUsersByCN(users)

	assert.Equal(t, []string{"cn=a,dc=x", "cn=b,dc=x", "cn=c,dc=x"}, userDNs(users))
}

func TestBuildOUPivotHref(t *testing.T) {
	assert.Empty(t, buildOUPivotHref(""))
	assert.Equal(t, "/users?ou=ou%3Dpeople", buildOUPivotHref("ou=people"))
}

func TestAdminUserDNs(t *testing.T) {
	app, _ := setupFullTestApp(t)

	admin := cachetest.NewUserWithDN(fxAliceDN, "usr-alice", "alice", false, nil)
	admin.AdminCount = true
	cachetest.Seed(app.ldapCache, []ldap.User{admin, fixtureUsers()[1]}, nil, nil)

	got := adminUserDNs(app.ldapCache)

	assert.Equal(t, map[string]struct{}{fxAliceDN: {}}, got, "disabled admins still count; non-admins do not")
	assert.Nil(t, adminUserDNs(nil))
}

func TestBuildUserDrawerVM(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedDirectoryFixture(t, app)

	const viewer = "cn=john.doe,ou=users,dc=test,dc=com"

	vm, ok := app.buildUserDrawerVM(fxAliceDN, viewer)
	require.True(t, ok)

	assert.Equal(t, "usr-alice", vm.User.CN())
	assert.Equal(t, []string{fxMidDN}, groupDNs(vm.User.Groups))
	assert.Equal(t, []string{fxLeafDN, fxOtherDN, fxRootDN}, groupDNs(vm.UnassignedGroups),
		"groups the user is not in, sorted by CN")
	assert.Equal(t, "ou=people", vm.OUName)
	assert.Equal(t, "/users?ou=ou%3Dpeople", vm.OUPivotHref)
	assert.False(t, vm.IsAD)
	assert.False(t, vm.Pinned)

	require.NoError(t, app.pinnedStore.Add(viewer, fxAliceDN))
	vm, ok = app.buildUserDrawerVM(fxAliceDN, viewer)
	require.True(t, ok)
	assert.True(t, vm.Pinned)

	_, ok = app.buildUserDrawerVM("cn=ghost,ou=people,dc=test,dc=local", viewer)
	assert.False(t, ok)
}

func TestPopulateGroupsForUser_NilCache(t *testing.T) {
	app := &App{}
	u := fixtureUsers()[0]

	full := app.populateGroupsForUser(&u)

	require.NotNil(t, full)
	assert.Equal(t, fxAliceDN, full.DN())
	assert.Empty(t, full.Groups)
}

func TestHandleUserV2_Detail(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	path := "/users/" + url.PathEscape(fxAliceDN)

	t.Run("full page", func(t *testing.T) {
		status, body := getBody(t, app, path, cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `id="main-content"`)
		assert.Contains(t, body, "usr-alice")
		assert.Contains(t, body, "grp-mid", "group membership is listed")
		assert.NotContains(t, body, disableButton, "no disable action outside Active Directory")
	})

	t.Run("htmx drawer fragment", func(t *testing.T) {
		status, body := getBody(t, app, path+"?fragment=drawer", cookies, map[string]string{"HX-Request": "true"})

		require.Equal(t, http.StatusOK, status)
		assert.NotContains(t, body, `id="main-content"`)
		assert.NotContains(t, body, "<html")
		assert.Contains(t, body, `data-recent-dn="`+fxAliceDN+`"`)
	})

	t.Run("fragment needs both the query and the htmx header", func(t *testing.T) {
		_, body := getBody(t, app, path+"?fragment=drawer", cookies, nil)
		assert.Contains(t, body, `id="main-content"`, "plain reload of a drawer URL gets the full page")

		_, body = getBody(t, app, path, cookies, map[string]string{"HX-Request": "true"})
		assert.Contains(t, body, `id="main-content"`, "htmx request without fragment=drawer gets the full page")
	})

	t.Run("unknown DN is 404", func(t *testing.T) {
		status, _ := getBody(t, app, "/users/"+url.PathEscape("cn=ghost,ou=people,dc=test,dc=local"), cookies, nil)
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("disable action only for enabled AD users", func(t *testing.T) {
		app.ldapConfig.IsActiveDirectory = true
		defer func() { app.ldapConfig.IsActiveDirectory = false }()

		_, body := getBody(t, app, path, cookies, nil)
		assert.Contains(t, body, disableButton)

		_, body = getBody(t, app, "/users/"+url.PathEscape(fxCarolDN), cookies, nil)
		assert.NotContains(t, body, disableButton, "already-disabled user offers no disable action")
	})
}

func TestHandleUsersV2_Filters(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	t.Run("memberOf filter", func(t *testing.T) {
		status, body := getBody(t, app, "/users?view=list&memberOf="+url.QueryEscape(fxLeafDN), cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "usr-bob")
		assert.NotContains(t, body, "usr-alice")
		assert.Contains(t, body, "grp-leaf", "the filter chip names the group by CN")
	})

	t.Run("OU filter", func(t *testing.T) {
		status, body := getBody(t, app, "/users?view=list&ou="+url.QueryEscape("ou=staff"), cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "usr-bob")
		assert.NotContains(t, body, "usr-alice")
	})

	t.Run("disabled users hidden unless requested", func(t *testing.T) {
		_, body := getBody(t, app, "/users?view=list", cookies, nil)
		assert.NotContains(t, body, "usr-carol")

		_, body = getBody(t, app, "/users?view=list&show-disabled=1", cookies, nil)
		assert.Contains(t, body, "usr-carol")
	})
}
