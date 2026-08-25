//go:build e2e

package e2e

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBulkToolbarAddMembers_AgainstOpenLDAP drives the ACTUAL bulk
// toolbar (v2-bulk.js): tick a group row checkbox on /groups, click
// "Add members…", answer the prompt() with an existing user DN, and
// verify the JS-built POST passes CSRF validation and lands the
// membership change.
//
// Regression guard for issue #652: submitForm() in v2-bulk.js built
// its POST form without a csrf_token, so EVERY toolbar action died
// with 403 "CSRF token validation failed". The server-side half of
// the contract (list page exposes the token via data-csrf) is pinned
// by TestBulkToolbar_ListPageExposesUsableCSRFToken in internal/web;
// this test covers the client half that only a browser executes.
func TestBulkToolbarAddMembers_AgainstOpenLDAP(t *testing.T) {
	cfg := DefaultTestConfig()
	tb := NewTestBrowser(t, cfg)
	defer tb.Close()

	disposableCN := fmt.Sprintf("bulk-toolbar-csrf-%d", time.Now().UnixNano())
	disposableDN := fmt.Sprintf("cn=%s,ou=groups,dc=example,dc=com", disposableCN)
	seedDisposableGroup(t, disposableCN)

	// testuser1 exists in the seed LDIF and is NOT the groupOfNames
	// bootstrap member (that's admin-user), so adding it is a real
	// state change we can assert on afterwards.
	const memberDN = "cn=testuser1,ou=users,dc=example,dc=com"

	page := tb.NewPage(t)
	tp := NewTestPage(t, page, cfg)
	require.NoError(t, tp.LoginAsAdmin())

	// Answer the toolbar's prompt() with the user DN to add.
	page.OnDialog(func(d playwright.Dialog) { _ = d.Accept(memberDN) })

	// Wait for the app's cache refresh loop to surface the seeded
	// group on the /groups list (same pattern as bulk_delete_test).
	checkboxSel := fmt.Sprintf(`input[data-bulk][value=%q]`, disposableDN)
	deadline := time.Now().Add(45 * time.Second)
	found := false

	for time.Now().Before(deadline) {
		tp.Navigate("/groups")
		if err := tp.WaitForSelector(checkboxSel); err == nil {
			found = true

			break
		}
		time.Sleep(1 * time.Second)
	}

	if !found {
		t.Skipf("disposable group %s never appeared on /groups within 45s — "+
			"cache refresh interval might be longer than expected", disposableCN)
	}

	// Tick the row checkbox → the bulk bar appears with the
	// groups-scope actions.
	require.NoError(t, page.Locator(checkboxSel).Check())
	require.NoError(t, tp.WaitForSelector(".bulk-bar"))

	// Click "Add members…" (first non-danger action in groups scope).
	// The prompt() fires, OnDialog accepts with memberDN, and
	// v2-bulk.js submits the form to /groups/bulk?action=add-members.
	require.NoError(t, page.Locator(".bulk-bar__action",
		playwright.PageLocatorOptions{HasText: "Add members"}).Click())

	// The handler redirects back to /groups; before the #652 fix this
	// rendered the 403 "Access forbidden" page instead.
	require.NoError(t, page.WaitForURL("**/groups"))

	html, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, html, "CSRF token validation failed",
		"bulk toolbar POST must carry the CSRF token (issue #652)")

	// End-to-end proof: the membership change actually landed.
	tp.Navigate("/groups/" + url.PathEscape(disposableDN))
	require.NoError(t, tp.WaitForSelector(".drawer--full"))

	detailHTML, err := page.Content()
	require.NoError(t, err)
	assert.True(t, strings.Contains(detailHTML, "testuser1"),
		"expected testuser1 to be a member of %s after bulk add-members", disposableCN)
}
