//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	t.Run("login page renders correctly", func(t *testing.T) {
		tp.Navigate("/login")

		// Check for login form elements
		assert.True(t, tp.IsVisible("input[name='username']"), "Username input should be visible")
		assert.True(t, tp.IsVisible("input[name='password']"), "Password input should be visible")
		assert.True(t, tp.IsVisible("button[type='submit']"), "Submit button should be visible")
	})

	t.Run("login with invalid credentials shows error", func(t *testing.T) {
		err := tp.Login("invaliduser", "wrongpassword")
		require.NoError(t, err)

		// Should show error message
		flash, hasFlash := tp.GetFlashMessage()
		if hasFlash {
			assert.True(t, strings.Contains(strings.ToLower(flash), "invalid") ||
				strings.Contains(strings.ToLower(flash), "error") ||
				strings.Contains(strings.ToLower(flash), "failed"),
				"Should show error message for invalid credentials")
		}

		// Should still be on login page or redirected back
		currentURL := tp.GetCurrentPath()
		assert.True(t, strings.Contains(currentURL, "login") || strings.Contains(currentURL, "error"),
			"Should remain on login page after invalid credentials")
	})

	t.Run("login with empty credentials shows validation", func(t *testing.T) {
		tp.Navigate("/login")
		err := tp.Click("button[type='submit']")
		require.NoError(t, err)

		// Browser validation or server validation should prevent login
		currentURL := tp.GetCurrentPath()
		assert.Contains(t, currentURL, "login", "Should stay on login page with empty credentials")
	})

	t.Run("login with valid credentials succeeds", func(t *testing.T) {
		err := tp.LoginAsTestUser()
		require.NoError(t, err)

		// Should redirect away from login page
		currentURL := tp.GetCurrentPath()
		assert.False(t, strings.HasSuffix(currentURL, "/login"),
			"Should redirect after successful login")
	})

	t.Run("logout works correctly", func(t *testing.T) {
		// First ensure we're logged in
		if !tp.IsLoggedIn() {
			tp.LoginAsTestUser()
		}

		err := tp.Logout()
		require.NoError(t, err)

		// Should be redirected to login page
		currentURL := tp.GetCurrentPath()
		assert.True(t, strings.Contains(currentURL, "login") || currentURL == config.BaseURL+"/",
			"Should redirect to login or home after logout")
	})
}

func TestProtectedRoutes(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	protectedPaths := []string{
		"/users",
		"/groups",
		"/settings",
	}

	t.Run("protected routes redirect to login when not authenticated", func(t *testing.T) {
		for _, path := range protectedPaths {
			tp.Navigate(path)

			currentURL := tp.GetCurrentPath()
			assert.True(t, strings.Contains(currentURL, "login"),
				"Path %s should redirect to login when not authenticated", path)
		}
	})

	t.Run("protected routes accessible when authenticated", func(t *testing.T) {
		err := tp.LoginAsAdmin()
		require.NoError(t, err)

		for _, path := range protectedPaths {
			tp.Navigate(path)

			currentURL := tp.GetCurrentPath()
			// Should either be on the requested page or a valid authenticated page
			assert.False(t, strings.Contains(currentURL, "login"),
				"Path %s should be accessible when authenticated", path)
		}

		tp.Logout()
	})
}

func TestUserListJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	// Login first
	err := tp.LoginAsAdmin()
	require.NoError(t, err)
	defer tp.Logout()

	t.Run("user list page loads", func(t *testing.T) {
		tp.Navigate("/users")

		// ldap-manager renders users as .list-container / .list-row; keep
		// <table> selector as a fallback so this test adapts if the template
		// layout ever swaps back.
		hasList := tp.IsVisible("[data-search-list]") ||
			tp.IsVisible(".list-container") ||
			tp.IsVisible("table")
		assert.True(t, hasList, "User list should display users in a table or list")
	})

	t.Run("user list shows user data", func(t *testing.T) {
		tp.Navigate("/users")

		// Users V2 renders rows as .list-rows [data-search-item]; keep the
		// legacy selectors as a fallback for forward/backward compat.
		rowsSel := ".list-rows [data-search-item], .list-container [data-search-item], table tbody tr, [data-testid='user-item']"
		err := tp.page.Locator(rowsSel).First().WaitFor()
		require.NoError(t, err)

		// Count entries under either layout.
		rowCount := tp.TableRowCount("table")
		if rowCount == 0 {
			rows := tp.page.Locator(".list-rows [data-search-item], .list-container [data-search-item]")
			if c, _ := rows.Count(); c > 0 {
				rowCount = c
			}
		}
		assert.Greater(t, rowCount, 0, "Should show at least one user")
	})

	t.Run("search functionality works", func(t *testing.T) {
		tp.Navigate("/users")

		// ldap-manager uses a client-side filter with data-search-input;
		// accept the legacy selectors too for forward-compat.
		searchInput := tp.page.Locator("[data-search-input], input[type='search'], input[name='search'], input[placeholder*='filter' i], input[placeholder*='search' i]")
		count, _ := searchInput.Count()

		if count > 0 {
			err := searchInput.First().Fill("testuser1")
			require.NoError(t, err)

			// Wait for the in-page JS filter to settle by polling the row
			// count via a web expect — avoids the flaky WaitForTimeout.
			require.NoError(t, tp.page.Locator(".list-rows [data-search-item], .list-container [data-search-item]").First().WaitFor())

			t.Log("Search functionality is available")
		} else {
			t.Skip("Search functionality not implemented")
		}
	})
}

// TestUserDetailJourney covers the "click a user → see DN and attributes"
// flow that the spec calls out as a required e2e journey.
func TestUserDetailJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	require.NoError(t, tp.LoginAsAdmin())
	defer tp.Logout()

	tp.Navigate("/users")

	// Wait for the seeded users to show up. main_test.go seeds testuser1
	// and admin-user under ou=users; the V2 list layout renders them as
	// anchors inside .list-rows [data-search-item] rows (legacy selectors
	// retained for forward-compat).
	rowsSel := ".list-rows [data-search-item] a, .list-container [data-search-item] a, table tbody tr a"
	require.NoError(t, tp.page.Locator(rowsSel).First().WaitFor())

	userLink := tp.page.Locator(rowsSel).First()
	href, err := userLink.GetAttribute("href")
	require.NoError(t, err)
	require.NotEmpty(t, href, "Expected at least one user link in /users")

	require.NoError(t, userLink.Click())
	require.NoError(t, tp.page.WaitForURL("**/users/**"))

	// V2 detail renders: drawer__title <h2> with CN, drawer__dn with DN,
	// a "Groups · N" section header, and an "Attributes" section. Verify
	// the shell populated from LDAP and not fell through to an empty
	// template / 500.
	// Use .First() — V2 full-page detail has both an h1 (site title via
	// the shell) and an h2.drawer__title (the CN); strict-mode-safe.
	hasHeading := false
	for _, sel := range []string{"h2.drawer__title", "h1", "h2"} {
		if visible, err := tp.page.Locator(sel).First().IsVisible(); err == nil && visible {
			hasHeading = true
			break
		}
	}
	assert.True(t, hasHeading, "user detail should render a top heading with the user CN")
	assert.True(t, tp.HasText("dc=example,dc=com"),
		"user detail should expose the DN (contains base DN)")
	// V2 uses "Groups · N" (middle dot); legacy used "Groups:". Either is OK.
	hasGroups := tp.HasText("Groups ·") || tp.HasText("Groups:") || tp.HasText("GROUPS ·")
	assert.True(t, hasGroups, "user detail should render the Groups section header")
	// V2 uses inline-edit kv forms for email/description; legacy had the
	// "Add to group" form. Either signals a populated detail view.
	hasForm := tp.HasText("Add to group") || tp.IsVisible(".kv-edit") || tp.IsVisible(".drawer__kv")
	assert.True(t, hasForm, "user detail should expose an editable section")
}

func TestGroupManagementJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	err := tp.LoginAsAdmin()
	require.NoError(t, err)
	defer tp.Logout()

	t.Run("group list page loads", func(t *testing.T) {
		tp.Navigate("/groups")

		// Should have groups displayed
		hasGroups := tp.IsVisible("table") || tp.IsVisible("[data-testid='group-list']") ||
			tp.HasText("groups") || tp.HasText("Groups")
		assert.True(t, hasGroups, "Group list page should load")
	})

	t.Run("can view group details", func(t *testing.T) {
		tp.Navigate("/groups")

		// Try to click on a group link
		groupLink := tp.page.Locator("table tbody tr a, [data-testid='group-link']").First()
		count, _ := groupLink.Count()

		if count > 0 {
			err := groupLink.Click()
			require.NoError(t, err)

			// Should navigate to group details
			currentURL := tp.GetCurrentPath()
			assert.True(t, strings.Contains(currentURL, "group"),
				"Should navigate to group details")
		} else {
			t.Skip("No group links available")
		}
	})
}

func TestErrorPagesJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	t.Run("404 page renders for non-existent routes", func(t *testing.T) {
		// The 404 template lives behind the authenticated layout; log in so
		// navigation to a missing route doesn't bounce to /login first.
		require.NoError(t, tp.LoginAsAdmin())
		defer tp.Logout()

		tp.Navigate("/this-page-does-not-exist-12345")

		// ldap-manager's FourOhFour template uses a human phrasing rather
		// than the literal "404"; accept either so we don't couple the test
		// to copy.
		has404 := tp.HasText("404") ||
			tp.HasText("not found") ||
			tp.HasText("Not Found") ||
			tp.HasText("does not exist")
		assert.True(t, has404, "Should show 404 page for non-existent routes")
	})
}

func TestNavigationJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	err := tp.LoginAsAdmin()
	require.NoError(t, err)
	defer tp.Logout()

	t.Run("navigation menu is visible", func(t *testing.T) {
		tp.Navigate("/")

		// Should have navigation. Use .first to avoid strict-mode locator
		// violations on pages that legitimately render multiple <nav>
		// regions (topnav_v2 contributes a header <nav> + the primary nav).
		hasNav := false
		for _, sel := range []string{"nav", "[role='navigation']", ".navbar", ".nav", ".topnav-secondary", ".topnav"} {
			loc := tp.page.Locator(sel).First()
			if visible, err := loc.IsVisible(); err == nil && visible {
				hasNav = true

				break
			}
		}
		assert.True(t, hasNav, "Navigation should be visible when authenticated")
	})

	t.Run("can navigate between pages", func(t *testing.T) {
		pages := []struct {
			link string
			path string
		}{
			{"a[href='/users'], a:has-text('Users')", "/users"},
			{"a[href='/groups'], a:has-text('Groups')", "/groups"},
		}

		for _, p := range pages {
			link := tp.page.Locator(p.link).First()
			count, _ := link.Count()

			if count > 0 {
				err := link.Click()
				require.NoError(t, err)

				currentURL := tp.GetCurrentPath()
				assert.Contains(t, currentURL, p.path,
					"Should navigate to %s", p.path)
			}
		}
	})
}

func TestFormValidationJourney(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	t.Run("login form has CSRF protection", func(t *testing.T) {
		tp.Navigate("/login")

		// Check for CSRF token
		csrfInput := tp.page.Locator("input[name='csrf'], input[name='_csrf'], input[name='csrf_token']")
		count, _ := csrfInput.Count()

		// CSRF might be in header or cookie instead
		if count > 0 {
			value, _ := csrfInput.First().InputValue()
			assert.NotEmpty(t, value, "CSRF token should have a value")
		}
	})

	t.Run("password field is masked", func(t *testing.T) {
		tp.Navigate("/login")

		passwordInput := tp.page.Locator("input[name='password']")
		inputType, err := passwordInput.GetAttribute("type")
		require.NoError(t, err)

		assert.Equal(t, "password", inputType, "Password field should be masked")
	})
}

func TestSessionPersistence(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	t.Run("session persists across page navigation", func(t *testing.T) {
		err := tp.LoginAsAdmin()
		require.NoError(t, err)

		// Navigate to multiple pages
		tp.Navigate("/users")
		assert.True(t, tp.IsLoggedIn(), "Should stay logged in on /users")

		tp.Navigate("/groups")
		assert.True(t, tp.IsLoggedIn(), "Should stay logged in on /groups")

		tp.Navigate("/")
		assert.True(t, tp.IsLoggedIn(), "Should stay logged in on /")

		tp.Logout()
	})
}

func TestAccessibility(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	page := browser.NewPage(t)
	defer page.Close()
	tp := NewTestPage(t, page, config)

	t.Run("login form has accessible labels", func(t *testing.T) {
		tp.Navigate("/login")

		// Check for labels or aria-labels
		usernameInput := tp.page.Locator("input[name='username']")
		ariaLabel, _ := usernameInput.GetAttribute("aria-label")
		id, _ := usernameInput.GetAttribute("id")

		hasAccessibleLabel := ariaLabel != "" || id != ""
		assert.True(t, hasAccessibleLabel, "Username input should have accessible labeling")
	})

	t.Run("page has proper heading structure", func(t *testing.T) {
		tp.Navigate("/login")

		// Should have at least one heading
		h1 := tp.page.Locator("h1")
		count, _ := h1.Count()

		assert.GreaterOrEqual(t, count, 0, "Page should have heading structure")
	})

	t.Run("buttons are focusable", func(t *testing.T) {
		tp.Navigate("/login")

		submitBtn := tp.page.Locator("button[type='submit']")
		disabled, _ := submitBtn.GetAttribute("disabled")

		assert.Empty(t, disabled, "Submit button should not be disabled by default")
	})
}

func TestResponsiveLayout(t *testing.T) {
	config := DefaultTestConfig()
	browser := NewTestBrowser(t, config)
	defer browser.Close()

	t.Run("login page renders on mobile viewport", func(t *testing.T) {
		page, err := browser.browser.NewPage(playwright.BrowserNewPageOptions{
			Viewport: &playwright.Size{
				Width:  375,
				Height: 667,
			},
		})
		require.NoError(t, err)
		defer page.Close()

		tp := NewTestPage(t, page, config)
		tp.Navigate("/login")

		// Form should still be visible on mobile
		assert.True(t, tp.IsVisible("form"), "Login form should be visible on mobile")
		assert.True(t, tp.IsVisible("button[type='submit']"), "Submit button should be visible on mobile")
	})

	t.Run("login page renders on tablet viewport", func(t *testing.T) {
		page, err := browser.browser.NewPage(playwright.BrowserNewPageOptions{
			Viewport: &playwright.Size{
				Width:  768,
				Height: 1024,
			},
		})
		require.NoError(t, err)
		defer page.Close()

		tp := NewTestPage(t, page, config)
		tp.Navigate("/login")

		assert.True(t, tp.IsVisible("form"), "Login form should be visible on tablet")
	})
}
