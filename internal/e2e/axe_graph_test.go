//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAxeAA_GraphPages asserts the Phase 3 graph view pages have zero
// WCAG 2.2 Level AA violations under axe-core. The graph view's stated
// conformance level is AA (not AAA — the dynamic SVG/JS interactions
// don't clear the AAA bar; the AAA-equivalent flat edge table below
// the canvas provides the text alternative). The login page has a
// separate AAA ratchet in TestAxeAAA_LoginPage. Covers:
//   - /graph?entity=<seeded-DN>&depth=2 (entity-focused mode)
//   - /users?view=graph                 (list-page Graph mode)
//   - /groups?view=graph
//   - /computers?view=graph
//
// Authenticated as the test user; uses the first user from /users to
// pick a real DN for the entity-focused page (matches the existing
// graph_test.go pattern).
func TestAxeAA_GraphPages(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsTestUser())

	// Discover a real seeded user DN by visiting /users and reading the
	// first row's href (mirrors graph_test.go).
	tp.Navigate("/users")
	require.NoError(t, page.Locator(".list-row__link").First().WaitFor())

	firstRowHref, err := page.Locator(".list-row__link").First().GetAttribute("href")
	require.NoError(t, err)
	require.NotEmpty(t, firstRowHref)
	require.True(t, strings.HasPrefix(firstRowHref, "/users/"), "unexpected href: %s", firstRowHref)
	dn, err := url.PathUnescape(strings.TrimPrefix(firstRowHref, "/users/"))
	require.NoError(t, err)
	require.Contains(t, dn, "cn=", "expected a CN-based DN")

	pages := []string{
		"/graph?entity=" + url.QueryEscape(dn) + "&depth=2",
		"/users?view=graph",
		"/groups?view=graph",
		"/computers?view=graph",
	}

	axePath, err := filepath.Abs("internal/e2e/testdata/axe.min.js")
	require.NoError(t, err, "resolve axe path")
	axeSrc, err := os.ReadFile(axePath)
	require.NoError(t, err, "read axe.min.js")

	for _, target := range pages {
		t.Run(target, func(t *testing.T) {
			tp.Navigate(target)
			// Wait for the graph canvas to appear so axe sees the
			// fully-rendered page (v2-graph.js mutates DOM on load).
			require.NoError(t, page.Locator("svg#graph-canvas").WaitFor())

			_, err := page.Evaluate(string(axeSrc))
			require.NoError(t, err, "inject axe")

			result, err := page.Evaluate(`
				() => axe.run(document, {
					runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
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
					t.Errorf("axe violation [%s] on %s: %s — %d node(s). help: %s",
						v.ID, target, v.Description, len(v.Nodes), v.HelpURL)
					for i, n := range v.Nodes {
						if i >= 3 {
							t.Logf("  (and %d more node(s) truncated)", len(v.Nodes)-i)

							break
						}
						t.Logf("  %s", n.Target)
					}
				}
				t.Fatalf("%d axe violation(s) on %s", len(ar.Violations), target)
			}

			fmt.Fprintf(os.Stderr, "axe AA pass on %s: 0 violations\n", target)
		})
	}
}
