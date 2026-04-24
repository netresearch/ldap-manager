//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAxeAAA_LoginPage asserts the /login page has zero WCAG 2.2 AAA
// violations according to axe-core. Blocks merge if any violation.
func TestAxeAAA_LoginPage(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	tp.Navigate("/login")

	axePath, err := filepath.Abs("internal/e2e/testdata/axe.min.js")
	require.NoError(t, err, "resolve axe path")
	axeSrc, err := os.ReadFile(axePath)
	require.NoError(t, err, "read axe.min.js")

	// Inject axe via page.Evaluate (runs in an isolated context that bypasses
	// the page's Content-Security-Policy). AddScriptTag with Content would be
	// blocked by our script-src 'self' CSP.
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
		t.Fatalf("%d axe violation(s) on /login", len(ar.Violations))
	}

	fmt.Fprintln(os.Stderr, "axe AAA pass on /login: 0 violations")
}

type axeResult struct {
	Violations []axeViolation `json:"violations"`
}

type axeViolation struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	HelpURL     string    `json:"helpUrl"`
	Nodes       []axeNode `json:"nodes"`
}

type axeNode struct {
	Target []string `json:"target"`
}
