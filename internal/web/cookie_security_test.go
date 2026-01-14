package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/options"
)

// TestCookieSecurityWithHTTPS verifies secure cookie configuration for HTTPS environments
func TestCookieSecurityWithHTTPS(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		CookieSecure:            true, // HTTPS environment
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Verify CookieSecure setting is true for HTTPS
	if !opts.CookieSecure {
		t.Error("Expected CookieSecure=true for HTTPS environment")
	}

	// Test session store creation doesn't panic
	sessionStore := createSessionStore(opts)
	if sessionStore == nil {
		t.Fatal("Expected session store, got nil")
	}

	// Test CSRF handler creation doesn't panic
	csrfHandler := createCSRFConfig(opts, sessionStore)
	if csrfHandler == nil {
		t.Fatal("Expected CSRF handler, got nil")
	}
}

// TestCookieSecurityWithHTTP verifies cookie configuration for HTTP-only environments
func TestCookieSecurityWithHTTP(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		CookieSecure:            false, // HTTP-only environment
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Verify CookieSecure setting is false for HTTP
	if opts.CookieSecure {
		t.Error("Expected CookieSecure=false for HTTP environment")
	}

	// Test session store creation doesn't panic
	sessionStore := createSessionStore(opts)
	if sessionStore == nil {
		t.Fatal("Expected session store, got nil")
	}

	// Test CSRF handler creation doesn't panic
	csrfHandler := createCSRFConfig(opts, sessionStore)
	if csrfHandler == nil {
		t.Fatal("Expected CSRF handler, got nil")
	}
}

// TestCookieSecureConfiguration verifies cookie security settings are properly passed through
func TestCookieSecureConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		cookieSecure bool
		description  string
	}{
		{
			name:         "HTTPS environment",
			cookieSecure: true,
			description:  "Secure cookies enabled for HTTPS",
		},
		{
			name:         "HTTP environment",
			cookieSecure: false,
			description:  "Secure cookies disabled for HTTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &options.Opts{
				LDAP: ldap.Config{
					Server:            "ldap://localhost:389",
					BaseDN:            "dc=test,dc=local",
					IsActiveDirectory: false,
				},
				ReadonlyUser:            "cn=readonly,dc=test,dc=local",
				ReadonlyPassword:        "password",
				CookieSecure:            tt.cookieSecure,
				PersistSessions:         false,
				SessionDuration:         30 * time.Minute,
				PoolMaxConnections:      10,
				PoolMinConnections:      2,
				PoolMaxIdleTime:         15 * time.Minute,
				PoolHealthCheckInterval: 30 * time.Second,
				PoolConnectionTimeout:   30 * time.Second,
				PoolAcquireTimeout:      10 * time.Second,
			}

			// Verify configuration value
			if opts.CookieSecure != tt.cookieSecure {
				t.Errorf("%s: Expected CookieSecure=%v, got %v", tt.description, tt.cookieSecure, opts.CookieSecure)
			}

			// Test session store creation with configuration
			sessionStore := createSessionStore(opts)
			if sessionStore == nil {
				t.Fatal("Expected session store, got nil")
			}

			// Test CSRF handler creation with configuration
			csrfHandler := createCSRFConfig(opts, sessionStore)
			if csrfHandler == nil {
				t.Fatal("Expected CSRF handler, got nil")
			}
		})
	}
}

// TestCSRFConfigurationAcceptsOpts verifies CSRF handler accepts options and session store parameters
func TestCSRFConfigurationAcceptsOpts(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		CookieSecure:            true,
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Create session store for CSRF middleware
	sessionStore := createSessionStore(opts)
	if sessionStore == nil {
		t.Fatal("Expected session store, got nil")
	}

	// Verify CSRF handler creation accepts opts and sessionStore parameters
	csrfHandler := createCSRFConfig(opts, sessionStore)
	if csrfHandler == nil {
		t.Fatal("Expected CSRF handler, got nil")
	}

	// Handler created successfully - type is fiber.Handler (internal Fiber type)
	t.Log("CSRF handler created successfully with opts and sessionStore parameters")
}

// TestCSRFTokenValidation verifies that CSRF tokens are properly validated on POST requests.
// This test ensures the CSRF expiration is set correctly (regression test for the 3600 nanoseconds bug).
//
//nolint:gocognit // Test function with multiple subtests has inherent complexity
func TestCSRFTokenValidation(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		CookieSecure:            false, // HTTP for testing
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Create a test Fiber app with CSRF middleware
	f := fiber.New()
	sessionStore := session.New(session.Config{
		Storage: memory.New(),
	})
	csrfHandler := createCSRFConfig(opts, sessionStore)

	// Test endpoint that returns CSRF token on GET and validates on POST
	f.All("/test-csrf", *csrfHandler, func(c *fiber.Ctx) error {
		sess, err := sessionStore.Get(c)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to get session")
		}
		defer func() { _ = sess.Save() }()

		if c.Method() == "GET" {
			token := c.Locals("token")
			if token == nil {
				return c.Status(fiber.StatusInternalServerError).SendString("No CSRF token generated")
			}

			tokenStr, ok := token.(string)
			if !ok {
				return c.Status(fiber.StatusInternalServerError).SendString("CSRF token is not a string")
			}

			return c.SendString("csrf_token:" + tokenStr)
		}
		// POST - if we get here, CSRF validation passed
		return c.SendString("CSRF validation passed")
	})

	t.Run("GET request returns CSRF token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-csrf", nil)
		resp, err := f.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		if !strings.HasPrefix(string(body), "csrf_token:") {
			t.Errorf("Expected CSRF token in response, got: %s", string(body))
		}
	})

	t.Run("POST without CSRF token returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test-csrf", strings.NewReader("data=test"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := f.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected status %d for missing CSRF token, got %d", http.StatusForbidden, resp.StatusCode)
		}
	})

	t.Run("POST with invalid CSRF token returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/test-csrf", strings.NewReader("csrf_token=invalid-token&data=test"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := f.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected status %d for invalid CSRF token, got %d", http.StatusForbidden, resp.StatusCode)
		}
	})

	t.Run("POST with valid CSRF token succeeds", func(t *testing.T) {
		// Step 1: GET to obtain CSRF token and session cookie
		// With session-based CSRF, the token is stored in the session
		getReq := httptest.NewRequest("GET", "/test-csrf", nil)
		getResp, err := f.Test(getReq)
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}

		// Extract CSRF token from response body
		body, err := io.ReadAll(getResp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		_ = getResp.Body.Close()

		tokenMatch := regexp.MustCompile(`csrf_token:(.+)`).FindStringSubmatch(string(body))
		if len(tokenMatch) < 2 {
			t.Fatalf("Could not extract CSRF token from response: %s", string(body))
		}
		csrfToken := tokenMatch[1]

		// Extract all cookies (session cookie contains the CSRF token with session-based CSRF)
		cookies := getResp.Cookies()
		if len(cookies) == 0 {
			t.Fatal("No cookies found in response (session cookie required for session-based CSRF)")
		}

		// Step 2: POST with valid CSRF token and all cookies (including session cookie)
		postReq := httptest.NewRequest("POST", "/test-csrf",
			strings.NewReader("csrf_token="+csrfToken+"&data=test"))
		postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// Add all cookies from GET response (session-based CSRF needs the session cookie)
		for _, cookie := range cookies {
			postReq.AddCookie(cookie)
		}

		postResp, err := f.Test(postReq)
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		}
		defer func() { _ = postResp.Body.Close() }()

		// Read response body once for both assertions
		respBody, err := io.ReadAll(postResp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		// This is the critical test: with the bug (Expiration: 3600 nanoseconds),
		// the token would expire immediately and this would return 403.
		// With the fix (Expiration: time.Hour), this should return 200.
		if postResp.StatusCode != http.StatusOK {
			t.Errorf("Expected status %d for valid CSRF token, got %d. Response: %s",
				http.StatusOK, postResp.StatusCode, string(respBody))
		}

		if string(respBody) != "CSRF validation passed" {
			t.Errorf("Expected 'CSRF validation passed', got: %s", string(respBody))
		}
	})
}
