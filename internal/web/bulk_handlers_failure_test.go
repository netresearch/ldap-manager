// internal/web/bulk_handlers_failure_test.go — bulk actions whose LDAP
// write fails. The unit harness points the per-user client at an
// unreachable server (ldap_fixture_test.go), so every write returns an
// error: these tests pin what the handlers do then — the redirect, the
// flash summary, and that the cache is not optimistically mutated.
// Successful writes need a directory server and are exercised by the
// integration and e2e suites.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postBulk sends an authenticated bulk POST and returns the response.
func postBulk(t *testing.T, app *App, path string, form url.Values, referer string, cookies []*http.Cookie) *http.Response {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if referer != "" {
		req.Header.Set(fiber.HeaderReferer, referer)
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// sessionWithoutPassword returns cookies for a session that carries a DN
// but no password, i.e. one getUserLDAP cannot bind with.
func sessionWithoutPassword(t *testing.T, store *session.Store) []*http.Cookie {
	t.Helper()

	mini := fiber.New()
	mini.Get("/set", func(c fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "cn=john.doe,ou=users,dc=test,dc=com")

		return sess.Save()
	})

	resp, err := mini.Test(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/set", http.NoBody))
	require.NoError(t, err)
	_ = resp.Body.Close()

	return resp.Cookies()
}

func TestBulkDeleteUsers_LDAPFailure(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {fxAliceDN, fxBobDN}}
	resp := postBulk(t, app, "/users/bulk?action=delete", form,
		"/users/"+url.PathEscape(fxAliceDN)+"?ou=ou%3Dpeople&panel=x", cookies)

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/users?ou=ou%3Dpeople", resp.Header.Get("Location"),
		"delete collapses a detail Referer to the list, keeps filters and drops the panel")

	_, ok := app.lookupUserByDN(fxAliceDN)
	assert.True(t, ok, "failed delete must not drop the user from the cache")
	_, ok = app.lookupUserByDN(fxBobDN)
	assert.True(t, ok)

	mid, ok := app.lookupGroupByDN(fxMidDN)
	require.True(t, ok)
	assert.Contains(t, mid.Members, fxAliceDN, "failed delete must not scrub group membership")

	_, body := getBody(t, app, "/users?view=list", cookies, nil)
	assert.Contains(t, body, "Failed to delete any of 2 users:")

	_, body = getBody(t, app, "/users?view=list&ou=ou%3Dstaff", cookies, nil)
	assert.NotContains(t, body, "Failed to delete", "the flash is consumed by the first read")
}

func TestBulkDeleteGroupsAndComputers_LDAPFailure(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	resp := postBulk(t, app, "/groups/bulk?action=delete", url.Values{"target_dn": {fxOtherDN}}, "", cookies)

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/groups", resp.Header.Get("Location"))

	_, ok := app.lookupGroupByDN(fxOtherDN)
	assert.True(t, ok, "failed delete must not drop the group from the cache")

	_, body := getBody(t, app, "/groups?view=list", cookies, nil)
	assert.Contains(t, body, "Failed to delete any of 1 group:", "singular noun for one target")

	resp = postBulk(t, app, "/computers/bulk?action=delete",
		url.Values{"target_dn": {"cn=pc1,ou=computers,dc=test,dc=local", "cn=pc2,ou=computers,dc=test,dc=local"}},
		"https://evil.example/computers", cookies)

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/computers", resp.Header.Get("Location"), "cross-origin Referer is ignored")

	_, body = getBody(t, app, "/computers?view=list", cookies, nil)
	assert.Contains(t, body, "Failed to delete any of 2 computers:")
}

func TestBulkDisable_LDAPFailureOnAD(t *testing.T) {
	app, store := setupFullTestApp(t)
	app.ldapConfig.IsActiveDirectory = true
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	referer := "/users?ou=ou%3Dpeople&panel=" + url.QueryEscape(fxAliceDN)
	resp := postBulk(t, app, "/users/bulk?action=disable", url.Values{"target_dn": {fxAliceDN}}, referer, cookies)

	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, referer, resp.Header.Get("Location"), "disable keeps the Referer, panel included")

	alice, ok := app.lookupUserByDN(fxAliceDN)
	require.True(t, ok)
	assert.True(t, alice.Enabled, "failed disable must not flip Enabled in the cache")

	_, body := getBody(t, app, "/users?view=list", cookies, nil)
	assert.Contains(t, body, "Failed to disable any of 1 user:")
}

func TestBulkMembership_LDAPFailureLeavesCache(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	t.Run("add-to-group", func(t *testing.T) {
		resp := postBulk(t, app, "/users/bulk?action=add-to-group",
			url.Values{"group_dn": {fxOtherDN}, "target_dn": {fxAliceDN, fxBobDN}},
			"/users?panel=x", cookies)

		require.Equal(t, http.StatusSeeOther, resp.StatusCode)
		assert.Equal(t, "/users?panel=x", resp.Header.Get("Location"))

		other, ok := app.lookupGroupByDN(fxOtherDN)
		require.True(t, ok)
		assert.Empty(t, other.Members, "failed adds are not applied to the cache")
	})

	t.Run("remove-from-group", func(t *testing.T) {
		resp := postBulk(t, app, "/users/bulk?action=remove-from-group",
			url.Values{"group_dn": {fxMidDN}, "target_dn": {fxAliceDN}}, "", cookies)

		require.Equal(t, http.StatusSeeOther, resp.StatusCode)
		assert.Equal(t, "/users", resp.Header.Get("Location"))

		mid, ok := app.lookupGroupByDN(fxMidDN)
		require.True(t, ok)
		assert.Contains(t, mid.Members, fxAliceDN, "failed removes are not applied to the cache")
	})

	t.Run("add-members to groups", func(t *testing.T) {
		resp := postBulk(t, app, "/groups/bulk?action=add-members",
			url.Values{"user_dn": {fxCarolDN}, "target_dn": {fxRootDN, fxOtherDN}}, "", cookies)

		require.Equal(t, http.StatusSeeOther, resp.StatusCode)
		assert.Equal(t, "/groups", resp.Header.Get("Location"))

		root, ok := app.lookupGroupByDN(fxRootDN)
		require.True(t, ok)
		assert.NotContains(t, root.Members, fxCarolDN)
	})

	t.Run("add-members without targets redirects without LDAP work", func(t *testing.T) {
		resp := postBulk(t, app, "/groups/bulk?action=add-members", url.Values{"user_dn": {fxCarolDN}}, "", cookies)

		require.Equal(t, http.StatusSeeOther, resp.StatusCode)
		assert.Equal(t, "/groups", resp.Header.Get("Location"))
	})
}

func TestBulkActions_SessionWithoutPasswordRedirectsToLogin(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := sessionWithoutPassword(t, store)

	cases := []struct {
		path string
		form url.Values
	}{
		{"/users/bulk?action=delete", url.Values{"target_dn": {fxAliceDN}}},
		{"/users/bulk?action=add-to-group", url.Values{"group_dn": {fxOtherDN}, "target_dn": {fxAliceDN}}},
		{"/users/bulk?action=remove-from-group", url.Values{"group_dn": {fxMidDN}, "target_dn": {fxAliceDN}}},
		{"/groups/bulk?action=add-members", url.Values{"user_dn": {fxAliceDN}, "target_dn": {fxOtherDN}}},
		{"/groups/bulk?action=delete", url.Values{"target_dn": {fxOtherDN}}},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp := postBulk(t, app, tc.path, tc.form, "", cookies)

			require.Equal(t, http.StatusSeeOther, resp.StatusCode)
			assert.Equal(t, "/login", resp.Header.Get("Location"))
		})
	}

	_, ok := app.lookupUserByDN(fxAliceDN)
	assert.True(t, ok)
	_, ok = app.lookupGroupByDN(fxOtherDN)
	assert.True(t, ok)
}
