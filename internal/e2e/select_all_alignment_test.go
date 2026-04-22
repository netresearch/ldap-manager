//go:build e2e

package e2e

import (
	"math"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSelectAllAlignsWithSearch asserts the "[x] all" checkbox chip
// and the search input are visually aligned along the same Y-center.
//
// User-reported regression: the chip sat ~2.8 px above the search
// input on /users and /groups. Root cause: Pico CSS ships a global
//
//	label { margin-bottom: calc(var(--pico-spacing) * .375) }
//
// for its "form label above input" convention. Our chip is a <label>
// used INLINE in a flex filter row. That 5.625 px margin-bottom
// contributes to the flex line cross-size (making the row 39.375 px
// instead of 33.75 px), and flex centering places the label's box at
// the row top with the margin consumed below — shifting its y-center
// 2.8125 px above the search input.
//
// Fix: `margin-bottom: 0` on .list-page__select-all and
// .list-row__check-wrap (app.css). This test locks that invariant.
//
// We compare the Y-centers on both /users and /groups because both
// list pages render the same filter row markup.
func TestSelectAllAlignsWithSearch(t *testing.T) {
	cfg := DefaultTestConfig()
	tb := NewTestBrowser(t, cfg)
	defer tb.Close()

	cases := []string{"/users", "/groups"}

	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			page, err := tb.browser.NewPage(playwright.BrowserNewPageOptions{
				Viewport: &playwright.Size{Width: 1280, Height: 900},
			})
			require.NoError(t, err)
			defer page.Close()

			tp := NewTestPage(t, page, cfg)
			require.NoError(t, tp.LoginAsAdmin())

			tp.Navigate(path)
			require.NoError(t, tp.WaitForSelector(".list-page__select-all"))
			require.NoError(t, tp.WaitForSelector(".list-page__search"))

			selectY := boundingYCenter(t, page, ".list-page__select-all")
			searchY := boundingYCenter(t, page, ".list-page__search")

			delta := math.Abs(selectY - searchY)
			assert.LessOrEqual(t, delta, 1.0,
				"select-all chip Y-center (%g) must match search input Y-center (%g) within 1 px; "+
					"delta=%g. Likely cause: Pico's default label { margin-bottom } is no longer "+
					"being reset on .list-page__select-all.",
				selectY, searchY, delta)

			// Heights must also be equal — a size mismatch is a bug
			// in its own right even if flex centering happens to hide it.
			selectH := boundingHeight(t, page, ".list-page__select-all")
			searchH := boundingHeight(t, page, ".list-page__search")
			assert.Equal(t, selectH, searchH,
				"select-all chip and search input must be the same height; got chip=%v search=%v",
				selectH, searchH)
		})
	}
}

func boundingYCenter(t *testing.T, page playwright.Page, sel string) float64 {
	t.Helper()
	raw, err := page.Evaluate(`(s) => {
		const el = document.querySelector(s);
		if (!el) return -1;
		const r = el.getBoundingClientRect();
		return r.top + r.height / 2;
	}`, sel)
	require.NoError(t, err)
	return toFloat(raw)
}

func boundingHeight(t *testing.T, page playwright.Page, sel string) float64 {
	t.Helper()
	raw, err := page.Evaluate(`(s) => {
		const el = document.querySelector(s);
		if (!el) return -1;
		return el.getBoundingClientRect().height;
	}`, sel)
	require.NoError(t, err)
	return toFloat(raw)
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return -1
}
