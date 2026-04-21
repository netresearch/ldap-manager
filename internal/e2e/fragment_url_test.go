//go:build e2e

package e2e

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFragmentURL_DirectNavRendersFullPage is a regression guard for a class
// of bugs where a direct navigation (browser F5, copy/paste, bookmarks) to a
// drawer-fragment URL like `/users/:dn?fragment=drawer` served only the bare
// fragment (no <html>, no <head>, no topnav, no stylesheets) — a usable but
// unstyled "just the header" view.
//
// The fix: handleUserV2/handleGroupV2/handleComputerV2 only honour
// ?fragment=drawer when the request carries HX-Request:true. A plain GET
// without the header now falls through to the full-page template.
//
// The earlier TestUsersV2_FlowAndAAA et al. did not catch this because they
// all exercise the htmx happy-path: list page → click row → drawer swap (htmx
// DOES add HX-Request). They never cover the direct-URL / F5 case where no
// HX-Request header is present.
//
// This test hits each of /users/:dn?fragment=drawer, /groups/:dn?...,
// /computers/:dn?... directly (no htmx), and asserts the response contains a
// full-page shell (html + body + topnav + drawer__title) rather than a bare
// fragment.
func TestFragmentURL_DirectNavRendersFullPage(t *testing.T) {
	config := DefaultTestConfig()

	// Use a net/http client with a cookie jar rather than Playwright so we
	// can inspect the raw HTML response byte-for-byte and prove the absence
	// of the bare-fragment markup. Playwright would happily render the
	// fragment and we would miss the regression.
	jar, err := cookiejar.New(nil)
	require.NoError(t, err, "new cookie jar")

	client := &http.Client{
		Jar: jar,
		// Don't auto-follow redirects so we can drive the login flow.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: GET /login to establish a session + CSRF cookie and extract
	// the form csrf_token value.
	csrfToken := getCSRFToken(t, client, config.BaseURL+"/login")

	// Step 2: POST credentials with csrf_token.
	loginForm := url.Values{}
	loginForm.Set("username", config.TestUser)
	loginForm.Set("password", config.TestUserPass)
	loginForm.Set("csrf_token", csrfToken)

	req, err := http.NewRequest(http.MethodPost, config.BaseURL+"/login",
		strings.NewReader(loginForm.Encode()))
	require.NoError(t, err, "build login POST")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	require.NoError(t, err, "login POST")
	_ = resp.Body.Close()
	// Login success is a 302/303 to / (or wherever); treat anything <400 as ok.
	require.Less(t, resp.StatusCode, 400, "login should succeed (got %d)", resp.StatusCode)

	// Step 3: For each entity kind, pick a real DN off the list page and
	// directly GET /…/:dn?fragment=drawer without HX-Request.
	cases := []struct {
		name       string
		listPath   string
		drawerCSS  string // text to grep for in the full page render
		topnavText string // topnav link text that should always appear
	}{
		{
			name:       "users",
			listPath:   "/users",
			drawerCSS:  "drawer__title",
			topnavText: "Users",
		},
		{
			name:       "groups",
			listPath:   "/groups",
			drawerCSS:  "drawer__title",
			topnavText: "Groups",
		},
		{
			name:       "computers",
			listPath:   "/computers",
			drawerCSS:  "drawer__title",
			topnavText: "Computers",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dn, found := firstRowDN(t, client, config.BaseURL+tc.listPath)
			if !found {
				t.Skipf("no seeded %s in fixture — skipping", tc.name)
			}

			// Direct F5-style GET of the fragment URL, NO HX-Request header.
			fragURL := config.BaseURL + tc.listPath + "/" +
				url.PathEscape(dn) + "?fragment=drawer"

			req, err := http.NewRequest(http.MethodGet, fragURL, nil)
			require.NoError(t, err, "build fragment GET")

			resp, err := client.Do(req)
			require.NoError(t, err, "fragment GET")
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			require.NoError(t, err, "read fragment response")

			require.Equal(t, http.StatusOK, resp.StatusCode,
				"fragment URL should render (body head: %s)", head(body, 256))

			html := string(body)

			// Full-page markers: the shell must be present.
			assert.Contains(t, html, "<html", "response should be a full page (missing <html)")
			assert.Contains(t, html, "<body", "response should be a full page (missing <body)")
			assert.Contains(t, html, "topnav", "response should contain the topnav")
			assert.Contains(t, html, "/static/app.css",
				"response should link the stylesheet (proof of full <head>)")

			// Drawer contents must also be rendered — the whole point of the
			// URL is that the selected entity is visible.
			assert.Contains(t, html, tc.drawerCSS,
				"response should include the drawer title element")

			// Topnav link text.
			assert.Contains(t, html, tc.topnavText,
				"response should include topnav label %q", tc.topnavText)

			// Negative assertion: a bare fragment would NOT start with
			// <!DOCTYPE or contain <html — the bug we are guarding against.
			// Match case-insensitively since templ emits <!doctype html>.
			trimmed := strings.TrimSpace(html)
			lower := strings.ToLower(trimmed)
			assert.True(t,
				strings.HasPrefix(lower, "<!doctype") ||
					strings.HasPrefix(lower, "<html"),
				"response should start with full-page doctype, got: %s",
				head([]byte(trimmed), 80))
		})
	}
}

// getCSRFToken fetches `loginURL` and scrapes the hidden csrf_token field
// out of the rendered login form. Fails the test if the cookie jar stays
// empty (sign that middleware did not run) or if no token is found.
func getCSRFToken(t *testing.T, client *http.Client, loginURL string) string {
	t.Helper()

	resp, err := client.Get(loginURL)
	require.NoError(t, err, "GET login page")

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err, "read login page")

	require.Equal(t, http.StatusOK, resp.StatusCode, "login page status")

	// Scan for name="csrf_token" value="…".
	html := string(body)
	marker := `name="csrf_token"`
	i := strings.Index(html, marker)
	require.GreaterOrEqual(t, i, 0, "csrf_token input not found on login page")

	// Look backwards OR forwards for value="…".
	tail := html[i:]
	vIdx := strings.Index(tail, `value="`)
	require.GreaterOrEqual(t, vIdx, 0, "csrf_token value= not found")

	rest := tail[vIdx+len(`value="`):]
	end := strings.Index(rest, `"`)
	require.GreaterOrEqual(t, end, 0, "csrf_token value closing quote not found")

	token := rest[:end]
	require.NotEmpty(t, token, "csrf_token should be non-empty")

	return token
}

// firstRowDN fetches the list page and extracts the DN of the first row by
// looking at the href pattern `/<entity>/<url-escaped-dn>`. Returns
// (dn, true) on success, ("", false) when the list has no rows.
func firstRowDN(t *testing.T, client *http.Client, listURL string) (string, bool) {
	t.Helper()

	resp, err := client.Get(listURL)
	require.NoError(t, err, "GET list page %s", listURL)

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, err, "read list page")
	require.Equal(t, http.StatusOK, resp.StatusCode, "list page status")

	// Look for the first list-row__link href. The detail URL path segment
	// is whatever comes between /<entity>/ and ? or " .
	prefix, err := extractListPathPrefix(listURL)
	require.NoError(t, err, "extract list path prefix")

	html := string(body)
	marker := `href="` + prefix + "/"
	i := strings.Index(html, marker)
	if i < 0 {
		return "", false
	}

	rest := html[i+len(marker):]
	end := strings.IndexAny(rest, `"?`)
	if end < 0 {
		return "", false
	}

	escaped := rest[:end]
	dn, err := url.PathUnescape(escaped)
	require.NoError(t, err, "unescape DN %q", escaped)

	return dn, true
}

// extractListPathPrefix pulls `/users` out of `http://host:port/users`.
func extractListPathPrefix(listURL string) (string, error) {
	u, err := url.Parse(listURL)
	if err != nil {
		return "", err
	}

	return u.Path, nil
}

// head returns the first n bytes of b as a string for error messages.
func head(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}

	return string(b[:n]) + "…"
}
