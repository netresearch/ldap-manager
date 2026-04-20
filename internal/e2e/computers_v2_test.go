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

// TestComputersV2_FlowAndAAA covers the happy path for the V2 computers list:
// list loads with a visible H1, and axe-core reports zero WCAG 2.2 AAA
// violations. When the fixture has at least one computer the test also
// verifies that clicking the first row swaps the drawer via htmx (populating
// the title) before re-running axe. If the e2e fixture has no seeded
// computers the drawer path is skipped but the list-page AAA check still
// runs.
func TestComputersV2_FlowAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())

	tp.Navigate("/computers")

	// H1 title is present and has a visible bounding box.
	require.NoError(t, page.Locator("h1.list-page__title").WaitFor())
	titleBox, err := page.Locator("h1.list-page__title").BoundingBox()
	require.NoError(t, err)
	require.NotNil(t, titleBox)
	assert.Greater(t, titleBox.Height, 0.0)

	// If there are seeded computers, click the first row to trigger the
	// drawer swap so AAA runs over the post-swap view. Otherwise keep the
	// empty-state list page and AAA over that.
	rowCount, _ := page.Locator(".list-row__link").Count()
	if rowCount > 0 {
		firstRow := page.Locator(".list-row__link").First()
		require.NoError(t, firstRow.Click())

		// Wait for the drawer head to appear (htmx afterSwap populates it).
		require.NoError(t, page.Locator(".drawer__head .drawer__title").WaitFor())

		titleText, err := page.Locator(".drawer__head .drawer__title").First().TextContent()
		require.NoError(t, err)
		assert.NotEmpty(t, titleText)
	} else {
		t.Log("no seeded computers in e2e fixture — running AAA on empty list view")
	}

	// axe AAA pass (list with zero rows still needs to be clean).
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
		t.Fatalf("%d axe violation(s) on /computers", len(ar.Violations))
	}
}
