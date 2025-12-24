//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

// TestConfig holds configuration for E2E tests
type TestConfig struct {
	BaseURL      string
	Headless     bool
	SlowMo       float64
	Timeout      float64
	AdminUser    string
	AdminPass    string
	TestUser     string
	TestUserPass string
}

// DefaultTestConfig returns sensible defaults for E2E testing
func DefaultTestConfig() TestConfig {
	baseURL := os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return TestConfig{
		BaseURL:      baseURL,
		Headless:     os.Getenv("E2E_HEADLESS") != "false",
		SlowMo:       0,
		Timeout:      30000,
		AdminUser:    envOrDefault("E2E_ADMIN_USER", "admin"),
		AdminPass:    envOrDefault("E2E_ADMIN_PASS", "adminpassword"),
		TestUser:     envOrDefault("E2E_TEST_USER", "testuser1"),
		TestUserPass: envOrDefault("E2E_TEST_USER_PASS", "password1"),
	}
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// TestBrowser wraps Playwright browser for testing
type TestBrowser struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	config  TestConfig
}

// NewTestBrowser creates a new browser instance for testing
func NewTestBrowser(t *testing.T, config TestConfig) *TestBrowser {
	t.Helper()

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("Failed to start Playwright: %v", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(config.Headless),
		SlowMo:   playwright.Float(config.SlowMo),
	})
	if err != nil {
		pw.Stop()
		t.Fatalf("Failed to launch browser: %v", err)
	}

	return &TestBrowser{
		pw:      pw,
		browser: browser,
		config:  config,
	}
}

// Close cleans up browser resources
func (tb *TestBrowser) Close() {
	if tb.browser != nil {
		tb.browser.Close()
	}
	if tb.pw != nil {
		tb.pw.Stop()
	}
}

// NewPage creates a new browser page with default settings
func (tb *TestBrowser) NewPage(t *testing.T) playwright.Page {
	t.Helper()

	page, err := tb.browser.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	page.SetDefaultTimeout(tb.config.Timeout)

	return page
}

// TestPage wraps a Playwright page with helper methods
type TestPage struct {
	page   playwright.Page
	config TestConfig
	t      *testing.T
}

// NewTestPage creates a wrapped test page
func NewTestPage(t *testing.T, page playwright.Page, config TestConfig) *TestPage {
	return &TestPage{
		page:   page,
		config: config,
		t:      t,
	}
}

// Navigate goes to a path relative to base URL
func (tp *TestPage) Navigate(path string) {
	tp.t.Helper()
	url := tp.config.BaseURL + path
	_, err := tp.page.Goto(url)
	if err != nil {
		tp.t.Fatalf("Failed to navigate to %s: %v", url, err)
	}
}

// Login performs login with given credentials
func (tp *TestPage) Login(username, password string) error {
	tp.Navigate("/login")

	// Wait for login form
	if err := tp.page.Locator("form").WaitFor(); err != nil {
		return fmt.Errorf("login form not found: %w", err)
	}

	// Fill credentials
	if err := tp.page.Locator("input[name='username']").Fill(username); err != nil {
		return fmt.Errorf("failed to fill username: %w", err)
	}

	if err := tp.page.Locator("input[name='password']").Fill(password); err != nil {
		return fmt.Errorf("failed to fill password: %w", err)
	}

	// Submit form
	if err := tp.page.Locator("button[type='submit']").Click(); err != nil {
		return fmt.Errorf("failed to click submit: %w", err)
	}

	// Wait for navigation
	time.Sleep(500 * time.Millisecond)

	return nil
}

// LoginAsAdmin logs in with admin credentials
func (tp *TestPage) LoginAsAdmin() error {
	return tp.Login(tp.config.AdminUser, tp.config.AdminPass)
}

// LoginAsTestUser logs in with test user credentials
func (tp *TestPage) LoginAsTestUser() error {
	return tp.Login(tp.config.TestUser, tp.config.TestUserPass)
}

// Logout performs logout
func (tp *TestPage) Logout() error {
	tp.Navigate("/logout")
	time.Sleep(300 * time.Millisecond)
	return nil
}

// IsLoggedIn checks if user is currently logged in
func (tp *TestPage) IsLoggedIn() bool {
	// Check for logout button or user indicator
	logoutBtn := tp.page.Locator("a[href='/logout'], button:has-text('Logout')")
	count, err := logoutBtn.Count()
	return err == nil && count > 0
}

// GetCurrentPath returns the current URL path
func (tp *TestPage) GetCurrentPath() string {
	return tp.page.URL()
}

// HasText checks if page contains specific text
func (tp *TestPage) HasText(text string) bool {
	locator := tp.page.Locator(fmt.Sprintf("text=%s", text))
	count, err := locator.Count()
	return err == nil && count > 0
}

// WaitForSelector waits for an element to appear
func (tp *TestPage) WaitForSelector(selector string) error {
	return tp.page.Locator(selector).WaitFor()
}

// Click clicks an element
func (tp *TestPage) Click(selector string) error {
	return tp.page.Locator(selector).Click()
}

// Fill fills an input field
func (tp *TestPage) Fill(selector, value string) error {
	return tp.page.Locator(selector).Fill(value)
}

// GetText returns text content of an element
func (tp *TestPage) GetText(selector string) (string, error) {
	return tp.page.Locator(selector).TextContent()
}

// Screenshot takes a screenshot for debugging
func (tp *TestPage) Screenshot(name string) {
	tp.t.Helper()
	path := fmt.Sprintf("test-screenshots/%s.png", name)
	_, err := tp.page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(path),
	})
	if err != nil {
		tp.t.Logf("Failed to take screenshot: %v", err)
	}
}

// GetFlashMessage returns flash message text if present
func (tp *TestPage) GetFlashMessage() (string, bool) {
	flashLocator := tp.page.Locator(".flash-message, .alert, [role='alert']")
	count, err := flashLocator.Count()
	if err != nil || count == 0 {
		return "", false
	}
	text, err := flashLocator.First().TextContent()
	if err != nil {
		return "", false
	}
	return text, true
}

// WaitForURL waits until URL matches pattern
func (tp *TestPage) WaitForURL(pattern string) error {
	return tp.page.WaitForURL(pattern)
}

// TableRowCount returns number of rows in a table
func (tp *TestPage) TableRowCount(tableSelector string) int {
	rows := tp.page.Locator(tableSelector + " tbody tr")
	count, err := rows.Count()
	if err != nil {
		return 0
	}
	return count
}

// SelectOption selects an option in a dropdown
func (tp *TestPage) SelectOption(selector string, value string) error {
	_, err := tp.page.Locator(selector).SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(value),
	})
	return err
}

// IsVisible checks if an element is visible
func (tp *TestPage) IsVisible(selector string) bool {
	visible, err := tp.page.Locator(selector).IsVisible()
	return err == nil && visible
}

// GetInputValue returns the value of an input field
func (tp *TestPage) GetInputValue(selector string) (string, error) {
	return tp.page.Locator(selector).InputValue()
}

// ConfirmDialog handles confirmation dialogs
func (tp *TestPage) ConfirmDialog(accept bool) {
	tp.page.OnDialog(func(dialog playwright.Dialog) {
		if accept {
			dialog.Accept()
		} else {
			dialog.Dismiss()
		}
	})
}

// WaitForLoadState waits for page to reach a specific load state
func (tp *TestPage) WaitForLoadState(state playwright.LoadState) error {
	return tp.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: &state,
	})
}
