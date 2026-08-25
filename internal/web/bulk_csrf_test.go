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

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
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
		groups: []ldap.Group{
			{Members: []string{"cn=john.doe,ou=users,dc=example,dc=com"}},
		},
	}

	store := session.New(session.Config{
		Storage: memory.New(),
	})

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	templateCache := NewTemplateCache(TemplateCacheConfig{
		DefaultTTL:      100 * time.Millisecond,
		MaxSize:         100,
		CleanupInterval: 50 * time.Millisecond,
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

	_ = app.ldapCache.RefreshGroups()

	csrfHandler := createCSRFConfig(&options.Opts{CookieSecure: false}, store)
	protected := f.Group("/", app.RequireAuth(), csrfHandler)
	// templateCacheMiddleware mirrors production's `cacheable` group so
	// TestBulkToolbar_ListPageIsNeverServedFromCache can pin the
	// invariant documented in setupRoutes.
	protected.Get("/groups", app.templateCacheMiddleware(), app.handleGroupsV2)
	protected.Post("/groups/bulk", app.handleBulkGroups)

	return app, store
}

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

	// Step 1: GET the groups list page like the browser does.
	getReq := httptest.NewRequest(http.MethodGet, "/groups", http.NoBody)
	for _, c := range cookies {
		getReq.AddCookie(c)
	}
	getResp, err := app.fiber.Test(getReq)
	require.NoError(t, err)
	defer func() { _ = getResp.Body.Close() }()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	body, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)

	// Step 2: read the token the bulk toolbar JS reads.
	m := dataCSRFRe.FindSubmatch(body)
	require.NotNil(t, m,
		"list page must expose the CSRF token as data-csrf on main[data-bulk-scope] — "+
			"without it the bulk toolbar cannot POST (issue #652)")
	token := string(m[1])
	require.NotEmpty(t, token)

	// Step 3: POST a bulk action with that token, carrying the session
	// cookie plus the csrf_ cookie the GET set. A user_dn with zero
	// target_dn values keeps the handler away from LDAP: it redirects
	// straight back to /groups — any non-403 proves the CSRF middleware
	// accepted the token.
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
	for _, c := range getResp.Cookies() {
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

	for i, want := range []string{"MISS", "MISS"} {
		req := httptest.NewRequest(http.MethodGet, "/groups", http.NoBody)
		for _, c := range cookies {
			req.AddCookie(c)
		}

		resp, err := app.fiber.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, want, resp.Header.Get("X-Cache"),
			"GET #%d: list pages embed a session-scoped CSRF token and must not be cached", i+1)
		_ = resp.Body.Close()
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
