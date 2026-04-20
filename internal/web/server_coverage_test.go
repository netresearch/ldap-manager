package web

// Additional server.go coverage tests that do not require a live LDAP server.
//
// These tests exercise NewApp, createFiberApp, setupMiddleware, setupRoutes,
// Listen, Shutdown, and the shutdown-path error branches by running the
// application in "no service account" mode (opts.ReadonlyUser == "").
// The heavy integration paths remain covered by the LDAP-backed integration
// suite in ldap_integration_test.go; these tests fill in the pure wiring.

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/options"
)

// newAppForCoverage creates a fully-wired *App without any real LDAP
// connection. Using `readonly-user = ""` skips the ldap.New() call inside
// NewApp so the test does not attempt to dial a directory server.
//
// The returned App is registered with t.Cleanup so its background goroutines
// (periodicCacheLogging, template-cache cleanup, rate-limiter cleanup) are
// shut down when the test ends. This prevents goroutine leaks that could
// cause flakes in -race mode across the test binary's lifetime.
func newAppForCoverage(t *testing.T) (*App, string) {
	t.Helper()

	tmp := t.TempDir()

	// Ensure NewApp can locate the asset manifest. It looks relative to CWD.
	chdirToRepoRoot(t)

	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://127.0.0.1:389",
			BaseDN:            "dc=example,dc=com",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "",
		ReadonlyPassword:        "",
		PersistSessions:         false,
		SessionPath:             filepath.Join(tmp, "sess.bbolt"),
		SessionDuration:         30 * time.Minute,
		CookieSecure:            false,
		TLSSkipVerify:           false,
		PoolMaxConnections:      2,
		PoolMinConnections:      1,
		PoolMaxIdleTime:         time.Minute,
		PoolMaxLifetime:         time.Hour,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   time.Second,
		PoolAcquireTimeout:      time.Second,
	}

	app, err := NewApp(opts)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	registerAppShutdown(t, app)

	return app, tmp
}

// registerAppShutdown arranges for app.Shutdown to run when the test ends,
// guarding against double-close of app.stopCacheLog when a test also calls
// Shutdown explicitly. Safe to call once per App.
func registerAppShutdown(t *testing.T, app *App) {
	t.Helper()

	t.Cleanup(func() {
		// Defensive: Shutdown closes stopCacheLog unconditionally. If a test
		// already called Shutdown, avoid panicking on double-close.
		defer func() { _ = recover() }()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = app.Shutdown(ctx)
	})
}

// chdirToRepoRoot changes the working directory so the "internal/web/static/manifest.json"
// relative path used by NewApp is resolvable from the test binary's cwd.
// The test binary runs in internal/web/, so the repo root is two levels up.
func chdirToRepoRoot(t *testing.T) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	// Walk up until we find go.mod or reach the filesystem root.
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if err := os.Chdir(dir); err != nil {
				t.Fatalf("chdir to repo root %s: %v", dir, err)
			}

			t.Cleanup(func() { _ = os.Chdir(cwd) })

			return
		}
	}

	t.Fatalf("could not locate repo root from %s", cwd)
}

func TestNewApp_NoServiceAccount(t *testing.T) {
	app, _ := newAppForCoverage(t)

	if app == nil {
		t.Fatal("expected non-nil app")
	}

	if app.ldapReadonly != nil {
		t.Error("ldapReadonly should be nil without service account")
	}

	if app.ldapCache != nil {
		t.Error("ldapCache should be nil without service account")
	}

	if app.fiber == nil {
		t.Fatal("fiber should be initialized")
	}

	if app.sessionStore == nil {
		t.Fatal("sessionStore should be initialized")
	}

	if app.templateCache == nil {
		t.Fatal("templateCache should be initialized")
	}

	if app.rateLimiter == nil {
		t.Fatal("rateLimiter should be initialized")
	}

	if app.assetManifest == nil {
		t.Fatal("assetManifest should be initialized")
	}
}

func TestNewApp_WithPersistSessions(t *testing.T) {
	chdirToRepoRoot(t)
	tmp := t.TempDir()

	opts := &options.Opts{
		LDAP: ldap.Config{
			Server: "ldap://127.0.0.1:389",
			BaseDN: "dc=example,dc=com",
		},
		ReadonlyUser:            "",
		ReadonlyPassword:        "",
		PersistSessions:         true,
		SessionPath:             filepath.Join(tmp, "sess.bbolt"),
		SessionDuration:         5 * time.Minute,
		CookieSecure:            false,
		PoolMaxConnections:      2,
		PoolMinConnections:      1,
		PoolMaxIdleTime:         time.Minute,
		PoolMaxLifetime:         time.Hour,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   time.Second,
		PoolAcquireTimeout:      time.Second,
	}

	app, err := NewApp(opts)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	// Shutdown cleanly so the bbolt file is released. registerAppShutdown is
	// not used here because this test asserts on the Shutdown return value.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestNewApp_TLSSkipVerify(t *testing.T) {
	chdirToRepoRoot(t)
	tmp := t.TempDir()

	opts := &options.Opts{
		LDAP: ldap.Config{
			Server: "ldaps://127.0.0.1:636",
			BaseDN: "dc=example,dc=com",
		},
		ReadonlyUser:            "",
		ReadonlyPassword:        "",
		PersistSessions:         false,
		SessionPath:             filepath.Join(tmp, "sess.bbolt"),
		SessionDuration:         5 * time.Minute,
		CookieSecure:            true,
		TLSSkipVerify:           true,
		PoolMaxConnections:      2,
		PoolMinConnections:      1,
		PoolMaxIdleTime:         time.Minute,
		PoolMaxLifetime:         time.Hour,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   time.Second,
		PoolAcquireTimeout:      time.Second,
	}

	app, err := NewApp(opts)
	if err != nil {
		t.Fatalf("NewApp with TLSSkipVerify: %v", err)
	}

	registerAppShutdown(t, app)

	if len(app.ldapOpts) < 2 {
		t.Errorf("expected at least 2 LDAP opts with TLS skip verify, got %d", len(app.ldapOpts))
	}
}

func TestCreateFiberApp_HasExpectedConfig(t *testing.T) {
	app := createFiberApp()
	if app == nil {
		t.Fatal("createFiberApp returned nil")
	}

	// Ensure the error handler is set (smoke test).
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/unregistered-path", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// The app has no routes registered so the 404 handler runs.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unregistered path, got %d", resp.StatusCode)
	}
}

func TestApp_RoutesRegistered(t *testing.T) {
	app, _ := newAppForCoverage(t)

	// Verify routes respond with their expected status codes. Protected routes
	// redirect to /login (302) because we have no session; that still proves
	// the route is registered. Health endpoints answer 200 directly.
	routes := []struct {
		method    string
		path      string
		wantCodes []int // accept any of these status codes
	}{
		// Protected routes without auth → 302 redirect to /login
		{http.MethodGet, "/", []int{http.StatusFound}},
		{http.MethodGet, "/users", []int{http.StatusFound}},
		{http.MethodGet, "/groups", []int{http.StatusFound}},
		{http.MethodGet, "/computers", []int{http.StatusFound}},
		{http.MethodGet, "/logout", []int{http.StatusFound}},
		{http.MethodGet, "/debug/cache", []int{http.StatusFound}},
		{http.MethodGet, "/debug/ldap-pool", []int{http.StatusFound}},
		// Login renders the form on GET → 200
		{http.MethodGet, "/login", []int{http.StatusOK}},
		// Health endpoints are public and respond 200 when components are healthy,
		// or 503 when the LDAP cache is not ready (the latter is the default in
		// this no-service-account test fixture).
		{http.MethodGet, "/health", []int{http.StatusOK, http.StatusServiceUnavailable}},
		{http.MethodGet, "/health/ready", []int{http.StatusOK, http.StatusServiceUnavailable}},
		{http.MethodGet, "/health/live", []int{http.StatusOK, http.StatusServiceUnavailable}},
	}

	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), r.method, r.path, http.NoBody)

			resp, err := app.fiber.Test(req)
			if err != nil {
				t.Fatalf("route %s %s: %v", r.method, r.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if !intInSlice(resp.StatusCode, r.wantCodes) {
				t.Errorf("route %s %s: got status %d, want one of %v",
					r.method, r.path, resp.StatusCode, r.wantCodes)
			}
		})
	}
}

// intInSlice reports whether needle is in haystack.
func intInSlice(needle int, haystack []int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}

	return false
}

func TestApp_ListenGracefulShutdown(t *testing.T) {
	app, _ := newAppForCoverage(t)

	// Reserve an ephemeral port to know what Listen will bind to. Close the
	// reservation before Listen runs so Listen can claim the port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := l.Addr().String()
	_ = l.Close()

	// App.Listen forwards to fiber.Listen which does NOT observe the passed
	// context (see server.go). Graceful shutdown is triggered by Shutdown(),
	// not by ctx cancellation, so we use a Background context here to avoid
	// suggesting otherwise.
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(context.Background(), addr) }()

	// Give Listen a moment to bind, then trigger graceful shutdown.
	time.Sleep(150 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		t.Logf("Shutdown returned: %v", err)
	}

	select {
	case err := <-errCh:
		// fiber.Listen returns nil when shut down cleanly.
		if err != nil {
			t.Logf("Listen returned err=%v (nil after shutdown is expected)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Listen did not return within 2s after Shutdown")
	}
}

// TestHandle500_FiberUnauthorizedRedirects verifies that a wrapped
// fiber.StatusUnauthorized error is translated into a /login redirect.
func TestHandle500_FiberUnauthorizedRedirects(t *testing.T) {
	f := fiber.New()
	f.Get("/x", func(c *fiber.Ctx) error {
		return handle500(c, fiber.NewError(fiber.StatusUnauthorized, "session expired"))
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody)

	resp, err := f.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect for unauthorized, got %d", resp.StatusCode)
	}

	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}
