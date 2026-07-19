//go:build e2e

package e2e

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNarrowViewport_RowClickShowsDrawer guards against the regression
// where the list + drawer pane hid the drawer at ≤900 px viewports while
// still htmx-swapping content into it — user clicks, server responds,
// but nothing visibly changes.
//
// Covers all three list pages (users, groups, computers) at the narrow
// breakpoint (768 px).
func TestNarrowViewport_RowClickShowsDrawer(t *testing.T) {
	cases := []struct {
		name, listPath string
	}{
		{"users", "/users"},
		{"groups", "/groups"},
		{"computers", "/computers"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultTestConfig()
			browser := NewTestBrowser(t, config)
			defer browser.Close()

			// Narrow mobile-ish viewport. Intentionally below the 900 px
			// breakpoint at which the CSS grid collapses to a single column.
			page, err := browser.browser.NewPage(playwright.BrowserNewPageOptions{
				Viewport: &playwright.Size{Width: 768, Height: 1024},
			})
			require.NoError(t, err)
			defer page.Close()
			tp := NewTestPage(t, page, config)

			require.NoError(t, tp.LoginAsTestUser())
			tp.Navigate(tc.listPath)

			// The empty-state message before any row is clicked.
			emptyText := page.Locator(".drawer__empty").First()

			rowCount, _ := page.Locator(".list-row__link").Count()
			if rowCount == 0 {
				t.Skipf("no seeded %s in fixture; skipping narrow-viewport path", tc.name)
			}

			// Click the first row.
			require.NoError(t, page.Locator(".list-row__link").First().Click())

			// htmx must swap in a .drawer__title and the drawer must be
			// visible (not display:none). We wait on the title rather than
			// asserting styles so the test also verifies htmx completed.
			drawerTitle := page.Locator(".drawer__head .drawer__title").First()
			require.NoError(t, drawerTitle.WaitFor(playwright.LocatorWaitForOptions{
				State: playwright.WaitForSelectorStateVisible,
			}), "drawer title must become visible after row click on narrow viewport")

			// The empty-state placeholder was replaced.
			emptyStillVisible, _ := emptyText.IsVisible()
			assert.False(t, emptyStillVisible, "original drawer empty-state should have been replaced")

			// The drawer element itself must not be display:none — otherwise
			// we just load HTML the user can't see.
			display, err := page.Evaluate(`
				() => getComputedStyle(document.querySelector('#drawer')).display
			`)
			require.NoError(t, err)
			assert.NotEqual(t, "none", display, "drawer must not be display:none at 768px after row click")

			// Title actually has text.
			txt, _ := drawerTitle.TextContent()
			assert.NotEmpty(t, txt, "drawer title must have populated content")
		})
	}
}
