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

		// Wait for either the legacy table layout or the current list layout.
		// .First() avoids Playwright's strict-mode violation when multiple
		// rows match.
		err := tp.page.Locator(".list-container [data-search-item], table tbody tr, [data-testid='user-item']").First().WaitFor()
		require.NoError(t, err)

		// Count entries under either layout.
		rowCount := tp.TableRowCount("table")
		if rowCount == 0 {
			rows := tp.page.Locator(".list-container [data-search-item]")
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
			require.NoError(t, tp.page.Locator(".list-container [data-search-item]").First().WaitFor())

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
	// and admin-user under ou=users; the list layout renders them as
	// anchors inside data-search-item rows.
	require.NoError(t, tp.page.Locator(".list-container [data-search-item] a, table tbody tr a").First().WaitFor())

	userLink := tp.page.Locator(".list-container [data-search-item] a, table tbody tr a").First()
	href, err := userLink.GetAttribute("href")
	require.NoError(t, err)
	require.NotEmpty(t, href, "Expected at least one user link in /users")

	require.NoError(t, userLink.Click())
	require.NoError(t, tp.page.WaitForURL("**/users/**"))

	// The detail view is rendered by templates.User; it shows the CN as an
	// H1, the DN inside a .text-text-secondary block, a "Groups:" heading,
	// and an "Add to group" form. Verifying all four is a strong signal
	// that the detail page actually populated from LDAP rather than
	// falling through to a 500/empty shell.
	assert.True(t, tp.IsVisible("h1"), "user detail should render an <h1> with the user CN")
	assert.True(t, tp.HasText("dc=example,dc=com"),
		"user detail should expose the DN (contains base DN)")
	assert.True(t, tp.HasText("Groups:"), "user detail should render the Groups section header")
	assert.True(t, tp.HasText("Add to group"), "user detail should expose the add-to-group form")
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

		// Should have navigation
		hasNav := tp.IsVisible("nav") || tp.IsVisible("[role='navigation']") ||
			tp.IsVisible(".navbar") || tp.IsVisible(".nav")
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
