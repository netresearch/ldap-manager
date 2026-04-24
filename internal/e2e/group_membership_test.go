//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
)

// waitForTagRowCount polls the DOM via page.WaitForFunction until the
// number of `.drawer--full .drawer__tag-row` entries equals expected.
// This is the reliable way to wait for an htmx `innerHTML` swap: the
// underlying fetch is async and `page.WaitForLoadState()` does not cover
// XHR-driven DOM mutations.
func waitForTagRowCount(t *testing.T, page playwright.Page, expected int) {
	t.Helper()
	expr := fmt.Sprintf(
		`() => document.querySelectorAll('.drawer--full .drawer__tag-row').length === %d`,
		expected,
	)
	if _, err := page.WaitForFunction(expr, nil); err != nil {
		// Surface the current count and flash text for a self-describing failure.
		actual, _ := page.Locator(".drawer--full .drawer__tag-row").Count()
		flash, _ := page.Locator(".drawer__flash--error").TextContent()
		t.Fatalf("expected tag row count %d within timeout, got %d (flash=%q): %v",
			expected, actual, strings.TrimSpace(flash), err)
	}
}

// TestAddRemoveGroupMembership is the explicit round-trip coverage
// that was missing when the v1.11 drawer add/remove forms shipped.
//
// Flow:
//  1. Log in as admin (has write ACL on the test LDAP — see aclLDIF).
//  2. Open testuser1's full-page user detail.
//  3. Note the group count, click the × on the first membership tag,
//     wait for the htmx swap, assert count dropped by 1.
//  4. Type an addable group's CN into the drawer's add-form input,
//     submit with Enter, wait for the swap, assert count bumped back.
//
// Asserts the whole modify pipeline — template form → handler → LDAP
// op → cache refresh → drawer re-render — is intact. A regression
// that silently swallows modify errors (any 2xx response + unchanged
// drawer) fails here.
func TestAddRemoveGroupMembership(t *testing.T) {
	cfg := DefaultTestConfig()
	tb := NewTestBrowser(t, cfg)
	defer tb.Close()

	page := tb.NewPage(t)
	tp := NewTestPage(t, page, cfg)

	if err := tp.LoginAsAdmin(); err != nil {
		t.Fatalf("login as admin: %v", err)
	}

	// testuser1 starts as a member of `developers`. The fixture
	// seeds `viewers` as a second addable group (see seedLDIF).
	tp.Navigate("/users/cn=testuser1,ou=users,dc=example,dc=com")

	if err := tp.WaitForSelector(".drawer--full"); err != nil {
		t.Fatalf("drawer--full did not render: %v", err)
	}

	tagRows := page.Locator(".drawer--full .drawer__tag-row")

	before, err := tagRows.Count()
	if err != nil {
		t.Fatalf("count tag rows: %v", err)
	}
	if before == 0 {
		t.Fatalf("expected at least one group membership for testuser1, got 0")
	}

	// --- REMOVE ---
	if err := page.Locator(".drawer--full .drawer__tag-remove").First().Click(); err != nil {
		t.Fatalf("click remove: %v", err)
	}

	// Wait for the htmx swap to bring the row count down.
	waitForTagRowCount(t, page, before-1)

	// --- ADD ---
	// Read the first addable group's CN from the datalist AFTER the swap
	// (the fresh fragment's datalist is what the drawer's picker uses).
	addableCN, err := page.Locator(".drawer__add-form datalist option").First().GetAttribute("value")
	if err != nil || addableCN == "" {
		t.Fatalf("no addable group in datalist: %v", err)
	}

	if err := page.Locator(".drawer__add-form .drawer__add-input").Fill(addableCN); err != nil {
		t.Fatalf("fill add input: %v", err)
	}
	if err := page.Locator(".drawer__add-form .drawer__add-input").Press("Enter"); err != nil {
		t.Fatalf("submit add form: %v", err)
	}

	// Wait for the htmx swap to bring the row count back up.
	waitForTagRowCount(t, page, before)
}

// TestAddGroupAsNonAdminShowsFlash asserts the silent-failure bug
// the user reported against Netresearch AD (bind without write perms
// swallows the LDAP error). When a modify fails with "Insufficient
// Access Rights", the drawer must render an inline error — a 200 OK
// with unchanged tags was the original regression.
func TestAddGroupAsNonAdminShowsFlash(t *testing.T) {
	cfg := DefaultTestConfig()
	tb := NewTestBrowser(t, cfg)
	defer tb.Close()

	page := tb.NewPage(t)
	tp := NewTestPage(t, page, cfg)

	if err := tp.LoginAsTestUser(); err != nil {
		t.Fatalf("login as testuser1: %v", err)
	}

	tp.Navigate("/users/cn=testuser1,ou=users,dc=example,dc=com")

	if err := tp.WaitForSelector(".drawer--full"); err != nil {
		t.Fatalf("drawer--full did not render: %v", err)
	}

	tagRows := page.Locator(".drawer--full .drawer__tag-row")

	before, err := tagRows.Count()
	if err != nil {
		t.Fatalf("count tag rows: %v", err)
	}
	if before == 0 {
		t.Fatalf("expected at least one group membership for testuser1, got 0")
	}

	// Ensure the × button is present and actionable before clicking —
	// surfaces any CSS/visibility regression as a clear failure instead
	// of a generic 30s click timeout.
	removeBtn := page.Locator(".drawer--full .drawer__tag-remove").First()
	if err := removeBtn.WaitFor(playwright.LocatorWaitForOptions{
		State: playwright.WaitForSelectorStateVisible,
	}); err != nil {
		t.Fatalf("remove button not visible: %v", err)
	}
	if err := removeBtn.Click(); err != nil {
		t.Fatalf("click remove: %v", err)
	}

	// Wait for the drawer to re-render (flash or same count); we poll
	// on the flash selector since the row count SHOULD stay unchanged
	// and can't be used as a swap signal.
	if _, err := page.WaitForFunction(
		`() => !!document.querySelector('.drawer--full .drawer__flash--error')`,
		nil,
	); err != nil {
		after, _ := tagRows.Count()
		t.Fatalf("expected drawer__flash--error to appear (row count before=%d after=%d): %v",
			before, after, err)
	}

	// The count must NOT have changed (no write perms).
	after, err := tagRows.Count()
	if err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != before {
		t.Fatalf("expected no-op (testuser1 lacks write perms), got before=%d after=%d", before, after)
	}

	flash, err := page.Locator(".drawer__flash--error").TextContent()
	if err != nil || strings.TrimSpace(flash) == "" {
		t.Fatalf("expected drawer__flash--error to explain the failure, got empty")
	}
	lower := strings.ToLower(flash)
	if !strings.Contains(lower, "permission") && !strings.Contains(lower, "access") {
		t.Fatalf("flash text should mention permission/access; got %q", flash)
	}
}
