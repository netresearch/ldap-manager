// internal/web/graph_integration_test.go — integration tests for
// /api/graph.json against a real OpenLDAP container. Skipped when no
// container is reachable on 127.0.0.1:1389.
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// TestGraphJSON_IntegrationUserFocus exercises /api/graph.json against a
// real OpenLDAP container with seeded data, going through the same
// RequireAuth-protected route the production server registers. testuser1
// is a member of two seeded groups (admins, developers), so the returned
// graph should contain at minimum the focus node plus the group edges.
func TestGraphJSON_IntegrationUserFocus(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)

	app, store := setupLDAPTestApp(t, env)
	// Refresh the cache after seeding so the user/group data the test
	// just installed is visible to BuildGraph.
	app.ldapCache.Refresh()

	cookies := createAuthSession(t, app, store)

	entity := "cn=testuser1,ou=users," + env.baseDN
	target := "/api/graph.json?entity=" + url.QueryEscape(entity)
	req := httptest.NewRequestWithContext(context.Background(), "GET", target, http.NoBody)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	resp, err := app.fiber.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var data ldap_cache.GraphData
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&data))

	require.Equal(t, entity, data.Focus)
	require.GreaterOrEqual(t, len(data.Nodes), 1, "expected at least one node (the focus)")
}
