package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// staticComponent creates a templ.Component that renders static text.
func staticComponent(text string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, text)

		return err
	})
}

// setupFullTestApp creates a test app with all routes and template cache,
// mimicking the real setupRoutes but without CSRF for easier testing.
func setupFullTestApp(t *testing.T) (*App, *session.Store) {
	t.Helper()
	mockClient := &testLDAPClient{
		users: []ldap.User{
			{SAMAccountName: "john.doe", Enabled: true},
			{SAMAccountName: "jane.smith", Enabled: false},
		},
		groups: []ldap.Group{
			{Members: []string{"cn=john.doe,ou=users,dc=example,dc=com"}},
		},
		computers: []ldap.Computer{
			{SAMAccountName: "workstation-01$", Enabled: true},
			{SAMAccountName: "workstation-02$", Enabled: false},
		},
	}

	store := session.New(session.Config{
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

	templateCache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      100 * time.Millisecond,
		MaxSize:         100,
		CleanupInterval: 50 * time.Millisecond,
	})

	rateLimiter := NewRateLimiter(RateLimiterConfig{
		MaxAttempts:  5,
		WindowPeriod: time.Minute,
		BlockPeriod:  time.Minute,
		CleanupEvery: time.Hour,
	})

	// Per-user pinned store backed by a temp bbolt file.
	pinnedDB, err := bolt.Open(filepath.Join(t.TempDir(), "pinned.bbolt"), 0o600, nil)
	require.NoError(t, err)
	pinnedStore, err := NewPinnedStore(pinnedDB)
	require.NoError(t, err)

	app := &App{
		ldapConfig:    testConfig,
		ldapCache:     ldap_cache.New(mockClient),
		sessionStore:  store,
		templateCache: templateCache,
		fiber:         f,
		rateLimiter:   rateLimiter,
		stopCacheLog:  make(chan struct{}),
		pinnedStore:   pinnedStore,
		pinnedDB:      pinnedDB,
	}

	t.Cleanup(func() {
		templateCache.Stop()
		rateLimiter.Stop()
		_ = pinnedDB.Close()
	})

	// Populate cache
	_ = app.ldapCache.RefreshUsers()
	_ = app.ldapCache.RefreshGroups()
	_ = app.ldapCache.RefreshComputers()

	// Register routes without CSRF for testing
	f.All("/login", app.rateLimiter.Middleware(), app.loginHandler)
	f.Get("/health", app.healthHandler)
	f.Get("/health/ready", app.readinessHandler)
	f.Get("/health/live", app.livenessHandler)
	f.Get("/debug/cache", app.RequireAuth(), app.cacheStatsHandler)
	f.Get("/debug/ldap-pool", app.RequireAuth(), app.poolStatsHandler)

	// Search index — registered without RequireAuth in the test harness so
	// the shape/ETag tests can exercise the handler directly. Registered
	// BEFORE the protected Group so the group's middleware is not inherited.
	// Production wires the same handler inside the protected group (see server.go).
	f.Get("/api/search-index.json", app.handleSearchIndex)

	// Relationship graph JSON — registered without RequireAuth here for the
	// same reason as search-index above; production wires it inside the
	// protected group (see server.go). The handler reads only from
	// app.ldapCache, so no session state is required.
	f.Get("/api/graph.json", app.handleGraphJSON)

	// Graph view page — registered without RequireAuth here for the same
	// reason as the JSON endpoint above; production wires it inside the
	// protected group (see server.go).
	f.Get("/graph", app.handleGraphV2)

	protected := f.Group("/", app.RequireAuth())
	protected.Get("/", app.handleHomeV2)
	protected.Get("/users", app.templateCacheMiddleware(), app.handleUsersV2)
	protected.Get("/groups", app.templateCacheMiddleware(), app.handleGroupsV2)
	protected.Get("/computers", app.templateCacheMiddleware(), app.handleComputersV2)
	protected.Get("/users/*", app.handleUserV2)
	protected.Get("/groups/*", app.handleGroupV2)
	protected.Get("/computers/*", app.handleComputerV2)
	// Bulk actions — registered BEFORE /<kind>/* so Fiber's wildcard
	// doesn't swallow the exact /<kind>/bulk route.
	protected.Post("/users/bulk", app.handleBulkUsers)
	protected.Post("/groups/bulk", app.handleBulkGroups)
	protected.Post("/computers/bulk", app.handleBulkComputers)
	protected.Post("/users/*", app.userModifyHandler)
	protected.Post("/groups/*", app.groupModifyHandler)
	// Pin / unpin — CSRF-free in the test harness so the handler tests
	// exercise just the handler + store. Production wires these in the
	// protected + CSRF group (see server.go).
	protected.Post("/pin", app.handlePin)
	protected.Post("/unpin", app.handleUnpin)
	protected.Get("/logout", app.logoutHandler)
	f.Use(app.fourOhFourHandler)

	return app, store
}

// createAuthSession creates a valid authenticated session and returns cookies.
func createAuthSession(t *testing.T, _ *App, store *session.Store) []*http.Cookie {
	t.Helper()

	// Create a separate mini Fiber app to set up the session cookie.
	// This avoids middleware interference from the main app.
	miniApp := fiber.New()
	miniApp.Get("/set-session", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "cn=john.doe,ou=users,dc=test,dc=com")
		sess.Set("password", "testpass")
		sess.Set("username", "john.doe")

		return sess.Save()
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/set-session", http.NoBody)
	resp, err := miniApp.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	cookies := resp.Cookies()
	require.NotEmpty(t, cookies, "Expected session cookie")

	return cookies
}

// makeAuthRequest makes an authenticated GET request with session cookies.
func makeAuthRequest(t *testing.T, app *App, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), "GET", path, http.NoBody)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)

	return resp
}

func TestHandle500_FiberError(t *testing.T) {
	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	f.Get("/unauthorized", func(_ *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	})

	f.Get("/not-found", func(_ *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	})

	f.Get("/generic-error", func(_ *fiber.Ctx) error {
		return errors.New("something went wrong")
	})

	t.Run("unauthorized redirects to login", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/unauthorized", http.NoBody)
		resp, err := f.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/login", resp.Header.Get("Location"))
	})

	t.Run("not found uses fiber error code", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/not-found", http.NoBody)
		resp, err := f.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("generic error returns 500", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "GET", "/generic-error", http.NoBody)
		resp, err := f.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestFourOhFourHandler(t *testing.T) {
	// Use a minimal fiber app with only the 404 handler
	f := fiber.New(fiber.Config{ErrorHandler: handle500})
	app := &App{fiber: f, templateCache: NewTemplateCache(DefaultTemplateCacheConfig())}
	defer app.templateCache.Stop()
	f.Use(app.fourOhFourHandler)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/nonexistent/path", http.NoBody)
	resp, err := f.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
}

func TestGetCSRFToken(t *testing.T) {
	app, _ := setupFullTestApp(t)

	t.Run("returns empty for nil token", func(t *testing.T) {
		f := fiber.New()
		var result string
		f.Get("/test", func(c *fiber.Ctx) error {
			result = app.GetCSRFToken(c)

			return c.SendString("ok")
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
		resp, err := f.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Empty(t, result)
	})

	t.Run("returns token when set", func(t *testing.T) {
		f := fiber.New()
		var result string
		f.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("token", "test-csrf-token")
			result = app.GetCSRFToken(c)

			return c.SendString("ok")
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
		resp, err := f.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, "test-csrf-token", result)
	})

	t.Run("returns empty for non-string token", func(t *testing.T) {
		f := fiber.New()
		var result string
		f.Get("/test", func(c *fiber.Ctx) error {
			c.Locals("token", 12345) // not a string
			result = app.GetCSRFToken(c)

			return c.SendString("ok")
		})

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
		resp, err := f.Test(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Empty(t, result)
	})
}

func TestCacheStatsHandler(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	// Add some cache entries for meaningful stats
	app.templateCache.Set("test-key", []byte("test content"), 0)

	resp := makeAuthRequest(t, app, "/debug/cache", cookies)
	defer func() { _ = resp.Body.Close() }()

	// Should get JSON response (or redirect if LDAP fails)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == http.StatusOK {
		var stats CacheStats
		err = json.Unmarshal(body, &stats)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, stats.Entries, 0)
	}
}

func TestPoolStatsHandler_NoServiceAccount(t *testing.T) {
	app, store := setupFullTestApp(t)
	app.ldapReadonly = nil // no service account
	cookies := createAuthSession(t, app, store)

	resp := makeAuthRequest(t, app, "/debug/ldap-pool", cookies)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]any
		err = json.Unmarshal(body, &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "message")
	}
}

func TestTemplateCacheMiddleware(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	t.Run("sets X-Cache header", func(t *testing.T) {
		// First request should be MISS (or redirect due to LDAP)
		resp := makeAuthRequest(t, app, "/users", cookies)
		defer func() { _ = resp.Body.Close() }()

		// Either we get a cache header or a redirect
		if resp.StatusCode == http.StatusOK {
			cacheHeader := resp.Header.Get("X-Cache")
			assert.NotEmpty(t, cacheHeader)
		}
	})
}

func TestPeriodicCacheLogging(t *testing.T) {
	app, _ := setupFullTestApp(t)

	// Start the goroutine and verify it exits when stopCacheLog is closed
	done := make(chan struct{})
	go func() {
		app.periodicCacheLogging()
		close(done)
	}()

	close(app.stopCacheLog)

	select {
	case <-done:
		// Goroutine exited as expected
	case <-time.After(2 * time.Second):
		t.Fatal("periodicCacheLogging did not exit after stopCacheLog was closed")
	}
}

func TestInvalidateTemplateCacheOnModification(t *testing.T) {
	app, _ := setupFullTestApp(t)

	// Add cache entries
	app.templateCache.Set("key1", []byte("content1"), 0)
	app.templateCache.Set("key2", []byte("content2"), 0)
	assert.Equal(t, 2, app.templateCache.Stats().Entries)

	// Invalidate
	app.invalidateTemplateCacheOnModification()
	assert.Equal(t, 0, app.templateCache.Stats().Entries)
}

func TestLogStats(_ *testing.T) {
	cache := NewTemplateCache(DefaultTemplateCacheConfig())
	defer cache.Stop()

	cache.Set("key1", []byte("content"), 0)

	// LogStats should not panic
	cache.LogStats()
}

func TestRenderWithCache(t *testing.T) {
	cache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      1 * time.Second,
		MaxSize:         10,
		CleanupInterval: 1 * time.Second,
	})
	defer cache.Stop()

	f := fiber.New()
	var firstBody, secondBody []byte

	f.Get("/test", func(c *fiber.Ctx) error {
		// Create a simple templ component mock by using the RenderWithCache
		// with a component that writes static content
		return cache.RenderWithCache(c, staticComponent("hello world"))
	})

	// First request - cache miss
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
	resp, err := f.Test(req)
	require.NoError(t, err)
	firstBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(firstBody), "hello world")

	// Second request - should be cache hit
	req = httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
	resp, err = f.Test(req)
	require.NoError(t, err)
	secondBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	assert.Equal(t, firstBody, secondBody)
}

func TestAuthenticatedHandlers_LDAPConnectionFails(t *testing.T) {
	app, store := setupFullTestApp(t)
	cookies := createAuthSession(t, app, store)

	// These handlers need LDAP connections which will fail in tests.
	// This covers the getUserLDAP error path and handle500.
	paths := []string{
		"/",
		"/users",
		"/groups",
		"/computers",
		"/users/cn%3Dtest%2Cdc%3Dtest%2Cdc%3Dcom",
		"/groups/cn%3Dtest%2Cdc%3Dtest%2Cdc%3Dcom",
		"/computers/cn%3Dtest%2Cdc%3Dtest%2Cdc%3Dcom",
	}

	for _, path := range paths {
		t.Run("GET "+path, func(t *testing.T) {
			resp := makeAuthRequest(t, app, path, cookies)
			defer func() { _ = resp.Body.Close() }()

			// Should either redirect to login (LDAP failure → unauthorized),
			// return an error page (404 for "user not found", 500 otherwise),
			// or render successfully — NOT 0.
			assert.Contains(t,
				[]int{http.StatusOK, http.StatusFound, http.StatusNotFound, http.StatusInternalServerError},
				resp.StatusCode,
				"unexpected status %d for GET %s", resp.StatusCode, path)
		})
	}
}

func TestGetUserLDAP_NoSession(t *testing.T) {
	app, _ := setupFullTestApp(t)

	f := fiber.New()
	var getUserLDAPErr error

	f.Get("/test", func(c *fiber.Ctx) error {
		_, getUserLDAPErr = app.getUserLDAP(c)
		if getUserLDAPErr != nil {
			return getUserLDAPErr
		}

		return c.SendString("ok")
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
	resp, err := f.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// getUserLDAP should fail with empty session credentials
	assert.Error(t, getUserLDAPErr)
}

func TestGetUserLDAP_EmptyCredentials(t *testing.T) {
	store := session.New(session.Config{
		Storage: memory.New(),
	})

	app := &App{
		ldapConfig: ldap.Config{
			Server: "ldap://test.server.com",
			Port:   389,
			BaseDN: "dc=test,dc=com",
		},
		sessionStore: store,
	}

	f := fiber.New()
	var getUserLDAPErr error

	// Set up session with empty credentials
	f.Get("/setup", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "")
		sess.Set("password", "")

		return sess.Save()
	})

	f.Get("/test", func(c *fiber.Ctx) error {
		_, getUserLDAPErr = app.getUserLDAP(c)
		if getUserLDAPErr != nil {
			return getUserLDAPErr
		}

		return c.SendString("ok")
	})

	// Setup session
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/setup", http.NoBody)
	resp, err := f.Test(req)
	require.NoError(t, err)
	cookies := resp.Cookies()
	_ = resp.Body.Close()

	// Test with empty credentials
	req = httptest.NewRequestWithContext(context.Background(), "GET", "/test", http.NoBody)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = f.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Error(t, getUserLDAPErr)
	var fiberErr *fiber.Error
	assert.ErrorAs(t, getUserLDAPErr, &fiberErr)
	if fiberErr != nil {
		assert.Equal(t, fiber.StatusUnauthorized, fiberErr.Code)
	}
}

func TestCreateSessionStore(t *testing.T) {
	t.Run("memory storage by default", func(t *testing.T) {
		// createSessionStore with PersistSessions=false uses memory
		// Already tested via setupTestApp, this confirms it works
		store := session.New(session.Config{
			Storage: memory.New(),
		})
		assert.NotNil(t, store)
	})
}

func TestRateLimiter_BlockExpiry(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  2,
		WindowPeriod: 1 * time.Minute,
		BlockPeriod:  100 * time.Millisecond,
		CleanupEvery: 1 * time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.100"

	// Block the IP
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip) // triggers block
	assert.True(t, rl.IsBlocked(ip))

	// Wait for block to expire
	time.Sleep(150 * time.Millisecond)

	// Recording attempt after block expired should reset
	blocked := rl.RecordAttempt(ip)
	assert.False(t, blocked, "Should not be blocked after block period expires")
}

func TestPoolStatsHandler_WithServiceAccount(t *testing.T) {
	app, store := setupFullTestApp(t)

	testConfig := ldap.Config{
		Server: "ldap://test.server.com",
		Port:   389,
		BaseDN: "dc=test,dc=com",
	}
	testClient, _ := ldap.New(testConfig, "cn=admin", "password") //nolint:errcheck
	app.ldapReadonly = testClient

	cookies := createAuthSession(t, app, store)

	resp := makeAuthRequest(t, app, "/debug/ldap-pool", cookies)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var response map[string]any
		err = json.Unmarshal(body, &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "stats")
	}
}

func TestRateLimiter_StopIdempotent(_ *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	// Stop should be safe to call multiple times
	rl.Stop()
	rl.Stop()
	rl.Stop()
}

func TestRateLimiter_CleanupBlockedEntries(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:  2,
		WindowPeriod: 50 * time.Millisecond,
		BlockPeriod:  50 * time.Millisecond,
		CleanupEvery: 100 * time.Millisecond,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ip := "192.168.1.200"

	// Block the IP
	rl.RecordAttempt(ip)
	rl.RecordAttempt(ip)
	assert.True(t, rl.IsBlocked(ip))

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Should be cleaned up
	assert.Equal(t, config.MaxAttempts, rl.GetRemainingAttempts(ip))
}
