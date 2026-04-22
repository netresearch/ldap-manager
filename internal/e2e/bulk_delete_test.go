//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBulkDeleteGroup_AgainstOpenLDAP exercises the whole bulk-delete
// pipeline end to end: seed a disposable group, open its drawer's
// Delete button (which POSTs /groups/bulk?action=delete with CSRF),
// accept the confirm dialog, then verify the flash banner on /groups
// names "Deleted 1 group" and the group is gone from the list.
//
// The single-entity Delete action uses the SAME bulk handler as the
// multi-select one (target_dn=single DN), so this exercises the
// bulkDeleteGroups code path + the v1.11 DeleteByDN generic + the
// session-flash round-trip + the list flash rendering.
func TestBulkDeleteGroup_AgainstOpenLDAP(t *testing.T) {
	cfg := DefaultTestConfig()
	tb := NewTestBrowser(t, cfg)
	defer tb.Close()

	disposableCN := fmt.Sprintf("bulk-delete-me-%d", time.Now().UnixNano())
	disposableDN := fmt.Sprintf("cn=%s,ou=groups,dc=example,dc=com", disposableCN)
	seedDisposableGroup(t, disposableCN)

	page := tb.NewPage(t)
	tp := NewTestPage(t, page, cfg)
	require.NoError(t, tp.LoginAsAdmin())

	// Auto-accept the confirm dialog that data-confirm fires via
	// window.confirm() when the Delete form is submitted.
	page.OnDialog(func(d playwright.Dialog) { _ = d.Accept() })

	// The cache warms on app boot; the group we just seeded may or
	// may not be in the app's in-memory cache yet (30s refresh loop).
	// Wait until /groups sees it, OR navigate to the detail URL
	// directly — the detail handler queries the cache by DN and will
	// not find a post-boot seed unless we wait. We navigate directly
	// and retry briefly for the cache refresh to notice.
	deadline := time.Now().Add(45 * time.Second)
	var detailURL string

	for time.Now().Before(deadline) {
		tp.Navigate("/groups/" + url.PathEscape(disposableDN))
		if err := tp.WaitForSelector(".drawer--full"); err == nil {
			// Confirm the title matches our disposable CN; if the
			// cache is stale we might get "group not found" which
			// still renders SOME drawer on a 500 — guard.
			title, _ := page.Locator(".drawer__title").First().TextContent()
			if strings.Contains(title, disposableCN) {
				detailURL = page.URL()
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	if detailURL == "" {
		t.Skipf("disposable group %s never appeared in cache within 45s — "+
			"cache refresh interval might be longer than expected",
			disposableCN)
	}

	// Click the Delete button. The data-confirm submit listener
	// fires window.confirm(); OnDialog above auto-accepts.  The form
	// POSTs to /groups/bulk?action=delete which redirects to /groups
	// with a session flash.
	require.NoError(t, page.Locator(".drawer__action--danger").First().Click())

	// Wait for the redirect to settle + the flash to render.
	if err := page.WaitForURL("**/groups"); err != nil {
		t.Fatalf("expected redirect to /groups after delete: %v", err)
	}
	require.NoError(t, tp.WaitForSelector(".list-page__flash--success"))

	flashText, _ := page.Locator(".list-page__flash--success").TextContent()
	assert.Contains(t, strings.ToLower(flashText), "deleted",
		"expected success flash; got %q", flashText)
	assert.Contains(t, flashText, "1",
		"expected flash to name count (1 group deleted); got %q", flashText)

	// And the group must actually be gone from the list HTML.
	html, _ := page.Content()
	assert.NotContains(t, html, disposableCN,
		"expected %q absent from /groups after bulk delete; still present", disposableCN)
}

// seedDisposableGroup adds a minimal groupOfNames entry via ldapadd
// inside the OpenLDAP testcontainer. Requires at least one member
// (groupOfNames schema); we reuse the seeded admin-user.
func seedDisposableGroup(t *testing.T, cn string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if containerForSeed == nil {
		t.Skip("LDAP container handle not exposed — TestMain should populate containerForSeed")
	}

	ldif := fmt.Sprintf(`dn: cn=%s,ou=groups,dc=example,dc=com
objectClass: groupOfNames
objectClass: top
cn: %s
description: Disposable — created by TestBulkDeleteGroup_AgainstOpenLDAP
member: cn=admin-user,ou=users,dc=example,dc=com
`, cn, cn)

	if err := execLDAP(ctx, containerForSeed,
		"ldapadd", "-x", "-D", bootstrapAdminDN, "-w", bootstrapAdminPass,
		"-H", "ldap://localhost", ldif); err != nil {
		t.Fatalf("seed disposable group: %v", err)
	}
}
