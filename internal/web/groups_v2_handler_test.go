// internal/web/groups_v2_handler_test.go — unit tests for the V2 group
// list/detail handlers and the nested-group helpers behind the drawer.
package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/ldap_cache/cachetest"
)

// Directory fixture shared by the group and user handler tests:
//
//	grp-root ── member ──> grp-mid ── member ──> grp-leaf ── member ──> usr-bob
//	                          └────── member ──> usr-alice
//	grp-other (no members, no parents)
//
// usr-carol is disabled and belongs to no group.
const (
	fxRootDN  = "cn=grp-root,ou=groups,dc=test,dc=local"
	fxMidDN   = "cn=grp-mid,ou=groups,dc=test,dc=local"
	fxLeafDN  = "cn=grp-leaf,ou=teams,dc=test,dc=local"
	fxOtherDN = "cn=grp-other,ou=groups,dc=test,dc=local"
	fxAliceDN = "cn=usr-alice,ou=people,dc=test,dc=local"
	fxBobDN   = "cn=usr-bob,ou=staff,dc=test,dc=local"
	fxCarolDN = "cn=usr-carol,ou=people,dc=test,dc=local"
)

func fixtureGroup(dn, cn string, members, memberOf []string) ldap.Group {
	g := cachetest.NewGroupWithDN(dn, cn, members)
	g.MemberOf = memberOf

	return g
}

func fixtureGroups() []ldap.Group {
	return []ldap.Group{
		fixtureGroup(fxRootDN, "grp-root", []string{fxMidDN}, nil),
		fixtureGroup(fxMidDN, "grp-mid", []string{fxLeafDN, fxAliceDN}, []string{fxRootDN}),
		fixtureGroup(fxLeafDN, "grp-leaf", []string{fxBobDN}, []string{fxMidDN}),
		fixtureGroup(fxOtherDN, "grp-other", nil, nil),
	}
}

func fixtureUsers() []ldap.User {
	return []ldap.User{
		cachetest.NewUserWithDN(fxAliceDN, "usr-alice", "alice", true, []string{fxMidDN}),
		cachetest.NewUserWithDN(fxBobDN, "usr-bob", "bob", true, []string{fxLeafDN}),
		cachetest.NewUserWithDN(fxCarolDN, "usr-carol", "carol", false, nil),
	}
}

// seedDirectoryFixture replaces the full-test-app cache with the fixture.
func seedDirectoryFixture(t *testing.T, app *App) {
	t.Helper()
	cachetest.Seed(app.ldapCache, fixtureUsers(), fixtureGroups(), []ldap.Computer{})
}

func groupDNs(groups []ldap.Group) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.DN())
	}

	return out
}

func userDNs(users []ldap.User) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.DN())
	}

	return out
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)

	return out
}

// getBody issues an authenticated GET with optional extra headers and
// returns status and body.
func getBody(t *testing.T, app *App, path string, cookies []*http.Cookie, headers map[string]string) (int, string) {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

func TestResolveChildGroups(t *testing.T) {
	all := fixtureGroups()

	t.Run("only group members are children", func(t *testing.T) {
		mid := all[1]
		// grp-mid has a group member (grp-leaf) and a user member (usr-alice).
		assert.Equal(t, []string{fxLeafDN}, groupDNs(resolveChildGroups(&mid, all)))
	})

	t.Run("no members yields nil", func(t *testing.T) {
		other := all[3]
		assert.Nil(t, resolveChildGroups(&other, all))
	})

	t.Run("nil group yields nil", func(t *testing.T) {
		assert.Nil(t, resolveChildGroups(nil, all))
	})
}

func TestCollectAncestorDNs(t *testing.T) {
	all := fixtureGroups()

	t.Run("transitive parents", func(t *testing.T) {
		leaf := all[2]
		assert.Equal(t, sorted([]string{fxMidDN, fxRootDN}), sorted(collectAncestorDNs(&leaf, all)))
	})

	t.Run("top-level group has no ancestors", func(t *testing.T) {
		root := all[0]
		assert.Empty(t, collectAncestorDNs(&root, all))
	})

	t.Run("membership cycle terminates and excludes self", func(t *testing.T) {
		const aDN, bDN = "cn=a,ou=groups,dc=test,dc=local", "cn=b,ou=groups,dc=test,dc=local"
		cyc := []ldap.Group{
			fixtureGroup(aDN, "a", []string{bDN}, []string{bDN}),
			fixtureGroup(bDN, "b", []string{aDN}, []string{aDN}),
		}
		assert.Equal(t, []string{bDN}, collectAncestorDNs(&cyc[0], cyc))
	})

	t.Run("unknown parent DN is still reported", func(t *testing.T) {
		const orphanParent = "cn=elsewhere,dc=other"
		g := fixtureGroup("cn=x,ou=groups,dc=test,dc=local", "x", nil, []string{orphanParent})
		assert.Equal(t, []string{orphanParent}, collectAncestorDNs(&g, all))
	})
}

func TestCollectDescendantDNs(t *testing.T) {
	all := fixtureGroups()

	t.Run("transitive child groups, users ignored", func(t *testing.T) {
		root := all[0]
		assert.Equal(t, sorted([]string{fxMidDN, fxLeafDN}), sorted(collectDescendantDNs(&root, all)))
	})

	t.Run("leaf group has no descendants", func(t *testing.T) {
		leaf := all[2]
		assert.Empty(t, collectDescendantDNs(&leaf, all))
	})

	t.Run("membership cycle terminates and excludes self", func(t *testing.T) {
		const aDN, bDN = "cn=a,ou=groups,dc=test,dc=local", "cn=b,ou=groups,dc=test,dc=local"
		cyc := []ldap.Group{
			fixtureGroup(aDN, "a", []string{bDN}, nil),
			fixtureGroup(bDN, "b", []string{aDN}, nil),
		}
		assert.Equal(t, []string{bDN}, collectDescendantDNs(&cyc[0], cyc))
	})
}

func TestFilterAddableChildGroups(t *testing.T) {
	all := fixtureGroups()
	mid := all[1]

	got := filterAddableChildGroups(all, &mid, []ldap.Group{all[2]})

	// Excluded: self (mid), existing child (leaf), ancestor (root) — adding
	// root as a child of mid would create a cycle.
	assert.Equal(t, []string{fxOtherDN}, groupDNs(got))

	t.Run("leaf may not adopt any ancestor", func(t *testing.T) {
		leaf := all[2]
		got := filterAddableChildGroups(all, &leaf, nil)
		assert.Equal(t, []string{fxOtherDN}, groupDNs(got))
	})
}

func TestFilterAddableParentGroups(t *testing.T) {
	all := fixtureGroups()
	mid := all[1]

	got := filterAddableParentGroups(all, &mid, []ldap.Group{all[0]})

	// Excluded: self (mid), existing parent (root), descendant (leaf) —
	// making leaf a parent of mid would create a cycle.
	assert.Equal(t, []string{fxOtherDN}, groupDNs(got))

	t.Run("root may not join any descendant", func(t *testing.T) {
		root := all[0]
		got := filterAddableParentGroups(all, &root, nil)
		assert.Equal(t, []string{fxOtherDN}, groupDNs(got))
	})
}

func TestFilterGroupsByMember(t *testing.T) {
	all := fixtureGroups()

	assert.Equal(t, []string{fxMidDN}, groupDNs(filterGroupsByMember(all, fxAliceDN)))
	assert.Equal(t, []string{fxRootDN}, groupDNs(filterGroupsByMember(all, fxMidDN)))
	assert.Empty(t, filterGroupsByMember(all, fxCarolDN))
	assert.Len(t, filterGroupsByMember(all, ""), len(all), "empty member DN is a no-op")
}

func TestFilterGroupsByOU(t *testing.T) {
	all := fixtureGroups()

	assert.Equal(t, []string{fxLeafDN}, groupDNs(filterGroupsByOU(all, "ou=teams")))
	assert.Equal(t, []string{fxRootDN, fxMidDN, fxOtherDN}, groupDNs(filterGroupsByOU(all, "ou=groups")))
	assert.Empty(t, filterGroupsByOU(all, "ou=nowhere"))
	assert.Len(t, filterGroupsByOU(all, ""), len(all), "empty OU is a no-op")
}

func TestSortGroupsByCN_CaseInsensitive(t *testing.T) {
	groups := []ldap.Group{
		cachetest.NewGroupWithDN("cn=b,dc=x", "beta", nil),
		cachetest.NewGroupWithDN("cn=a,dc=x", "Alpha", nil),
		cachetest.NewGroupWithDN("cn=c,dc=x", "charlie", nil),
	}
	sortGroupsByCN(groups)

	assert.Equal(t, []string{"cn=a,dc=x", "cn=b,dc=x", "cn=c,dc=x"}, groupDNs(groups))
}

func TestBuildGroupOUPivotHref(t *testing.T) {
	assert.Empty(t, buildGroupOUPivotHref(""))
	assert.Equal(t, "/groups?ou=ou%3Dteams", buildGroupOUPivotHref("ou=teams"))
}

func TestLookupUserCN(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedDirectoryFixture(t, app)

	assert.Equal(t, "usr-alice", lookupUserCN(fxAliceDN, app.ldapCache))
	assert.Equal(t, "usr-carol", lookupUserCN(fxCarolDN, app.ldapCache), "disabled users resolve too")
	assert.Empty(t, lookupUserCN("cn=ghost,dc=test,dc=local", app.ldapCache))
	assert.Empty(t, lookupUserCN("", app.ldapCache))
	assert.Empty(t, lookupUserCN(fxAliceDN, nil))
}

func TestBuildGroupDrawerVM(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedDirectoryFixture(t, app)

	const viewer = "cn=john.doe,ou=users,dc=test,dc=com"

	vm, ok := app.buildGroupDrawerVM(fxMidDN, viewer)
	require.True(t, ok)

	assert.Equal(t, "grp-mid", vm.Group.CN())
	assert.Equal(t, []string{fxAliceDN}, userDNs(vm.Group.Members), "only user members are resolved")
	assert.Equal(t, []string{fxRootDN}, groupDNs(vm.Group.ParentGroups))
	assert.Equal(t, []string{fxLeafDN}, groupDNs(vm.ChildGroups))
	assert.Equal(t, []string{fxOtherDN}, groupDNs(vm.AddableChildren))
	assert.Equal(t, []string{fxOtherDN}, groupDNs(vm.AddableParents))
	assert.Equal(t, []string{fxBobDN, fxCarolDN}, userDNs(vm.UnassignedUsers), "non-members, sorted by CN")
	assert.Equal(t, "ou=groups", vm.OUName)
	assert.Equal(t, "/groups?ou=ou%3Dgroups", vm.OUPivotHref)
	assert.False(t, vm.Pinned)

	t.Run("pinned state comes from the viewer's store", func(t *testing.T) {
		require.NoError(t, app.pinnedStore.Add(viewer, fxMidDN))

		vm, ok := app.buildGroupDrawerVM(fxMidDN, viewer)
		require.True(t, ok)
		assert.True(t, vm.Pinned)

		vm, ok = app.buildGroupDrawerVM(fxMidDN, "cn=someone-else,dc=test,dc=com")
		require.True(t, ok)
		assert.False(t, vm.Pinned, "pins are per viewer")
	})

	t.Run("unknown DN is not found", func(t *testing.T) {
		_, ok := app.buildGroupDrawerVM("cn=ghost,ou=groups,dc=test,dc=local", viewer)
		assert.False(t, ok)
	})

	t.Run("cache-less app finds nothing", func(t *testing.T) {
		app.ldapCache = nil
		_, ok := app.buildGroupDrawerVM(fxMidDN, viewer)
		assert.False(t, ok)
	})
}

func TestPopulateMembersForGroup_NilCache(t *testing.T) {
	app := &App{}
	g := fixtureGroups()[1]

	full := app.populateMembersForGroup(&g)

	require.NotNil(t, full)
	assert.Equal(t, fxMidDN, full.DN())
	assert.Empty(t, full.Members)
	assert.Empty(t, full.ParentGroups)
}

func TestHandleGroupV2_Detail(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	path := "/groups/" + url.PathEscape(fxMidDN)

	t.Run("full page", func(t *testing.T) {
		status, body := getBody(t, app, path, cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `id="main-content"`, "full page wraps the drawer in the page layout")
		assert.Contains(t, body, "grp-mid")
		assert.Contains(t, body, "usr-alice", "member is listed")
		assert.Contains(t, body, "grp-root", "parent group is listed")
		assert.Contains(t, body, "grp-leaf", "child group is listed")
	})

	t.Run("htmx drawer fragment", func(t *testing.T) {
		status, body := getBody(t, app, path+"?fragment=drawer", cookies, map[string]string{"HX-Request": "true"})

		require.Equal(t, http.StatusOK, status)
		assert.NotContains(t, body, `id="main-content"`, "fragment has no page layout")
		assert.NotContains(t, body, "<html")
		assert.Contains(t, body, `data-recent-dn="`+fxMidDN+`"`)
		assert.Contains(t, body, "usr-alice")
	})

	t.Run("fragment needs both the query and the htmx header", func(t *testing.T) {
		status, body := getBody(t, app, path+"?fragment=drawer", cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `id="main-content"`, "plain reload of a drawer URL gets the full page")

		_, body = getBody(t, app, path, cookies, map[string]string{"HX-Request": "true"})
		assert.Contains(t, body, `id="main-content"`, "htmx request without fragment=drawer gets the full page")
	})

	t.Run("unknown DN is 404", func(t *testing.T) {
		status, _ := getBody(t, app, "/groups/"+url.PathEscape("cn=ghost,ou=groups,dc=test,dc=local"), cookies, nil)
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("unauthenticated request is redirected to login", func(t *testing.T) {
		resp := makeAuthRequest(t, app, path, nil)
		defer closeHTTPResponse(t, resp)
		assertHTTPRedirect(t, resp)
	})
}

func TestHandleGroupsV2_Filters(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	t.Run("member filter", func(t *testing.T) {
		status, body := getBody(t, app, "/groups?view=list&member="+url.QueryEscape(fxAliceDN), cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "grp-mid")
		assert.NotContains(t, body, "grp-other")
		assert.NotContains(t, body, "grp-leaf")
		assert.Contains(t, body, "usr-alice", "the filter chip names the member by CN")
	})

	t.Run("OU filter", func(t *testing.T) {
		status, body := getBody(t, app, "/groups?view=list&ou="+url.QueryEscape("ou=teams"), cookies, nil)

		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "grp-leaf")
		assert.NotContains(t, body, "grp-other")
		assert.NotContains(t, body, "grp-mid")
	})
}
