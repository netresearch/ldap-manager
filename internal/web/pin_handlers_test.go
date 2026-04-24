// internal/web/pin_handlers_test.go
package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPin_RequiresAuth verifies that an unauthenticated POST /pin is rejected
// with either a redirect to /login or a 401. No pin is written.
func TestPin_RequiresAuth(t *testing.T) {
	app, _ := setupFullTestApp(t)

	form := url.Values{"target": {"cn=admins,dc=test"}}
	req := httptest.NewRequest("POST", "/pin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Contains(t,
		[]int{fiber.StatusFound, fiber.StatusSeeOther, fiber.StatusUnauthorized},
		resp.StatusCode)
}

// TestPinUnpin_RoundTrip verifies that an authenticated POST /pin records a
// pin in the store, and a subsequent POST /unpin removes it.
func TestPinUnpin_RoundTrip(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	// Matches the DN the mini auth-session app sets in server_test.go.
	const authDN = "cn=john.doe,ou=users,dc=test,dc=com"
	target := "cn=demo-group,ou=Groups,dc=test"

	form := url.Values{"target": {target}}

	req := httptest.NewRequest("POST", "/pin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Contains(t, []int{fiber.StatusNoContent, fiber.StatusOK}, resp.StatusCode)

	pinned, err := app.pinnedStore.IsPinned(authDN, target)
	require.NoError(t, err)
	assert.True(t, pinned, "target should be pinned after POST /pin")

	req2 := httptest.NewRequest("POST", "/unpin", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	_ = resp2.Body.Close()
	assert.Contains(t, []int{fiber.StatusNoContent, fiber.StatusOK}, resp2.StatusCode)

	pinned, err = app.pinnedStore.IsPinned(authDN, target)
	require.NoError(t, err)
	assert.False(t, pinned, "target should be unpinned after POST /unpin")
}
