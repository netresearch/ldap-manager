// internal/web/graph_v2_handler_test.go — unit tests for /api/graph.json.
package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
