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

// TestGroupsV2_FlowAndAAA covers the happy path for the V2 groups list:
// list loads with a visible H1, and — when the fixture has at least one
// group — clicking the first row swaps the drawer via htmx (populating the
// title) and axe-core reports zero WCAG 2.2 AAA violations on the post-swap
// view. If the e2e fixture has no seeded groups the drawer path is skipped.
func TestGroupsV2_FlowAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())

	tp.Navigate("/groups")

	// H1 title is present and has a visible bounding box.
	require.NoError(t, page.Locator("h1.list-page__title").WaitFor())
	titleBox, err := page.Locator("h1.list-page__title").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, titleBox)
	assert.Greater(t, titleBox.Height, 0.0)

	// If there are no seeded groups in the fixture, skip the drawer swap
	// and AAA checks — nothing to click.
	rowCount, _ := page.Locator(".list-row__link").Count()
	if rowCount == 0 {
		t.Skip("no seeded groups in e2e fixture — skipping drawer swap path")
	}

	// Click the first row — htmx should swap the drawer in place.
	firstRow := page.Locator(".list-row__link").First()
	require.NoError(t, firstRow.Click())

	// Wait for the drawer head to appear (htmx afterSwap populates it).
	require.NoError(t, page.Locator(".drawer__head .drawer__title").WaitFor())

	// Drawer now has a non-empty title.
	titleText, err := page.Locator(".drawer__head .drawer__title").First().TextContent()
	require.NoError(t, err)
	assert.NotEmpty(t, titleText)

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
		t.Fatalf("%d axe violation(s) on /groups post-drawer-swap", len(ar.Violations))
	}
}
