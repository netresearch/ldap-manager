//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsersV2_FlowAndAAA covers the happy path for the V2 users list:
// list loads with a visible H1, initial drawer shows empty state, clicking
// the first row swaps the drawer via htmx (populating title + pivots),
// hx-push-url advances the URL to /users/:dn, and axe-core reports zero
// WCAG 2.2 AAA violations on the post-swap view.
func TestUsersV2_FlowAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())

	tp.Navigate("/users")

	// H1 title is present and has a visible bounding box.
	require.NoError(t, page.Locator("h1.list-page__title").WaitFor())
	titleBox, err := page.Locator("h1.list-page__title").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, titleBox)
	assert.Greater(t, titleBox.Height, 0.0)

	// Empty drawer message on initial load.
	emptyVisible, err := page.Locator(".drawer .drawer__empty").First().IsVisible()
	require.NoError(t, err)
	assert.True(t, emptyVisible, "initial drawer should show empty state")

	// Click the first row — htmx should swap the drawer in place.
	rowCount, _ := page.Locator(".list-row__link").Count()
	require.Greater(t, rowCount, 0, "expected at least one user row")

	firstRow := page.Locator(".list-row__link").First()
	require.NoError(t, firstRow.Click())

	// Wait for the drawer head to appear (htmx afterSwap populates it).
	require.NoError(t, page.Locator(".drawer__head .drawer__title").WaitFor())

	// Drawer now has a title and at least one pivot.
	titleText, err := page.Locator(".drawer__head .drawer__title").First().TextContent()
	require.NoError(t, err)
	assert.NotEmpty(t, titleText)

	pivotCount, _ := page.Locator(".drawer__pivot").Count()
	assert.GreaterOrEqual(t, pivotCount, 1, "at least one pivot link")

	// URL was push-updated to /users/:dn via hx-push-url.
	assert.Contains(t, page.URL(), "/users/", "hx-push-url should advance the URL")

	// axe AAA pass on the post-swap page.
	axePath, err := filepath.Abs("internal/e2e/testdata/axe.min.js")
	require.NoError(t, err, "resolve axe path")
	axeSrc, err := os.ReadFile(axePath)
	require.NoError(t, err, "read axe.min.js")

	// Inject axe via page.Evaluate (runs in an isolated context that bypasses
	// the page's Content-Security-Policy). AddScriptTag with Content would be
	// blocked by our script-src 'self' CSP.
	_, err = page.Evaluate(string(axeSrc))
	require.NoError(t, err, "inject axe via evaluate")

	raw, err := page.Evaluate(`
		() => axe.run(document, {
			runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag2aaa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
			resultTypes: ['violations'],
		})
	`)
	require.NoError(t, err, "axe.run")

	b, err := json.Marshal(raw)
	require.NoError(t, err, "marshal axe result")

	var ar axeResult
	require.NoError(t, json.Unmarshal(b, &ar), "unmarshal axe result")

	if len(ar.Violations) > 0 {
		for _, v := range ar.Violations {
			t.Errorf("axe violation [%s]: %s — %d node(s). help: %s",
				v.ID, v.Description, len(v.Nodes), v.HelpURL)
			for i, n := range v.Nodes {
				if i >= 3 {
					t.Logf("  (and %d more node(s) truncated)", len(v.Nodes)-i)
					break
				}
				t.Logf("  %s", n.Target)
			}
		}
		t.Fatalf("%d axe violation(s) on /users post-drawer-swap", len(ar.Violations))
	}
}
