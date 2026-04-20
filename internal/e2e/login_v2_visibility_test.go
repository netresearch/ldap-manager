//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginV2_VisibleAndInteractive asserts that the new /login page is not
// only DOM-correct (covered by TestAxeAAA_LoginPage) but also visually
// rendered and interactive. This would have caught the CSP-induced white
// page: the login form was present in markup but hidden by an orphan
// x-cloak + display:none rule because Alpine could not initialise.
func TestLoginV2_VisibleAndInteractive(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	// Fail the test on any CSP or script error — these silently break
	// the page without invalidating the DOM.
	var consoleErrors []string
	page.On("pageerror", func(err error) {
		consoleErrors = append(consoleErrors, err.Error())
	})

	tp.Navigate("/login")

	// The login form must render with a non-zero bounding box.
	card, err := page.Locator("form.login-card").BoundingBox()
	require.NoError(t, err, "login-card bounding box")
	require.NotNil(t, card, "login-card must have a layout box")
	assert.Greater(t, card.Height, 0.0, "login-card height")
	assert.Greater(t, card.Width, 0.0, "login-card width")

	// <html> must be display:block (not hidden by an orphan x-cloak).
	display, err := page.Evaluate("getComputedStyle(document.documentElement).display")
	require.NoError(t, err, "evaluate html display")
	assert.Equal(t, "block", display, "html must not be display:none")

	// Theme toggle must swap data-theme on click.
	initialTheme, _ := page.Evaluate("document.documentElement.getAttribute('data-theme')")
	require.NoError(t, page.Click("[data-toggle=theme]"), "click theme toggle")
	newTheme, _ := page.Evaluate("document.documentElement.getAttribute('data-theme')")
	assert.NotEqual(t, initialTheme, newTheme, "theme toggle must change data-theme")

	// Density toggle must swap data-density on click.
	initialDensity, _ := page.Evaluate("document.documentElement.getAttribute('data-density')")
	require.NoError(t, page.Click("[data-toggle=density]"), "click density toggle")
	newDensity, _ := page.Evaluate("document.documentElement.getAttribute('data-density')")
	assert.NotEqual(t, initialDensity, newDensity, "density toggle must change data-density")

	// No page errors (CSP, eval, undefined global, etc.) during the test.
	assert.Empty(t, consoleErrors, "no page errors during login flow")
}
