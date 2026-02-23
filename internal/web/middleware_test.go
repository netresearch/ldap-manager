package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorSessionStorage simulates session store failures
type errorSessionStorage struct {
	shouldError bool
	mu          sync.Mutex
}

func (e *errorSessionStorage) Get(_ string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shouldError {
		return nil, errors.New("simulated session store failure")
	}

	return nil, nil
}

func (e *errorSessionStorage) Set(_ string, _ []byte, _ time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shouldError {
		return errors.New("simulated session store failure")
	}

	return nil
}

func (e *errorSessionStorage) Delete(_ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shouldError {
		return errors.New("simulated session store failure")
	}

	return nil
}

func (e *errorSessionStorage) Reset() error {
	return nil
}

func (e *errorSessionStorage) Close() error {
	return nil
}

func (e *errorSessionStorage) SetShouldError(shouldError bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shouldError = shouldError
}

// setupMiddlewareTestApp creates a minimal app for middleware testing
func setupMiddlewareTestApp() (*App, *session.Store) {
	store := session.New(session.Config{
		Storage: memory.New(),
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	app := &App{
		sessionStore: store,
		fiber:        f,
	}

	return app, store
}

// setupMiddlewareTestAppWithErrorStorage creates app with error-prone storage
func setupMiddlewareTestAppWithErrorStorage() (*App, *errorSessionStorage) {
	errStorage := &errorSessionStorage{shouldError: false}
	store := session.New(session.Config{
		Storage: errStorage,
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	app := &App{
		sessionStore: store,
		fiber:        f,
	}

	return app, errStorage
}

// TestRequireAuth_SessionGetError tests behavior when session store fails
func TestRequireAuth_SessionGetError(t *testing.T) {
	app, errStorage := setupMiddlewareTestAppWithErrorStorage()

	// Register a protected route
	app.fiber.Get("/protected", app.RequireAuth(), func(c *fiber.Ctx) error {
		return c.SendString("protected content")
	})

	// Enable errors after route registration
	errStorage.SetShouldError(true)

	req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to login on session error
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

// TestRequireAuth_FreshSession tests redirect for new/fresh sessions
func TestRequireAuth_FreshSession(t *testing.T) {
	app, _ := setupMiddlewareTestApp()

	// Register a protected route
	app.fiber.Get("/protected", app.RequireAuth(), func(c *fiber.Ctx) error {
		return c.SendString("protected content")
	})

	// Request without any session cookie - session will be fresh
	req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Fresh session should redirect to login
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))
}

// TestRequireAuth_EmptyDN tests handling of sessions with empty DN
func TestRequireAuth_EmptyDN(t *testing.T) {
	app, store := setupMiddlewareTestApp()

	// Set up route that creates session with empty DN
	app.fiber.Get("/set-empty-session", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "") // Empty DN

		return sess.Save()
	})

	app.fiber.Get("/protected", app.RequireAuth(), func(c *fiber.Ctx) error {
		return c.SendString("protected content")
	})

	// First set the session with empty DN
	req1 := httptest.NewRequest(http.MethodGet, "/set-empty-session", http.NoBody)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	// Get the session cookie
	cookies := resp1.Cookies()
	require.NotEmpty(t, cookies, "Expected session cookie")

	// Now try to access protected route with empty DN session
	req2 := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	// Empty DN should redirect to login
	assert.Equal(t, http.StatusFound, resp2.StatusCode)
	assert.Equal(t, "/login", resp2.Header.Get("Location"))
}

// TestRequireAuth_ValidSession tests successful authentication
func TestRequireAuth_ValidSession(t *testing.T) {
	app, store := setupMiddlewareTestApp()

	// Set up route that creates valid session
	app.fiber.Get("/login-test", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "cn=testuser,ou=users,dc=example,dc=com")

		return sess.Save()
	})

	var capturedDN string
	app.fiber.Get("/protected", app.RequireAuth(), func(c *fiber.Ctx) error {
		capturedDN = GetUserDN(c)

		return c.SendString("protected content")
	})

	// First set the valid session
	req1 := httptest.NewRequest(http.MethodGet, "/login-test", http.NoBody)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	cookies := resp1.Cookies()
	require.NotEmpty(t, cookies)

	// Now access protected route
	req2 := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	// Should get 200 OK and have userDN in context
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, "cn=testuser,ou=users,dc=example,dc=com", capturedDN)
}

// TestRequireAuth_CorruptedSessionData tests handling of non-string DN
func TestRequireAuth_CorruptedSessionData(t *testing.T) {
	app, store := setupMiddlewareTestApp()

	// Set up route that creates session with wrong type for DN
	app.fiber.Get("/corrupt-session", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", 12345) // Integer instead of string

		return sess.Save()
	})

	app.fiber.Get("/protected", app.RequireAuth(), func(c *fiber.Ctx) error {
		return c.SendString("protected content")
	})

	// First set the corrupted session
	req1 := httptest.NewRequest(http.MethodGet, "/corrupt-session", http.NoBody)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	cookies := resp1.Cookies()
	require.NotEmpty(t, cookies)

	// Now try to access protected route
	req2 := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	// Corrupted (non-string) DN should redirect to login
	assert.Equal(t, http.StatusFound, resp2.StatusCode)
	assert.Equal(t, "/login", resp2.Header.Get("Location"))
}

// TestOptionalAuth_NoSession tests OptionalAuth with no session
func TestOptionalAuth_NoSession(t *testing.T) {
	app, _ := setupMiddlewareTestApp()

	var capturedDN string
	app.fiber.Get("/optional", app.OptionalAuth(), func(c *fiber.Ctx) error {
		capturedDN = GetUserDN(c)

		return c.SendString("content")
	})

	req := httptest.NewRequest(http.MethodGet, "/optional", http.NoBody)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Should succeed but with empty userDN
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, capturedDN)
}

// TestOptionalAuth_WithValidSession tests OptionalAuth with valid session
func TestOptionalAuth_WithValidSession(t *testing.T) {
	app, store := setupMiddlewareTestApp()

	// Set up route that creates valid session
	app.fiber.Get("/login-test", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "cn=testuser,ou=users,dc=example,dc=com")

		return sess.Save()
	})

	var capturedDN string
	app.fiber.Get("/optional", app.OptionalAuth(), func(c *fiber.Ctx) error {
		capturedDN = GetUserDN(c)

		return c.SendString("content")
	})

	// First set the valid session
	req1 := httptest.NewRequest(http.MethodGet, "/login-test", http.NoBody)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	cookies := resp1.Cookies()

	// Now access optional route
	req2 := httptest.NewRequest(http.MethodGet, "/optional", http.NoBody)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, "cn=testuser,ou=users,dc=example,dc=com", capturedDN)
}

// TestOptionalAuth_SessionError tests OptionalAuth graceful handling of errors
func TestOptionalAuth_SessionError(t *testing.T) {
	app, errStorage := setupMiddlewareTestAppWithErrorStorage()

	var handlerCalled bool
	app.fiber.Get("/optional", app.OptionalAuth(), func(c *fiber.Ctx) error {
		handlerCalled = true

		return c.SendString("content")
	})

	errStorage.SetShouldError(true)

	req := httptest.NewRequest(http.MethodGet, "/optional", http.NoBody)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// OptionalAuth should continue without auth context on error
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, handlerCalled)
}

// TestOptionalAuth_EmptyDN tests OptionalAuth with empty DN
func TestOptionalAuth_EmptyDN(t *testing.T) {
	app, store := setupMiddlewareTestApp()

	app.fiber.Get("/set-empty", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "")

		return sess.Save()
	})

	var capturedDN string
	app.fiber.Get("/optional", app.OptionalAuth(), func(c *fiber.Ctx) error {
		capturedDN = GetUserDN(c)

		return c.SendString("content")
	})

	// Set empty session
	req1 := httptest.NewRequest(http.MethodGet, "/set-empty", http.NoBody)
	resp1, err := app.fiber.Test(req1)
	require.NoError(t, err)
	_ = resp1.Body.Close()

	cookies := resp1.Cookies()

	// Access optional route
	req2 := httptest.NewRequest(http.MethodGet, "/optional", http.NoBody)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	// Should continue but without setting userDN
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Empty(t, capturedDN)
}

// TestGetUserDN_NoContext tests GetUserDN with no user in context
func TestGetUserDN_NoContext(t *testing.T) {
	app := fiber.New()

	var result string
	app.Get("/test", func(c *fiber.Ctx) error {
		result = GetUserDN(c)

		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Empty(t, result)
}

// TestGetUserDN_WithValidContext tests GetUserDN with valid user context
func TestGetUserDN_WithValidContext(t *testing.T) {
	app := fiber.New()

	var result string
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("userDN", "cn=test,dc=example,dc=com")
		result = GetUserDN(c)

		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "cn=test,dc=example,dc=com", result)
}

// TestGetUserDN_WrongType tests GetUserDN with wrong type in context
func TestGetUserDN_WrongType(t *testing.T) {
	app := fiber.New()

	var result string
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("userDN", 12345) // Wrong type
		result = GetUserDN(c)

		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Empty(t, result)
}

// TestRequireUserDN_Success tests successful RequireUserDN call
func TestRequireUserDN_Success(t *testing.T) {
	app := fiber.New()

	var resultDN string
	var resultErr error
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("userDN", "cn=test,dc=example,dc=com")
		resultDN, resultErr = RequireUserDN(c)

		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.NoError(t, resultErr)
	assert.Equal(t, "cn=test,dc=example,dc=com", resultDN)
}

// TestRequireUserDN_NoContext tests RequireUserDN without user context
func TestRequireUserDN_NoContext(t *testing.T) {
	app := fiber.New()

	var resultDN string
	var resultErr error
	app.Get("/test", func(c *fiber.Ctx) error {
		resultDN, resultErr = RequireUserDN(c)
		if resultErr != nil {
			return resultErr
		}

		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Error(t, resultErr)
	assert.Empty(t, resultDN)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestRequireUserDN_EmptyDN tests RequireUserDN with empty DN in context
func TestRequireUserDN_EmptyDN(t *testing.T) {
	app := fiber.New()

	var resultDN string
	var resultErr error
	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("userDN", "")
		resultDN, resultErr = RequireUserDN(c)
		if resultErr != nil {
			return resultErr
		}

		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Error(t, resultErr)
	assert.Empty(t, resultDN)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestConcurrentSessionAccess tests middleware under concurrent access
func TestConcurrentSessionAccess(t *testing.T) {
	app, store := setupMiddlewareTestApp()

	// Set up login route
	app.fiber.Get("/login-test", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "cn=concurrent,dc=example,dc=com")

		return sess.Save()
	})

	app.fiber.Get("/protected", app.RequireAuth(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Create session first
	req := httptest.NewRequest(http.MethodGet, "/login-test", http.NoBody)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	cookies := resp.Cookies()
	_ = resp.Body.Close()

	// Concurrent access
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for range 10 {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
			resp, err := app.fiber.Test(req)
			if err != nil {
				errors <- err

				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- fiber.NewError(resp.StatusCode, "unexpected status")
			}
		})
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}
