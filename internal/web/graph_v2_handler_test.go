// internal/web/graph_v2_handler_test.go — unit tests for /api/graph.json.
package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/ldap_cache/cachetest"
)

// bobDN is the seeded user used by the success-path tests. The handler
// only reads from app.ldapCache, so it doesn't need to match any session
// identity.
const bobDN = "cn=bob,dc=test,dc=local"

// seedBob installs a single user with bobDN into the test app's cache so
// /api/graph.json?entity=bobDN resolves to a real entity instead of 404.
func seedBob(t *testing.T, app *App) {
	t.Helper()
	cachetest.Seed(app.ldapCache, []ldap.User{
		cachetest.NewUserWithDN(bobDN, "bob", "bob", true, nil),
	}, nil, nil)
}

func TestHandleGraphJSON_MissingEntity(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/graph.json", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleGraphJSON_InvalidDN(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/graph.json?entity=not~a~dn", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleGraphJSON_UnknownDN(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/graph.json?entity=cn=ghost,dc=ex,dc=com", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestHandleGraphJSON_InvalidDepth(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/graph.json?entity="+bobDN+"&depth=abc", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestHandleGraphJSON_ETagStable(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedBob(t, app)

	req1 := httptest.NewRequest("GET", "/api/graph.json?entity="+bobDN, nil)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)

	if resp1.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp1.Body)
		_ = resp1.Body.Close()
		t.Fatalf("first call: got %d, want 200; body=%q", resp1.StatusCode, string(body))
	}

	etag := resp1.Header.Get("ETag")
	_ = resp1.Body.Close()

	if etag == "" {
		t.Fatal("ETag missing on first call")
	}

	req2 := httptest.NewRequest("GET", "/api/graph.json?entity="+bobDN, nil)
	req2.Header.Set("If-None-Match", etag)
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("status: got %d, want 304", resp2.StatusCode)
	}
}

func TestHandleGraphJSON_NotModifiedKeepsHeaders(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedBob(t, app)

	req1 := httptest.NewRequest("GET", "/api/graph.json?entity="+bobDN, nil)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)
	etag := resp1.Header.Get("ETag")
	_ = resp1.Body.Close()
	require.NotEmpty(t, etag)

	req2 := httptest.NewRequest("GET", "/api/graph.json?entity="+bobDN, nil)
	req2.Header.Set("If-None-Match", etag)
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	require.Equal(t, http.StatusNotModified, resp2.StatusCode)
	require.Equal(t, etag, resp2.Header.Get("ETag"), "304 must echo ETag")
	require.NotEmpty(t, resp2.Header.Get("Cache-Control"), "304 must keep Cache-Control")
}

func TestHandleGraphJSON_DepthClamping(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedBob(t, app)

	for _, raw := range []string{"0", "99", "-5"} {
		t.Run("depth="+raw, func(t *testing.T) {
			url := "/api/graph.json?entity=" + bobDN + "&depth=" + raw
			req := httptest.NewRequest("GET", url, nil)
			resp, err := app.fiber.Test(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("depth=%q status: got %d, want 200", raw, resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var data ldap_cache.GraphData
			if err := json.Unmarshal(body, &data); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if data.Depth < 1 || data.Depth > 3 {
				t.Errorf("depth=%q returned Depth=%d, expected [1,3]", raw, data.Depth)
			}
		})
	}
}

func TestHandleUsersV2_GraphMode(t *testing.T) {
	app, store := setupFullTestApp(t)
	// Seed bob with one group membership so BuildListGraph has both
	// a user (ring 2) and a group (ring 1).
	cachetest.Seed(app.ldapCache,
		[]ldap.User{
			cachetest.NewUserWithDN(bobDN, "bob", "bob", true, []string{
				"cn=engineers,ou=Groups,dc=test,dc=local",
			}),
		},
		[]ldap.Group{
			cachetest.NewGroupWithDN("cn=engineers,ou=Groups,dc=test,dc=local", "engineers", []string{bobDN}),
		},
		nil,
	)

	cookies := createAuthSession(t, app, store)
	req := httptest.NewRequest("GET", "/users?view=graph", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	for _, marker := range []string{
		`id="graph-canvas"`,
		`id="graph-data"`,
		`class="graph-table"`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("missing HTML marker %q in /users?view=graph response", marker)
		}
	}
}

func TestHandleGraphV2_RendersHTML(t *testing.T) {
	app, _ := setupFullTestApp(t)
	seedBob(t, app)

	req := httptest.NewRequest("GET", "/graph?entity="+bobDN, nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	for _, marker := range []string{
		`id="graph-canvas"`,
		`id="graph-data"`,
		`class="graph-table"`,
		`Relationships: bob`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("missing HTML marker %q", marker)
		}
	}
}

func TestHandleGroupsV2_GraphMode(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	// Seed a group with bob as a member; the handler walks Members
	// and looks each DN up in Users/Computers caches.
	const engineersDN = "cn=engineers,ou=Groups,dc=test,dc=local"
	cachetest.Seed(app.ldapCache,
		[]ldap.User{
			cachetest.NewUserWithDN(bobDN, "bob", "bob", true, []string{engineersDN}),
		},
		[]ldap.Group{
			cachetest.NewGroupWithDN(engineersDN, "engineers", []string{bobDN}),
		},
		nil,
	)

	req := httptest.NewRequest("GET", "/groups?view=graph", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	for _, marker := range []string{`id="graph-canvas"`, `id="graph-data"`, `class="graph-table"`} {
		if !strings.Contains(html, marker) {
			t.Errorf("missing HTML marker %q", marker)
		}
	}
	// Member-lookup regression check: bob should appear in the graph
	// data because the groups handler walks Members and resolves them.
	require.Contains(t, html, "bob", "expected the looked-up member to appear in the rendered graph")
}

func TestHandleComputersV2_GraphMode(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	const ws01DN = "cn=ws01,ou=Computers,dc=test,dc=local"
	const engineersDN = "cn=engineers,ou=Groups,dc=test,dc=local"
	cachetest.Seed(app.ldapCache,
		nil,
		[]ldap.Group{
			cachetest.NewGroupWithDN(engineersDN, "engineers", []string{ws01DN}),
		},
		[]ldap.Computer{
			cachetest.NewComputerWithDN(ws01DN, "ws01", "ws01$", true, []string{engineersDN}),
		},
	)

	req := httptest.NewRequest("GET", "/computers?view=graph", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	for _, marker := range []string{`id="graph-canvas"`, `id="graph-data"`, `class="graph-table"`, "ws01"} {
		if !strings.Contains(html, marker) {
			t.Errorf("missing HTML marker %q", marker)
		}
	}
}

// TestHandleUsersV2_TableMode covers the new ?view=table branch — the
// full-width Table view with no filter rail or detail drawer.
func TestHandleUsersV2_TableMode(t *testing.T) {
	app, store := setupFullTestApp(t)
	cachetest.Seed(app.ldapCache,
		[]ldap.User{
			cachetest.NewUserWithDN(bobDN, "bob", "bob", true, nil),
		},
		nil,
		nil,
	)

	cookies := createAuthSession(t, app, store)
	req := httptest.NewRequest("GET", "/users?view=table", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html",
		"table render must declare HTML content-type — earlier omission caused browsers to display raw HTML")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	for _, marker := range []string{
		`class="list-table"`,
		`<th scope="col">CN</th>`,
		"bob",
		`graph-segmented`,
	} {
		require.Contains(t, html, marker, "missing table-view marker %q", marker)
	}
}

// TestHandleGroupsV2_TableMode mirrors TestHandleUsersV2_TableMode.
func TestHandleGroupsV2_TableMode(t *testing.T) {
	app, store := setupFullTestApp(t)
	const engineersDN = "cn=engineers,ou=Groups,dc=test,dc=local"
	cachetest.Seed(app.ldapCache, nil,
		[]ldap.Group{
			cachetest.NewGroupWithDN(engineersDN, "engineers", []string{bobDN}),
		},
		nil,
	)

	cookies := createAuthSession(t, app, store)
	req := httptest.NewRequest("GET", "/groups?view=table", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `class="list-table"`)
	require.Contains(t, string(body), "engineers")
}

// TestHandleComputersV2_TableMode mirrors the user/group versions.
func TestHandleComputersV2_TableMode(t *testing.T) {
	app, store := setupFullTestApp(t)
	const ws01DN = "cn=ws01,ou=Computers,dc=test,dc=local"
	cachetest.Seed(app.ldapCache, nil, nil, []ldap.Computer{
		cachetest.NewComputerWithDN(ws01DN, "ws01", "ws01$", true, nil),
	})

	cookies := createAuthSession(t, app, store)
	req := httptest.NewRequest("GET", "/computers?view=table", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `class="list-table"`)
	require.Contains(t, string(body), "ws01")
}

// TestHandleUsersV2_PersistentViewViaCookie covers the cookie path:
// when no ?view= is in the URL, pickView falls back to the graph-view
// cookie. Earlier the template-cache key didn't include the cookie, so
// the first cached response stuck regardless of preference.
func TestHandleUsersV2_PersistentViewViaCookie(t *testing.T) {
	app, store := setupFullTestApp(t)
	cachetest.Seed(app.ldapCache,
		[]ldap.User{
			cachetest.NewUserWithDN(bobDN, "bob", "bob", true, nil),
		},
		nil, nil,
	)

	cookies := createAuthSession(t, app, store)
	req := httptest.NewRequest("GET", "/users", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.AddCookie(&http.Cookie{Name: "graph-view", Value: "table"})

	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `class="list-table"`,
		"cookie should resolve to table view when no ?view= is set")
}
