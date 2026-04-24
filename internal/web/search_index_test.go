// internal/web/search_index_test.go
package web

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIndex_ShapeAndContentType(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/search-index.json", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	assert.NotEmpty(t, resp.Header.Get("ETag"))

	body, _ := io.ReadAll(resp.Body)
	var entries []SearchIndexEntry
	require.NoError(t, json.Unmarshal(body, &entries))

	for _, e := range entries {
		assert.Contains(t, []string{"user", "group", "computer"}, e.Type)
		assert.NotEmpty(t, e.DN)
		assert.NotEmpty(t, e.CN)
	}
}

func TestSearchIndex_ETagRespected(t *testing.T) {
	app, _ := setupFullTestApp(t)

	resp1, err := app.fiber.Test(httptest.NewRequest("GET", "/api/search-index.json", nil))
	require.NoError(t, err)
	etag := resp1.Header.Get("ETag")
	require.NotEmpty(t, etag)
	_ = resp1.Body.Close()

	req2 := httptest.NewRequest("GET", "/api/search-index.json", nil)
	req2.Header.Set("If-None-Match", etag)
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, fiber.StatusNotModified, resp2.StatusCode)
}

// TestSearchIndex_ETagStableAcrossInvocations guards the
// deterministic-sort fix in buildSearchIndex. Without the sort,
// ldap_cache's iteration order can vary between invocations and every
// request would mint a fresh ETag, defeating the client-side cache.
// Three back-to-back requests must all produce the same ETag when the
// underlying cache content is unchanged.
func TestSearchIndex_ETagStableAcrossInvocations(t *testing.T) {
	app, _ := setupFullTestApp(t)

	etags := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest("GET", "/api/search-index.json", nil)
		resp, err := app.fiber.Test(r)
		require.NoError(t, err)
		etags = append(etags, resp.Header.Get("ETag"))
		_ = resp.Body.Close()
	}

	assert.NotEmpty(t, etags[0])
	assert.Equal(t, etags[0], etags[1], "ETag must be stable across invocations")
	assert.Equal(t, etags[1], etags[2], "ETag must be stable across invocations")
}

// TestImmediateOU locks in the ParseDN-based implementation:
//   - returns "ou=<value>" for the first OU RDN walking root-upward
//   - copes with escaped commas inside RDN values (cn=Last\, First, …)
//   - returns "" when there is no OU component or the DN is malformed
//   - returns lowercase "ou=" prefix regardless of input casing
func TestImmediateOU(t *testing.T) {
	cases := []struct {
		name string
		dn   string
		want string
	}{
		{
			name: "straightforward user DN",
			dn:   "cn=alice,ou=Engineering,dc=example,dc=com",
			want: "ou=Engineering",
		},
		{
			name: "uppercase OU in input → lowercase ou= in output",
			dn:   "CN=alice,OU=Engineering,DC=example,DC=com",
			want: "ou=Engineering",
		},
		{
			name: "mixed-case 'Ou=' parses as ou",
			dn:   "cn=alice,Ou=Engineering,dc=example,dc=com",
			want: "ou=Engineering",
		},
		{
			name: "innermost OU returned when multiple are nested",
			dn:   "cn=alice,ou=Engineering,ou=London,dc=example,dc=com",
			want: "ou=Engineering",
		},
		{
			name: "escaped comma in CN — raw-comma scanner mis-parses this",
			dn:   `cn=Last\, First,ou=Sales,dc=example,dc=com`,
			want: "ou=Sales",
		},
		{
			name: "no OU component",
			dn:   "cn=alice,dc=example,dc=com",
			want: "",
		},
		{
			name: "empty DN",
			dn:   "",
			want: "",
		},
		{
			name: "malformed DN",
			dn:   "this-is-not-a-dn",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := immediateOU(tc.dn)
			assert.Equal(t, tc.want, got,
				"immediateOU(%q) mismatch", tc.dn)
		})
	}
}
