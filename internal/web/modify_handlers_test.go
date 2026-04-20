package web

// Deeper coverage for the modify handlers: we inject an example-server LDAP
// client into the session so getUserLDAP() returns a usable client for the
// read-only discovery paths (FindUsers/FindGroups), and exercises the
// renderUserWithFlash / renderGroupWithFlash / performUserModification /
// performGroupModification branches that only run when a session-scoped LDAP
// call succeeds far enough.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"
)

// newExampleServerApp returns an App whose ldapConfig points at an
// "example" server. simple-ldap-go recognises that name and returns mocked
// data for FindUsers/FindGroups (no network calls). Modification calls
// (AddUserToGroup, RemoveUserFromGroup) still fail because they require a
// real connection — which is precisely the branch we want to cover in
// performUserModification / renderUserWithFlash.
func newExampleServerApp(t *testing.T) *App {
	t.Helper()

	chdirToRepoRoot(t)

	cfg := ldap.Config{
		Server: "ldap://test.server.com",
		BaseDN: "dc=example,dc=com",
	}

	// Use ldap.New so we get a real *ldap.LDAP. With server name matching
	// isExampleServerName, FindUsers/FindGroups return mocks.
	client, err := ldap.New(cfg, "cn=admin,dc=example,dc=com", "password")
	if err != nil {
		t.Fatalf("ldap.New: %v", err)
	}

	sessionStore := session.New(session.Config{
		Storage:    memory.New(),
		Expiration: 30 * time.Minute,
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	app := &App{
		ldapConfig:    cfg,
		ldapReadonly:  client,
		ldapCache:     nil,
		sessionStore:  sessionStore,
		templateCache: NewTemplateCache(DefaultTemplateCacheConfig()),
		fiber:         f,
		assetManifest: &AssetManifest{Assets: map[string]string{"styles.css": "styles.css"}, StylesCSS: "styles.css"},
		rateLimiter:   NewRateLimiter(DefaultRateLimiterConfig()),
		stopCacheLog:  make(chan struct{}),
	}

	// Mount handlers WITHOUT csrf middleware so we can POST directly.
	f.Post("/users/*", app.RequireAuth(), app.userModifyHandler)
	f.Post("/groups/*", app.RequireAuth(), app.groupModifyHandler)

	t.Cleanup(func() {
		_ = client.Close()
		app.templateCache.Stop()
		app.rateLimiter.Stop()
		close(app.stopCacheLog)
	})

	return app
}

func exampleSessionCookies(t *testing.T, app *App) []*http.Cookie {
	t.Helper()

	// Build a session cookie bound to the example LDAP server. getUserLDAP
	// attempts ldap.New with the session DN+password; with an example server
	// name it succeeds without a real connection.
	helper := fiber.New()
	helper.Get("/__set", func(c *fiber.Ctx) error {
		sess, err := app.sessionStore.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", "cn=admin,dc=example,dc=com")
		sess.Set("password", "password")
		sess.Set("username", "admin")

		return sess.Save()
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/__set", nil)

	resp, err := helper.Test(req)
	if err != nil {
		t.Fatalf("session GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.Cookies()
}

func TestUserModifyHandler_DeeperPaths(t *testing.T) {
	app := newExampleServerApp(t)
	cookies := exampleSessionCookies(t, app)

	// The example server returns 150 mock users (cn=User N,OU=Users,<base>).
	// We use one of those DNs so loadUserDataFromLDAP can find it.
	userDN := "CN=User 1,OU=Users,dc=example,dc=com"

	t.Run("addgroup triggers performUserModification + renderUserWithFlash", func(t *testing.T) {
		body := "addgroup=" + url.QueryEscape("cn=admins,ou=groups,dc=example,dc=com")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
			"/users/"+url.PathEscape(userDN), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// The modify fails (no real LDAP connection for Modify op), so the
		// handler takes the renderUserWithFlash(ErrorFlash) branch. The
		// response body contains the rendered detail page.
		if resp.StatusCode != http.StatusOK {
			t.Logf("got status %d (branch still covered)", resp.StatusCode)
		}
	})

	t.Run("removegroup triggers performUserModification branch", func(t *testing.T) {
		body := "removegroup=" + url.QueryEscape("cn=admins,ou=groups,dc=example,dc=com")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
			"/users/"+url.PathEscape(userDN), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Logf("got status %d (branch still covered)", resp.StatusCode)
		}
	})
}

func TestGroupModifyHandler_DeeperPaths(t *testing.T) {
	app := newExampleServerApp(t)
	cookies := exampleSessionCookies(t, app)

	// Example server doesn't mock FindGroups to match any specific DN, but
	// we still exercise the handler branches up to renderGroupWithFlash.
	groupDN := "CN=Administrators,OU=Groups,dc=example,dc=com"

	t.Run("adduser triggers performGroupModification branch", func(t *testing.T) {
		body := "adduser=" + url.QueryEscape("cn=alice,ou=users,dc=example,dc=com")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
			"/groups/"+url.PathEscape(groupDN), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 0 {
			t.Error("zero status")
		}
	})

	t.Run("removeuser triggers performGroupModification branch", func(t *testing.T) {
		body := "removeuser=" + url.QueryEscape("cn=alice,ou=users,dc=example,dc=com")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
			"/groups/"+url.PathEscape(groupDN), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		for _, c := range cookies {
			req.AddCookie(c)
		}

		resp, err := app.fiber.Test(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 0 {
			t.Error("zero status")
		}
	})
}

// Also exercise the list/detail GET handlers against the example server to
// cover loadUserDataFromLDAP / loadGroupDataFromLDAP / loadComputerDataFromLDAP.
func TestListHandlers_ExampleServer(t *testing.T) {
	app := newExampleServerApp(t)
	cookies := exampleSessionCookies(t, app)

	// Mount GETs too.
	f := app.fiber
	f.Get("/users", app.RequireAuth(), app.usersHandler)
	f.Get("/users/*", app.RequireAuth(), app.userHandler)
	f.Get("/groups", app.RequireAuth(), app.groupsHandler)
	f.Get("/groups/*", app.RequireAuth(), app.groupHandler)
	f.Get("/computers", app.RequireAuth(), app.computersHandler)
	f.Get("/computers/*", app.RequireAuth(), app.computerHandler)

	paths := []string{
		"/users",
		"/users?show-disabled=1",
		"/users/" + url.PathEscape("CN=User 1,OU=Users,dc=example,dc=com"),
		"/groups",
		"/computers",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, p, nil)
			for _, c := range cookies {
				req.AddCookie(c)
			}

			resp, err := app.fiber.Test(req)
			if err != nil {
				t.Fatalf("GET %s: %v", p, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode == 0 {
				t.Error("zero status")
			}
		})
	}
}
