//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHomeV2_VisibleAndAAA verifies the signed-in home page renders
// visibly, is AAA-clean per axe-core, and that the command palette
// opens when the user presses Ctrl+K.
//
// Mirrors the pattern of TestAxeAAA_LoginPage: inject axe-core via
// page.Evaluate (bypasses our script-src 'self' CSP) and run axe.run
// with the full WCAG AAA tag set.
func TestHomeV2_VisibleAndAAA(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	// Sign in as the seeded test user. Login posts the form and waits a
	// short beat; the handler then redirects to "/" on success.
	require.NoError(t, tp.LoginAsTestUser(), "sign in as seeded test user")

	tp.Navigate("/")

	// Visibility: the greeting must have a non-zero bounding box.
	box, err := page.Locator("h1.home__greet").BoundingBox()
	require.NoError(t, err, "home greeting bounding box")
	require.NotNil(t, box, "home greeting must have a layout box")
	assert.Greater(t, box.Height, 0.0, "home greeting height")
	assert.Greater(t, box.Width, 0.0, "home greeting width")

	// AAA axe-core pass. The vendored script is injected via Evaluate
	// (runs in an isolated world that is not subject to the page CSP).
	axePath, err := filepath.Abs("internal/e2e/testdata/axe.min.js")
	require.NoError(t, err, "resolve axe path")
	axeSrc, err := os.ReadFile(axePath)
	require.NoError(t, err, "read axe.min.js")

	_, err = page.Evaluate(string(axeSrc))
	require.NoError(t, err, "inject axe via evaluate")

	result, err := page.Evaluate(`
		() => axe.run(document, {
			runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag2aaa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
			resultTypes: ['violations'],
		})
	`)
	require.NoError(t, err, "axe.run")

	raw, err := json.Marshal(result)
	require.NoError(t, err, "marshal axe result")

	var ar axeResult
	require.NoError(t, json.Unmarshal(raw, &ar), "unmarshal axe result")

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
		t.Fatalf("%d axe violation(s) on /", len(ar.Violations))
	}

	fmt.Fprintln(os.Stderr, "axe AAA pass on /: 0 violations")

	// Palette opens on Ctrl+K. Use Control (not Meta) so the test is
	// consistent across Linux CI and local macOS dev boxes.
	require.NoError(t, page.Keyboard().Press("Control+k"), "press Ctrl+K")

	openAttr, err := page.Evaluate(`document.getElementById('cmd-palette').hasAttribute('open')`)
	require.NoError(t, err, "read palette open attribute")
	assert.Equal(t, true, openAttr, "palette <dialog> must have the open attribute after Ctrl+K")
}
