//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/url"
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

	// Unlike the bulk-delete e2e (whose action under test removes its
	// disposable group), this test leaves the group behind — and later
	// tests in the package pick groups dynamically, so the leftover
	// polluted them (CI: TestAddRemoveGroupMembership hit LDAP error 20
	// "value already exists" adding testuser1 to a stale
	// bulk-toolbar-csrf-* group). Delete it when the test ends.
	// The happy path deletes the group THROUGH THE UI at the end of the
	// test (bulk "Delete groups"), which removes it from LDAP and the
	// app cache in one stroke. This backstop only fires when the test
	// dies before reaching that step. It deletes directly in the
	// directory, so the app cache keeps a ghost of the group until the
	// next refresh — that ghost poisoned TestAddRemoveGroupMembership in
	// CI (it sorts first in the addable datalist; adding to it fails
	// silently and cascades) — hence the settle wait for one refresh
	// cycle before letting the next test start.
	cleanedViaUI := false
	t.Cleanup(func() {
		if cleanedViaUI {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := execLDAP(ctx, containerForSeed,
			"ldapdelete", "-x", "-D", bootstrapAdminDN, "-w", bootstrapAdminPass,
			"-H", "ldap://localhost", disposableDN+"\n"); err != nil {
			t.Logf("cleanup: failed to delete %s: %v", disposableDN, err)
		}
		t.Logf("cleanup: waiting out one cache refresh so the deleted group's ghost cannot poison later tests")
		time.Sleep(35 * time.Second)
	})

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
	// group on the /groups list. Poll with Locator.Count() — the list
	// page is server-rendered and static after navigation, so a
	// WaitFor-style call would burn its full 30s page timeout staring
	// at a DOM that cannot change, turning the loop into ~1 attempt.
	checkboxSel := fmt.Sprintf(`input[data-bulk][value=%q]`, disposableDN)
	deadline := time.Now().Add(45 * time.Second)
	found := false

	for time.Now().Before(deadline) {
		tp.Navigate("/groups")
		if n, cntErr := page.Locator(checkboxSel).Count(); cntErr == nil && n > 0 {
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
	// The click is wrapped in ExpectNavigation because the page is
	// ALREADY on /groups — a bare WaitForURL("**/groups") would match
	// the current URL and resolve before the POST navigation happens,
	// letting the assertions below run against the pre-POST DOM.
	// (add-members redirects without a flash, unlike bulk delete, so
	// there is no flash element to wait for.)
	_, err := page.ExpectNavigation(func() error {
		return page.Locator(".bulk-bar__action",
			playwright.PageLocatorOptions{HasText: "Add members"}).Click()
	})
	require.NoError(t, err)

	// The handler redirects back to /groups; before the #652 fix this
	// rendered the 403 "Access forbidden" page instead.
	html, err := page.Content()
	require.NoError(t, err)
	require.NotContains(t, html, "CSRF token validation failed",
		"bulk toolbar POST must carry the CSRF token (issue #652)")

	// End-to-end proof: the membership change actually landed. Assert
	// on the member tag's remove-form input, which renders ONLY for
	// actual members — a bare Contains(html, "testuser1") would also
	// match the add-user datalist of UNASSIGNED users, i.e. it holds in
	// both the landed and the silently-failed case (add-members logs
	// per-entry LDAP errors and redirects without a flash).
	tp.Navigate("/groups/" + url.PathEscape(disposableDN))
	require.NoError(t, tp.WaitForSelector(".drawer--full"))

	memberMarker := fmt.Sprintf(
		`.drawer__tag-remove-form input[name="removeuser"][value=%q]`, memberDN)
	n, err := page.Locator(memberMarker).Count()
	require.NoError(t, err)
	assert.Equal(t, 1, n,
		"expected testuser1's member remove-form in %s after bulk add-members", disposableCN)

	// Tear down through the UI: bulk "Delete groups" removes the group
	// from LDAP AND the app cache synchronously (a direct ldapdelete
	// would leave a cache ghost that poisons later tests — seen in CI).
	// This also exercises submitForm's second groups-scope action with
	// the same CSRF token plumbing. OnDialog above auto-accepts the
	// confirm() prompt.
	tp.Navigate("/groups")
	require.NoError(t, page.Locator(checkboxSel).Check())
	require.NoError(t, tp.WaitForSelector(".bulk-bar"))

	_, err = page.ExpectNavigation(func() error {
		return page.Locator(".bulk-bar__action",
			playwright.PageLocatorOptions{HasText: "Delete groups"}).Click()
	})
	require.NoError(t, err)
	require.NoError(t, tp.WaitForSelector(".list-page__flash--success"))

	gone, err := page.Locator(checkboxSel).Count()
	require.NoError(t, err)
	require.Zero(t, gone, "disposable group %s must be gone from /groups after bulk delete", disposableCN)
	cleanedViaUI = true
}
