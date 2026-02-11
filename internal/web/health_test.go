package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// setupHealthTestApp creates a test application for health endpoint testing (with service account)
func setupHealthTestApp() *App {
	mockClient := &testLDAPClient{
		users: []ldap.User{
			{SAMAccountName: "test.user", Enabled: true},
		},
		groups:    []ldap.Group{},
		computers: []ldap.Computer{},
	}

	sessionStore := session.New(session.Config{
		Storage: memory.New(),
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	testConfig := ldap.Config{
		Server: "ldap://test.server.com",
		Port:   389,
		BaseDN: "dc=test,dc=com",
	}
	testClient, _ := ldap.New(testConfig, "cn=admin", "password") //nolint:errcheck

	app := &App{
		ldapReadonly: testClient,
		ldapCache:    ldap_cache.New(mockClient),
		sessionStore: sessionStore,
		fiber:        f,
	}

	// Populate cache - errors are expected in test environment with mock client
	_ = app.ldapCache.RefreshUsers()     //nolint:errcheck
	_ = app.ldapCache.RefreshGroups()    //nolint:errcheck
	_ = app.ldapCache.RefreshComputers() //nolint:errcheck

	// Setup health routes (no authentication required)
	f.Get("/health", app.healthHandler)
	f.Get("/ready", app.readinessHandler)
	f.Get("/live", app.livenessHandler)

	return app
}

// setupHealthTestAppNoServiceAccount creates a test application without service account
func setupHealthTestAppNoServiceAccount() *App {
	sessionStore := session.New(session.Config{
		Storage: memory.New(),
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	app := &App{
		ldapReadonly: nil,
		ldapCache:    nil,
		sessionStore: sessionStore,
		fiber:        f,
	}

	f.Get("/health", app.healthHandler)
	f.Get("/ready", app.readinessHandler)
	f.Get("/live", app.livenessHandler)

	return app
}

func TestHealthHandler(t *testing.T) {
	app := setupHealthTestApp()

	t.Run("returns health status JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Should return 200 or 503 depending on health state
		if resp.StatusCode != fiber.StatusOK && resp.StatusCode != fiber.StatusServiceUnavailable {
			t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
		}

		// Verify JSON response structure
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
		}

		// Check expected fields
		if _, ok := response["cache"]; !ok {
			t.Error("Response should contain 'cache' field")
		}
		if _, ok := response["connection_pool"]; !ok {
			t.Error("Response should contain 'connection_pool' field")
		}
		if _, ok := response["overall_healthy"]; !ok {
			t.Error("Response should contain 'overall_healthy' field")
		}
	})

	t.Run("response is JSON content type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
		}
	})
}

func TestHealthHandlerNoServiceAccount(t *testing.T) {
	app := setupHealthTestAppNoServiceAccount()

	t.Run("returns healthy status without service account", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		assertHTTPStatus(t, resp, fiber.StatusOK)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
		}

		healthy, ok := response["overall_healthy"]
		if !ok {
			t.Error("Response should contain 'overall_healthy' field")
		}
		if healthy != true {
			t.Errorf("Expected overall_healthy=true, got %v", healthy)
		}

		mode, ok := response["mode"]
		if !ok {
			t.Error("Response should contain 'mode' field")
		}
		if mode != "per-user credentials" {
			t.Errorf("Expected mode='per-user credentials', got %v", mode)
		}
	})
}

func TestLivenessHandler(t *testing.T) {
	app := setupHealthTestApp()

	t.Run("returns alive status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		assertHTTPStatus(t, resp, fiber.StatusOK)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
		}

		// Check status field
		status, ok := response["status"]
		if !ok {
			t.Error("Response should contain 'status' field")
		}
		if status != "alive" {
			t.Errorf("Expected status 'alive', got '%v'", status)
		}

		// Check uptime field (present when service account is configured)
		if _, ok := response["uptime"]; !ok {
			t.Error("Response should contain 'uptime' field when service account is configured")
		}
	})

	t.Run("always returns 200 for liveness", func(t *testing.T) {
		// Liveness probe should always return 200 if app is running
		req := httptest.NewRequest("GET", "/live", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Liveness should always return 200, got %d", resp.StatusCode)
		}
	})
}

func TestLivenessHandlerNoServiceAccount(t *testing.T) {
	app := setupHealthTestAppNoServiceAccount()

	t.Run("returns alive status without uptime", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		assertHTTPStatus(t, resp, fiber.StatusOK)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
		}

		status, ok := response["status"]
		if !ok {
			t.Error("Response should contain 'status' field")
		}
		if status != "alive" {
			t.Errorf("Expected status 'alive', got '%v'", status)
		}
	})
}

func TestReadinessHandler(t *testing.T) {
	app := setupHealthTestApp()

	t.Run("returns readiness status JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ready", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Should return 200 or 503 depending on readiness state
		if resp.StatusCode != fiber.StatusOK && resp.StatusCode != fiber.StatusServiceUnavailable {
			t.Errorf("Expected status 200 or 503, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
		}

		// Check expected fields
		if _, ok := response["status"]; !ok {
			t.Error("Response should contain 'status' field")
		}
	})
}

func TestReadinessHandlerNoServiceAccount(t *testing.T) {
	app := setupHealthTestAppNoServiceAccount()

	t.Run("returns ready without service account", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ready", http.NoBody)
		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		assertHTTPStatus(t, resp, fiber.StatusOK)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
		}

		status, ok := response["status"]
		if !ok {
			t.Error("Response should contain 'status' field")
		}
		if status != "ready" {
			t.Errorf("Expected status 'ready', got %v", status)
		}
	})
}

func TestGetHealthStatusCode(t *testing.T) {
	app := setupHealthTestApp()

	tests := []struct {
		name           string
		overallHealthy bool
		cacheStatus    string
		poolHealthy    bool
		expected       int
	}{
		{
			name:           "fully healthy returns 200",
			overallHealthy: true,
			cacheStatus:    "healthy",
			poolHealthy:    true,
			expected:       fiber.StatusOK,
		},
		{
			name:           "degraded cache returns 200",
			overallHealthy: false,
			cacheStatus:    "degraded",
			poolHealthy:    true,
			expected:       fiber.StatusOK,
		},
		{
			name:           "healthy cache but unhealthy pool returns 200",
			overallHealthy: false,
			cacheStatus:    "healthy",
			poolHealthy:    false,
			expected:       fiber.StatusOK,
		},
		{
			name:           "unhealthy returns 503",
			overallHealthy: false,
			cacheStatus:    "unhealthy",
			poolHealthy:    false,
			expected:       fiber.StatusServiceUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := app.getHealthStatusCode(tc.overallHealthy, tc.cacheStatus, tc.poolHealthy)
			if result != tc.expected {
				t.Errorf("Expected status %d, got %d", tc.expected, result)
			}
		})
	}
}

func TestGetReadinessStatus(t *testing.T) {
	app := setupHealthTestApp()

	tests := []struct {
		name           string
		cacheHealthy   bool
		warmedUp       bool
		poolHealthy    bool
		expectedStatus string
		expectReason   bool // true if we expect a non-empty reason
	}{
		{
			name:           "fully ready returns empty status",
			cacheHealthy:   true,
			warmedUp:       true,
			poolHealthy:    true,
			expectedStatus: "",
			expectReason:   false,
		},
		{
			name:           "all unhealthy returns not ready",
			cacheHealthy:   false,
			warmedUp:       false,
			poolHealthy:    false,
			expectedStatus: "not ready",
			expectReason:   true,
		},
		{
			name:           "cache unhealthy and not warmed up",
			cacheHealthy:   false,
			warmedUp:       false,
			poolHealthy:    true,
			expectedStatus: "not ready",
			expectReason:   true,
		},
		{
			name:           "cache and pool unhealthy",
			cacheHealthy:   false,
			warmedUp:       true,
			poolHealthy:    false,
			expectedStatus: "not ready",
			expectReason:   true,
		},
		{
			name:           "not warmed up and pool unhealthy",
			cacheHealthy:   true,
			warmedUp:       false,
			poolHealthy:    false,
			expectedStatus: "warming up",
			expectReason:   true,
		},
		{
			name:           "only cache unhealthy",
			cacheHealthy:   false,
			warmedUp:       true,
			poolHealthy:    true,
			expectedStatus: "not ready",
			expectReason:   true,
		},
		{
			name:           "only not warmed up",
			cacheHealthy:   true,
			warmedUp:       false,
			poolHealthy:    true,
			expectedStatus: "warming up",
			expectReason:   true,
		},
		{
			name:           "only pool unhealthy",
			cacheHealthy:   true,
			warmedUp:       true,
			poolHealthy:    false,
			expectedStatus: "not ready",
			expectReason:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, reason := app.getReadinessStatus(tc.cacheHealthy, tc.warmedUp, tc.poolHealthy)

			if status != tc.expectedStatus {
				t.Errorf("Expected status '%s', got '%s'", tc.expectedStatus, status)
			}

			if tc.expectReason && reason == "" {
				t.Error("Expected non-empty reason")
			}
			if !tc.expectReason && reason != "" {
				t.Errorf("Expected empty reason, got '%s'", reason)
			}
		})
	}
}
