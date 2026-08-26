package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/options"
)

// setupCSRFBulkTestApp builds a test app whose list GET and bulk POST
// routes run behind the REAL CSRF middleware in production order
// (RequireAuth → csrf), unlike setupFullTestApp which strips CSRF.
// This is the harness gap that let issue #652 ship: the bulk toolbar
// posted without a csrf_token and no test noticed.
func setupCSRFBulkTestApp(t *testing.T) (*App, *session.Store) {
	t.Helper()

	mockClient := &testLDAPClient{
		users: []ldap.User{
			{SAMAccountName: "john.doe", Enabled: true},
		},
		groups: []ldap.Group{
			{Members: []string{"cn=john.doe,ou=users,dc=example,dc=com"}},
		},
		computers: []ldap.Computer{
			{SAMAccountName: "workstation-01$", Enabled: true},
		},
	}

	store := session.NewStore(session.Config{
		Storage: memory.New(),
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	// Minutes-scale TTL on purpose: TestBulkToolbar_ListPageIsNeverServedFromCache
	// asserts the second GET is a MISS because nothing STORES list pages.
	// With a millisecond TTL, a stored-but-expired entry would also read
	// as MISS and the guard would silently pass over the very mutation
	// (RenderWithCache on a list handler) it exists to catch.
	templateCache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      5 * time.Minute,
		MaxSize:         100,
		CleanupInterval: time.Minute,
	})

	pinnedDB, err := bolt.Open(filepath.Join(t.TempDir(), "pinned.bbolt"), 0o600, nil)
	require.NoError(t, err)
	pinnedStore, err := NewPinnedStore(pinnedDB)
	require.NoError(t, err)

	app := &App{
		ldapConfig: ldap.Config{
			Server: "ldap://test.server.com",
			Port:   389,
			BaseDN: "dc=test,dc=com",
		},
		ldapCache:     ldap_cache.New(mockClient),
		sessionStore:  store,
		templateCache: templateCache,
		fiber:         f,
		stopCacheLog:  make(chan struct{}),
		pinnedStore:   pinnedStore,
		pinnedDB:      pinnedDB,
	}

	t.Cleanup(func() {
		templateCache.Stop()
		_ = pinnedDB.Close()
	})

	_ = app.ldapCache.RefreshUsers()
	_ = app.ldapCache.RefreshGroups()
	_ = app.ldapCache.RefreshComputers()

	csrfHandler := createCSRFConfig(&options.Opts{CookieSecure: false}, store)
	protected := f.Group("/", app.RequireAuth(), csrfHandler)
	// templateCacheMiddleware mirrors production's `cacheable` group so
	// TestBulkToolbar_ListPageIsNeverServedFromCache can pin the
	// invariant documented in setupRoutes. All three list pages are
	// registered: each one hosts the bulk toolbar and received the same
	// data-csrf plumbing.
	protected.Get("/users", app.templateCacheMiddleware(), app.handleUsersV2)
	protected.Get("/groups", app.templateCacheMiddleware(), app.handleGroupsV2)
	protected.Get("/computers", app.templateCacheMiddleware(), app.handleComputersV2)
	protected.Post("/groups/bulk", app.handleBulkGroups)

	return app, store
}

// bulkListPaths are the three list pages that host the bulk toolbar.
var bulkListPaths = []string{"/users", "/groups", "/computers"}

var dataCSRFRe = regexp.MustCompile(`data-csrf="([^"]+)"`)

// TestBulkToolbar_ListPageExposesUsableCSRFToken reproduces issue #652:
// the bulk toolbar (v2-bulk.js) builds its POST form from the list page
// DOM, so the list page MUST expose the current CSRF token via the
// data-csrf attribute on main[data-bulk-scope] — and that token must be
// accepted by the CSRF middleware on the /groups/bulk POST. Before the
// fix, the list page carried no token at all and every bulk action
// died with 403 "CSRF token validation failed".
func TestBulkToolbar_ListPageExposesUsableCSRFToken(t *testing.T) {
	app, store := setupCSRFBulkTestApp(t)

	cookies := createAuthSession(t, app, store)

	// Step 1: every list page hosting the bulk toolbar must expose the
	// token the toolbar JS reads — not just /groups, since all three
	// received the same data-csrf plumbing.
	var token string
	for _, path := range bulkListPaths {
		getReq := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		for _, c := range cookies {
			getReq.AddCookie(c)
		}
		getResp, err := app.fiber.Test(getReq)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, getResp.StatusCode, "GET %s", path)

		body, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)
		_ = getResp.Body.Close()

		m := dataCSRFRe.FindSubmatch(body)
		require.NotNil(t, m,
			"%s must expose the CSRF token as data-csrf on main[data-bulk-scope] — "+
				"without it the bulk toolbar cannot POST (issue #652)", path)
		require.NotEmpty(t, string(m[1]), "data-csrf on %s", path)

		// CSRF storage is session-backed: one token per session, so all
		// three pages must render the SAME token. This also closes the
		// wrong-but-non-empty mutation (e.g. a hardcoded string in one
		// handler) that per-path NotEmpty alone would let survive.
		if token != "" {
			require.Equal(t, token, string(m[1]), "token diverged on %s", path)
		}
		token = string(m[1])
		cookies = append(cookies, getResp.Cookies()...)
	}

	// Step 2: POST a bulk action with that token, carrying the session
	// cookie plus the csrf_ cookies collected from the GETs above. A
	// user_dn with zero target_dn values keeps the handler away from
	// LDAP: it redirects straight back to /groups — any non-403 proves
	// the CSRF middleware accepted the token.
	form := url.Values{
		"csrf_token": {token},
		"user_dn":    {"cn=u,ou=users,dc=test,dc=com"},
	}
	postReq := httptest.NewRequest(http.MethodPost,
		"/groups/bulk?action=add-members",
		strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		postReq.AddCookie(c)
	}

	postResp, err := app.fiber.Test(postReq)
	require.NoError(t, err)
	defer func() { _ = postResp.Body.Close() }()

	require.NotEqual(t, http.StatusForbidden, postResp.StatusCode,
		"bulk POST with the page-exposed token must pass CSRF validation")
	require.Equal(t, http.StatusSeeOther, postResp.StatusCode)
}

// TestBulkToolbar_ListPageIsNeverServedFromCache pins the invariant
// documented in setupRoutes: list pages carry a session-scoped CSRF
// token in data-csrf, so their HTML must never be STORED in the
// template cache. The middleware only serves entries and nothing
// stores list renders — a second GET must therefore be a cache MISS.
// If a future change makes a list handler store via RenderWithCache,
// this reddens before stale-token 403s reach users.
func TestBulkToolbar_ListPageIsNeverServedFromCache(t *testing.T) {
	app, store := setupCSRFBulkTestApp(t)

	cookies := createAuthSession(t, app, store)

	for _, path := range bulkListPaths {
		for i := 1; i <= 2; i++ {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			for _, c := range cookies {
				req.AddCookie(c)
			}

			resp, err := app.fiber.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s", path)
			require.Equal(t, "MISS", resp.Header.Get("X-Cache"),
				"GET #%d %s: list pages embed a session-scoped CSRF token and must not be cached", i, path)
			_ = resp.Body.Close()
		}
	}
}

// TestBulkToolbar_PostWithoutTokenIs403 pins the counterpart: the same
// route chain rejects a token-less bulk POST. This is the pre-fix
// behaviour of v2-bulk.js and proves the harness actually runs the CSRF
// middleware (setupFullTestApp does not, which is how #652 escaped).
func TestBulkToolbar_PostWithoutTokenIs403(t *testing.T) {
	app, store := setupCSRFBulkTestApp(t)

	cookies := createAuthSession(t, app, store)

	form := url.Values{"target_dn": {"cn=g,dc=test,dc=com"}, "user_dn": {"cn=u,dc=test,dc=com"}}
	req := httptest.NewRequest(http.MethodPost,
		"/groups/bulk?action=add-members",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}
