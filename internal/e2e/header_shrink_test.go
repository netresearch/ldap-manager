//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHeaderDoesNotShrinkWithLongLists is a regression guard for the
// user-reported bug: the topnav and topnav-secondary bars got visibly
// denser on pages with long lists (>200 users/groups/computers).
//
// Root cause:
//
//	<body> is `display: flex; flex-direction: column`. Its three
//	children — topnav, topnav-secondary, list-page — all default to
//	`flex-shrink: 1`. When list-page's intrinsic content overflows
//	the viewport (202 rows × ~44px ≈ 8900px), the flex algorithm
//	distributes shrinkage across all siblings. topnav had
//	`min-height: var(--density-touch-size)` = 33.75px. Because
//	Pico ships box-sizing:border-box globally, that min-height
//	INCLUDES the 0.5rem vertical padding, so under pressure the
//	topnav collapsed from its natural ~50px down to 33.75px — the
//	padding visually disappeared and the logo/cmdk button ate the
//	whole row.
//
// Fix: `flex-shrink: 0` on `.topnav` and `.topnav-secondary` (see
// app.css). list-page already has `min-height: 0` and an inner
// scrolling pane, so it absorbs the entire overflow on its own.
//
// This test asserts the header height on /users (202 seeded rows) is
// the SAME as on / (no list) at the same viewport — the invariant
// that was violated before the fix.
func TestHeaderDoesNotShrinkWithLongLists(t *testing.T) {
	cfg := DefaultTestConfig()
	tb := NewTestBrowser(t, cfg)
	defer tb.Close()

	viewports := []struct {
		name          string
		width, height int
	}{
		{"1280x900", 1280, 900},
		{"1280x700", 1280, 700},
		{"1280x480", 1280, 480},
	}

	for _, vp := range viewports {
		t.Run(vp.name, func(t *testing.T) {
			// --- Control: home page, no list, same viewport. ---
			homeTopnav, homeSecondary := measureHeader(t, tb, cfg, "/", vp.width, vp.height)

			// --- Stress: /users with 202 seeded rows. ---
			listTopnav, listSecondary := measureHeader(t, tb, cfg, "/users", vp.width, vp.height)

			// Sanity: rows must actually overflow (otherwise the bug
			// conditions are not met and the test would pass for the
			// wrong reason).
			assertListOverflows(t, tb, cfg, "/users", vp.width, vp.height)

			assert.Equal(t, homeTopnav, listTopnav,
				"topnav height must be identical on / vs /users at %s — "+
					"a smaller value on /users means body-flex is shrinking it "+
					"(regression: remove `flex-shrink: 0` from .topnav)",
				vp.name)

			assert.Equal(t, homeSecondary, listSecondary,
				"topnav-secondary height must be identical on / vs /users at %s — "+
					"a smaller value on /users means body-flex is shrinking it "+
					"(regression: remove `flex-shrink: 0` from .topnav-secondary)",
				vp.name)

			// Additional guard: the topnav must be at least tall enough
			// to contain the cmdk button + its natural padding. Anything
			// below 40px is the bug regardless of the home/list compare.
			assert.GreaterOrEqual(t, listTopnav, 40,
				"topnav on /users at %s must be at least 40px (natural cmdk button + padding); "+
					"got %dpx — flex-shrink regression", vp.name, listTopnav)
		})
	}
}

// measureHeader navigates to `path` at the given viewport and returns
// the topnav and topnav-secondary offsetHeight values in pixels.
func measureHeader(t *testing.T, tb *TestBrowser, cfg TestConfig, path string, width, height int) (int, int) {
	t.Helper()

	page, err := tb.browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: width, Height: height},
	})
	require.NoError(t, err)
	defer page.Close()

	tp := NewTestPage(t, page, cfg)
	require.NoError(t, tp.LoginAsAdmin())

	tp.Navigate(path)
	require.NoError(t, tp.WaitForSelector(".topnav"))
	require.NoError(t, page.WaitForLoadState())

	raw, err := page.Evaluate(`() => [
		document.querySelector('.topnav')?.offsetHeight ?? -1,
		document.querySelector('.topnav-secondary')?.offsetHeight ?? -1,
	]`)
	require.NoError(t, err)

	arr := raw.([]interface{})
	return intOf(arr[0]), intOf(arr[1])
}

// assertListOverflows verifies the precondition that list-rows
// scrollHeight actually exceeds its offsetHeight — i.e. the list IS
// long enough to put flex pressure on the body. Without this the
// header compare is not exercising the bug.
func assertListOverflows(t *testing.T, tb *TestBrowser, cfg TestConfig, path string, width, height int) {
	t.Helper()

	page, err := tb.browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{Width: width, Height: height},
	})
	require.NoError(t, err)
	defer page.Close()

	tp := NewTestPage(t, page, cfg)
	require.NoError(t, tp.LoginAsAdmin())

	tp.Navigate(path)
	require.NoError(t, tp.WaitForSelector(".list-rows"))

	raw, err := page.Evaluate(`() => {
		const el = document.querySelector('.list-rows');
		return {offset: el.offsetHeight, scroll: el.scrollHeight};
	}`)
	require.NoError(t, err)
	m := raw.(map[string]interface{})
	assert.Greater(t, intOf(m["scroll"]), intOf(m["offset"])+100,
		"precondition: .list-rows must overflow its box (scrollHeight > offsetHeight+100) "+
			"for this test to exercise the flex-shrink bug; got scroll=%v offset=%v",
		m["scroll"], m["offset"])
}

// intOf coerces a JSON number (int or float64) to int.
func intOf(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return -1
}
