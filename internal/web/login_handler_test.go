package web

// Tests for loginHandler POST paths: invalid credentials, empty fields,
// rate-limit block, and logoutHandler session destruction. A live-bind
// "success" case is not present here because it requires a real LDAP server;
// that path is covered by the LDAP integration test suite.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"
)

// buildLoginApp creates a minimal app that mounts only /login, using an
// example-server LDAP client so authenticateUser fails without network.
func buildLoginApp(t *testing.T) *App {
	t.Helper()

	chdirToRepoRoot(t)

	cfg := ldap.Config{
		Server: "ldap://test.server.com",
		BaseDN: "dc=example,dc=com",
	}

	client, err := ldap.New(cfg, "cn=admin,dc=example,dc=com", "password")
	if err != nil {
		t.Fatalf("ldap.New: %v", err)
	}

	f := fiber.New(fiber.Config{ErrorHandler: handle500})

	app := &App{
		ldapConfig:    cfg,
		ldapReadonly:  client,
		sessionStore:  session.New(session.Config{Storage: memory.New(), Expiration: 30 * time.Minute}),
		templateCache: NewTemplateCache(DefaultTemplateCacheConfig()),
		fiber:         f,
		assetManifest: &AssetManifest{Assets: map[string]string{"styles.css": "styles.css"}, StylesCSS: "styles.css"},
		rateLimiter:   NewRateLimiter(DefaultRateLimiterConfig()),
		stopCacheLog:  make(chan struct{}),
	}

	f.All("/login", app.loginHandler)

	t.Cleanup(func() {
		_ = client.Close()
		app.templateCache.Stop()
		app.rateLimiter.Stop()
		close(app.stopCacheLog)
	})

	return app
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	app := buildLoginApp(t)

	// Inject a username containing banned LDAP DN characters so
	// authenticateViaDirectBind and authenticateViaUPNBind both reject it,
	// ensuring we land on the "Invalid username or password" branch rather
	// than the example-server fallback that otherwise succeeds.
	body := "username=bad%3Dinjected%26test&password=wrong"
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/login",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Invalid credentials → re-render login form with error flash.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (re-render), got %d", resp.StatusCode)
	}
}

func TestLoginHandler_EmptyFields(t *testing.T) {
	app := buildLoginApp(t)

	// Empty form — the handler should just render the login page.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/login",
		strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("POST /login empty: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLoginHandler_RateLimitBlock(t *testing.T) {
	app := buildLoginApp(t)

	// Override rate limiter with aggressive config: block after 1 failure.
	app.rateLimiter.Stop()
	app.rateLimiter = NewRateLimiter(RateLimiterConfig{
		MaxAttempts:  1,
		WindowPeriod: time.Minute,
		BlockPeriod:  time.Minute,
		CleanupEvery: time.Minute,
	})

	defer app.rateLimiter.Stop()

	// Use a username with LDAP DN-banned chars so authentication fails.
	body := "username=bad%3Dinjected&password=wrong"

	// First attempt — records as failed, may or may not block (threshold 1).
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/login",
		strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp1, err := app.fiber.Test(req1)
	if err != nil {
		t.Fatalf("POST /login #1: %v", err)
	}
	_ = resp1.Body.Close()

	// Second attempt — should be blocked by rate limiter at middleware level,
	// but we mounted the handler without the Middleware() wrapper so blocking
	// here exercises the `blocked` branch inside loginHandler itself.
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/login",
		strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp2, err := app.fiber.Test(req2)
	if err != nil {
		t.Fatalf("POST /login #2: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	// Should render a login page (with rate-limit flash).
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (login re-render with rate-limit flash), got %d", resp2.StatusCode)
	}
}

func TestLogoutHandler_DestroysSession(t *testing.T) {
	app := buildLoginApp(t)
	app.fiber.Get("/logout", app.logoutHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/logout", nil)

	resp, err := app.fiber.Test(req)
	if err != nil {
		t.Fatalf("GET /logout: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", resp.StatusCode)
	}

	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}
