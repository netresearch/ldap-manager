//go:build e2e

package e2e

import (
	"net/url"
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphHappyPath covers the Slice 4 interactive graph flow:
//  1. Sign in as test user, navigate to /users, pick the first user's DN.
//  2. Navigate directly to /graph?entity=<dn>&depth=2 (the drawer pivot
//     that Slice 6 will add isn't shipped yet).
//  3. Assert the SVG canvas and edge table are present.
//  4. Assert the inline graph data is parseable.
//  5. Click an expandable node's badge — assert the aria-live region
//     announces an "Expanded ..." message.
//  6. With reduced-motion emulated, repeat the page load — confirm the
//     same DOM still renders (motion-disable verification proper belongs
//     in the Slice 6 axe-core ratchet).
//
// Plan deviations:
//   - Slice 6 will add the "View relationships" drawer pivot (Task 35).
//     Until then we navigate directly to /graph.
//   - Slice 4 expand updates the SVG only; the edge <table> is SSR-only,
//     so the plan's "new <tr> appears" assertion is impossible without
//     a server-side re-render. Skipped.
func TestGraphHappyPath(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())

	// Step 1: pick the first user from /users.
	tp.Navigate("/users")
	require.NoError(t, page.Locator(".list-row__link").First().WaitFor())

	firstRowHref, err := page.Locator(".list-row__link").First().GetAttribute("href")
	require.NoError(t, err)
	require.NotEmpty(t, firstRowHref)
	// firstRowHref looks like "/users/cn%3Dtestuser1%2Cou%3Dusers%2Cdc%3Dtest%2Cdc%3Dlocal".
	const userPrefix = "/users/"
	require.True(t, strings.HasPrefix(firstRowHref, userPrefix), "unexpected user href shape: %s", firstRowHref)
	encodedDN := strings.TrimPrefix(firstRowHref, userPrefix)
	// userDetailHref emits paths via url.PathEscape; decode with the
	// matching PathUnescape (QueryUnescape would convert '+' to space,
	// which is wrong for path segments).
	dn, err := url.PathUnescape(encodedDN)
	require.NoError(t, err)
	// Don't assume a specific base DN — local dev uses dc=test,dc=local
	// while CI's e2e fixture uses dc=example,dc=com. Just sanity-check
	// the shape: it should be a CN under some OU.
	require.Contains(t, dn, "cn=", "expected a CN-based DN")
	require.Contains(t, dn, "ou=", "expected the DN to live under an OU")

	// Step 2: direct nav to /graph for that user, depth=2.
	graphPath := "/graph?entity=" + url.QueryEscape(dn) + "&depth=2"
	tp.Navigate(graphPath)

	// Step 3: SVG canvas + table present.
	require.NoError(t, page.Locator("svg#graph-canvas").WaitFor())
	require.NoError(t, page.Locator("table.graph-table").WaitFor())

	// Step 4: inline graph-data template parses.
	dataLen, err := page.Evaluate(`() => {
		const el = document.getElementById('graph-data');
		if (!el || !el.content) return -1;
		try {
			return JSON.parse(el.content.textContent).nodes.length;
		} catch (e) {
			return -2;
		}
	}`)
	require.NoError(t, err)
	// playwright-go's JSON transport unmarshals numbers as float64 by
	// default, but accept int/int64 too to stay robust across versions.
	var dataLenInt int
	switch v := dataLen.(type) {
	case int:
		dataLenInt = v
	case int64:
		dataLenInt = int(v)
	case float64:
		dataLenInt = int(v)
	default:
		t.Fatalf("expected numeric from page.Evaluate, got %T (%v)", dataLen, dataLen)
	}
	assert.Greater(t, dataLenInt, 0, "graph-data should contain at least the focus node")

	// Step 5: click an expandable node and watch the aria-live region.
	// First, wait for v2-graph.js to have activated nodes (added tabindex).
	require.NoError(t, page.Locator(".graph-node[tabindex='0']").First().WaitFor())

	// Look for an expandable node with a badge.
	badgeLocator := page.Locator(".graph-node[data-expandable='true'] .graph-node__expand-badge").First()
	badgeCount, err := badgeLocator.Count()
	require.NoError(t, err)
	if badgeCount > 0 {
		require.NoError(t, badgeLocator.Click())
		// Aria-live region should pick up an "Expanded" announcement.
		require.NoError(t, page.Locator("#graph-announce").WaitFor())
		// Wait briefly for fetch + announce.
		_, err = page.WaitForFunction(`() => {
			const el = document.getElementById('graph-announce');
			return el && el.textContent.indexOf('Expanded') !== -1;
		}`, nil)
		require.NoError(t, err, "expected aria-live region to announce expansion")
	} else {
		t.Log("no expandable node visible — graph topology has no further hops; skipping expand assertion")
	}

	// Step 6: reduced-motion render. The CSS already disables graph
	// transitions when prefers-reduced-motion is set; the JS doesn't
	// add any. We just confirm the same selectors still resolve under
	// the reduced-motion media query.
	rmPage := browser.NewPage(t)
	defer rmPage.Close()
	require.NoError(t, rmPage.EmulateMedia(playwright.PageEmulateMediaOptions{
		ReducedMotion: playwright.ReducedMotionReduce,
	}))

	rmTP := NewTestPage(t, rmPage, config)
	require.NoError(t, rmTP.LoginAsTestUser())
	rmTP.Navigate(graphPath)
	require.NoError(t, rmPage.Locator("svg#graph-canvas").WaitFor())
	require.NoError(t, rmPage.Locator("table.graph-table").WaitFor())
}
