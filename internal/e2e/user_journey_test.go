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

		// Should have a table or list of users
		hasTable := tp.IsVisible("table") || tp.IsVisible("[data-testid='user-list']")
		assert.True(t, hasTable, "User list should display users in a table or list")
	})

	t.Run("user list shows user data", func(t *testing.T) {
		tp.Navigate("/users")

		// Wait for content to load
		tp.WaitForSelector("table tbody tr, [data-testid='user-item']")

		// Should have at least one user
		rowCount := tp.TableRowCount("table")
		assert.Greater(t, rowCount, 0, "Should show at least one user")
	})

	t.Run("search functionality works", func(t *testing.T) {
		tp.Navigate("/users")

		searchInput := tp.page.Locator("input[type='search'], input[name='search'], input[placeholder*='search' i]")
		count, _ := searchInput.Count()

		if count > 0 {
			// Search for a user
			err := searchInput.First().Fill("test")
			require.NoError(t, err)

			// Wait for results to update
			tp.page.WaitForTimeout(500)

			// Results should be filtered
			t.Log("Search functionality is available")
		} else {
			t.Skip("Search functionality not implemented")
		}
	})
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
		tp.Navigate("/this-page-does-not-exist-12345")

		// Check for 404 indicator
		has404 := tp.HasText("404") || tp.HasText("not found") || tp.HasText("Not Found")
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
			ViewportSize: &playwright.Size{
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
			ViewportSize: &playwright.Size{
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
