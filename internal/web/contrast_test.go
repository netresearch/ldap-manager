// internal/web/contrast_test.go
package web

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestAppCSSContrastAAA verifies every declared text/background pair in
// app.css meets WCAG 2.2 AAA contrast (≥7:1 for normal text). Tokens are
// read straight from the file so drift is caught automatically.
func TestAppCSSContrastAAA(t *testing.T) {
	path := filepath.Join("static", "app.css")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	light := parseTokens(string(raw), "")    // :root
	dark := parseTokens(string(raw), "dark") // :root[data-theme="dark"]

	// dark inherits anything not overridden
	for k, v := range light {
		if _, ok := dark[k]; !ok {
			dark[k] = v
		}
	}

	pairs := []struct {
		theme  string
		fg, bg string
		min    float64 // 7.0 for AAA normal text; 4.5 for non-essential
	}{
		{"light", "--fg", "--bg", 7.0},
		{"light", "--fg-muted", "--bg", 7.0},
		{"light", "--fg", "--bg-subtle", 7.0},
		{"light", "--fg-muted", "--bg-subtle", 7.0},
		{"light", "--accent-fg", "--accent", 7.0},
		{"dark", "--fg", "--bg", 7.0},
		{"dark", "--fg-muted", "--bg", 7.0},
		{"dark", "--fg", "--bg-subtle", 7.0},
		{"dark", "--fg-muted", "--bg-subtle", 7.0},
		{"dark", "--accent-fg", "--accent", 7.0},
		{"dark", "--accent", "--bg", 7.0}, // accent-as-text on dark bg (headings)
	}

	for _, p := range pairs {
		tokens := light
		if p.theme == "dark" {
			tokens = dark
		}
		fg, ok := tokens[p.fg]
		if !ok {
			t.Errorf("%s: token %s not found", p.theme, p.fg)
			continue
		}
		bg, ok := tokens[p.bg]
		if !ok {
			t.Errorf("%s: token %s not found", p.theme, p.bg)
			continue
		}
		ratio := contrast(fg, bg)
		if ratio < p.min {
			t.Errorf("%s: %s on %s = %.2f (need ≥%.1f)",
				p.theme, p.fg, p.bg, ratio, p.min)
		}
	}
}

var (
	// Tightened to the two hex forms parseHex accepts (#RGB, #RRGGBB).
	// Previously the pattern allowed 3-8 hex digits, which would quietly
	// swallow #RRGGBBAA tokens: they'd match here but fail parseHex and
	// then surface as "token not found" later in the test. Either extend
	// parseHex for alpha or tighten the regex; chose the latter because
	// we don't use alpha tokens anywhere in app.css.
	reTokenLine = regexp.MustCompile(`^\s*(--[a-z][a-z0-9-]*)\s*:\s*(#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}))\s*;`)
	reDarkBlock = regexp.MustCompile(`(?s):root\[data-theme="dark"\]\s*\{(.*?)\}`)
	// Match ":root { ... }" (but not ":root[..." selectors). Non-greedy
	// body so the first closing "}" ends the block. The file may contain
	// multiple :root blocks (colours + density); we search each and merge.
	reLightBlock = regexp.MustCompile(`(?s):root\s*\{(.*?)\}`)
)

func parseTokens(css, theme string) map[string][3]int {
	var blocks []string
	switch theme {
	case "dark":
		m := reDarkBlock.FindStringSubmatch(css)
		if len(m) == 2 {
			blocks = append(blocks, m[1])
		}
	default:
		// Merge every plain ":root { ... }" block (colour tokens +
		// density tokens live in separate :root blocks).
		for _, m := range reLightBlock.FindAllStringSubmatch(css, -1) {
			if len(m) == 2 {
				blocks = append(blocks, m[1])
			}
		}
	}

	out := map[string][3]int{}
	for _, block := range blocks {
		for _, line := range strings.Split(block, "\n") {
			m := reTokenLine.FindStringSubmatch(line)
			if len(m) != 3 {
				continue
			}
			if rgb, ok := parseHex(m[2]); ok {
				out[m[1]] = rgb
			}
		}
	}
	return out
}

func parseHex(h string) ([3]int, bool) {
	h = strings.TrimPrefix(h, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return [3]int{}, false
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.ParseInt(h[i*2:i*2+2], 16, 0)
		if err != nil {
			return [3]int{}, false
		}
		v[i] = int(n)
	}
	return v, true
}

// relativeLuminance per WCAG 2.x.
func relativeLuminance(rgb [3]int) float64 {
	f := func(c int) float64 {
		x := float64(c) / 255.0
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(rgb[0]) + 0.7152*f(rgb[1]) + 0.0722*f(rgb[2])
}

func contrast(a, b [3]int) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
