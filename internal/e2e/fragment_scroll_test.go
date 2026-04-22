//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFragmentURL_DirectNavIsScrollable guards against the regression
// the user reported: F5 on a `/users/:dn?fragment=drawer` URL served
// the full-page template correctly (that bug was already fixed) but the
// page itself had no scrollbar — the list-page shell's
// `html { height: 100dvh; overflow: hidden }` swallowed the document
// scroller, so content below the fold was unreachable.
//
// Fix: `html:has(> body.has-page-scroll) { height: auto; overflow: visible }`
// in app.css. When the full-page detail template sets has-page-scroll
// on body, the <html> element now releases its own clip.
//
// We assert the *computed* html overflow because it is the direct
// invariant that fixes the bug. A JS-only check (scrollY > 0 after
// scrollTo) is unreliable: browsers propagate body overflow:visible to
// the viewport and keep scrollTop updatable even when html is clipped
// — the APIs lie about scroll state, but the scrollbar really is gone.
func TestFragmentURL_DirectNavIsScrollable(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	// Intentionally short viewport so the detail page overflows and a
	// real scrollbar would be needed.
	page, err := browser.browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: 1280, Height: 500},
	})
	require.NoError(t, err)
	defer page.Close()

	tp := NewTestPage(t, page, config)
	require.NoError(t, tp.LoginAsAdmin())

	// Directly navigate to a fragment URL (mimics F5 / copy-paste).
	tp.Navigate("/users/cn=testuser1,ou=users,dc=example,dc=com?fragment=drawer")

	require.NoError(t, tp.WaitForSelector(".drawer--full"),
		"drawer--full must render on direct fragment URL")

	// Sanity: body must have the has-page-scroll opt-in, otherwise this
	// test is not exercising the rule we care about.
	bodyClass, err := page.Evaluate(`() => document.body.className`)
	require.NoError(t, err)
	assert.Contains(t, bodyClass.(string), "has-page-scroll",
		"full-page detail template should set body.has-page-scroll — otherwise "+
			"the `html:has(> body.has-page-scroll)` rule cannot apply")

	// The actual invariant: <html> must not be clipping at the viewport
	// box when body opted into page scroll. Without the `:has()` rule the
	// list-page shell's html {height:100dvh; overflow:hidden} swallows
	// the scrollbar, and content below the fold is unreachable.
	htmlOverflowY, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).overflowY`,
	)
	require.NoError(t, err)
	assert.NotEqual(t, "hidden", htmlOverflowY,
		"html overflow must not be clipped when body.has-page-scroll is set — "+
			"regression swallows the page scrollbar (user-reported F5 on fragment URL)")

	htmlHeight, err := page.Evaluate(
		`() => getComputedStyle(document.documentElement).height`,
	)
	require.NoError(t, err)
	// When the fix is in place, html's height computes to "auto" or to
	// the document's content height — not the clipped 100dvh. Accept
	// anything that is NOT exactly the viewport height (500px).
	assert.NotEqual(t, "500px", htmlHeight,
		"html height must not be clipped to the viewport when body.has-page-scroll is set; "+
			"got %q which matches the `height: 100dvh` clip that swallowed the scrollbar", htmlHeight)
}
