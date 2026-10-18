// internal/web/users_edit_test.go — inline attribute edit dispatched from
// POST /users/* when the form carries `field`. The unit harness has no
// writable directory (ldap_fixture_test.go), so the write itself always
// fails; a successful edit is covered by the integration suite.
package web

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUserV2Edit(t *testing.T) {
	app, store := setupFullTestApp(t)
	seedDirectoryFixture(t, app)
	cookies := createAuthSession(t, app, store)

	path := "/users/" + url.PathEscape(fxAliceDN)

	t.Run("field outside the whitelist is rejected", func(t *testing.T) {
		resp := postBulk(t, app, path, url.Values{"field": {"userPassword"}, "value": {"x"}}, "", cookies)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "field not editable", string(body))
	})

	for _, tc := range []struct{ name, field, value string }{
		{"set email", "email", "alice@example.com"},
		{"clear description", "description", ""},
	} {
		t.Run("directory failure on "+tc.name+" is a 500 and leaves the cache", func(t *testing.T) {
			resp := postBulk(t, app, path, url.Values{"field": {tc.field}, "value": {tc.value}},
				"", cookies)

			require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.NotContains(t, string(body), "127.0.0.1", "internal error details are not leaked")

			alice, ok := app.lookupUserByDN(fxAliceDN)
			require.True(t, ok)
			assert.Nil(t, alice.Mail)
			assert.Empty(t, alice.Description)
		})
	}

	t.Run("session without password is sent to login", func(t *testing.T) {
		resp := postBulk(t, app, path, url.Values{"field": {"email"}, "value": {"a@b.c"}}, "",
			sessionWithoutPassword(t, store))

		require.Equal(t, http.StatusSeeOther, resp.StatusCode)
		assert.Equal(t, "/login", resp.Header.Get("Location"))
	})
}
