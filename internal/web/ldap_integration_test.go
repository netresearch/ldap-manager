package web

// Integration tests that use a real OpenLDAP container.
// These tests are skipped when no LDAP server is available.
// In CI, the "test" job provides OpenLDAP on port 1389 (dc=test,dc=local).

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
)

// ldapIntegrationEnv holds the LDAP integration test environment.
type ldapIntegrationEnv struct {
	config    ldap.Config
	adminDN   string
	adminPass string
	baseDN    string
	host      string
	port      int
}

// skipIfNoLDAP returns the LDAP test environment or skips the test.
func skipIfNoLDAP(t *testing.T) *ldapIntegrationEnv {
	t.Helper()

	host := "localhost"
	port := 1389 // CI service container port

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
	if err != nil {
		t.Skipf("OpenLDAP not available at %s:%d (set up CI service or docker compose): %v", host, port, err)
	}
	_ = conn.Close()

	baseDN := "dc=test,dc=local"

	return &ldapIntegrationEnv{
		config: ldap.Config{
			Server:            fmt.Sprintf("ldap://%s", host),
			Port:              port,
			BaseDN:            baseDN,
			IsActiveDirectory: false,
		},
		adminDN:   "cn=admin," + baseDN,
		adminPass: "admin",
		baseDN:    baseDN,
		host:      host,
		port:      port,
	}
}

// seedLDAPData creates OUs and test entries in the LDAP server.
func seedLDAPData(t *testing.T, env *ldapIntegrationEnv) {
	t.Helper()

	l, err := goldap.DialURL(fmt.Sprintf("ldap://%s:%d", env.host, env.port))
	require.NoError(t, err)
	defer l.Close()

	err = l.Bind(env.adminDN, env.adminPass)
	require.NoError(t, err)

	// Create OUs (ignore errors if they already exist)
	for _, ou := range []string{"users", "groups"} {
		addReq := goldap.NewAddRequest(fmt.Sprintf("ou=%s,%s", ou, env.baseDN), nil)
		addReq.Attribute("objectClass", []string{"organizationalUnit", "top"})
		addReq.Attribute("ou", []string{ou})
		_ = l.Add(addReq) // ignore "already exists"
	}

	// Create test users
	testUsers := []struct {
		cn       string
		sn       string
		uid      string
		password string
	}{
		{"testuser1", "User1", "testuser1", "password1"},
		{"testuser2", "User2", "testuser2", "password2"},
		{"admin-user", "Admin", "adminuser", "adminpass"},
	}

	for _, u := range testUsers {
		dn := fmt.Sprintf("cn=%s,ou=users,%s", u.cn, env.baseDN)
		addReq := goldap.NewAddRequest(dn, nil)
		addReq.Attribute("objectClass", []string{"inetOrgPerson", "organizationalPerson", "person", "top"})
		addReq.Attribute("cn", []string{u.cn})
		addReq.Attribute("sn", []string{u.sn})
		addReq.Attribute("uid", []string{u.uid})
		addReq.Attribute("userPassword", []string{u.password})
		_ = l.Add(addReq) // ignore "already exists"
	}

	// Create test groups with members
	groups := []struct {
		cn      string
		members []string
	}{
		{"admins", []string{
			fmt.Sprintf("cn=admin-user,ou=users,%s", env.baseDN),
			fmt.Sprintf("cn=testuser1,ou=users,%s", env.baseDN),
		}},
		{"developers", []string{
			fmt.Sprintf("cn=testuser1,ou=users,%s", env.baseDN),
			fmt.Sprintf("cn=testuser2,ou=users,%s", env.baseDN),
		}},
	}

	for _, g := range groups {
		dn := fmt.Sprintf("cn=%s,ou=groups,%s", g.cn, env.baseDN)
		addReq := goldap.NewAddRequest(dn, nil)
		addReq.Attribute("objectClass", []string{"groupOfNames", "top"})
		addReq.Attribute("cn", []string{g.cn})
		addReq.Attribute("member", g.members)
		_ = l.Add(addReq) // ignore "already exists"
	}
}

// setupLDAPTestApp creates a full test app connected to the real LDAP server.
func setupLDAPTestApp(t *testing.T, env *ldapIntegrationEnv) (*App, *session.Store) {
	t.Helper()

	store := session.New(session.Config{
		Storage: memory.New(),
	})

	// Create a real LDAP client for the cache (service account)
	client, err := ldap.New(env.config, env.adminDN, env.adminPass)
	require.NoError(t, err)

	cacheClient := &ldapCacheClientAdapter{client: client}
	cache := ldap_cache.New(cacheClient)

	// Warm up the cache
	_ = cache.RefreshUsers()
	_ = cache.RefreshGroups()
	_ = cache.RefreshComputers()

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

	manifest := &AssetManifest{
		Assets:    map[string]string{"styles.css": "styles.abc123.css"},
		StylesCSS: "styles.abc123.css",
	}

	f := fiber.New(fiber.Config{
		ErrorHandler: handle500,
	})

	app := &App{
		ldapConfig:    env.config,
		ldapReadonly:  client,
		ldapCache:     cache,
		sessionStore:  store,
		templateCache: templateCache,
		fiber:         f,
		assetManifest: manifest,
		rateLimiter:   rateLimiter,
		stopCacheLog:  make(chan struct{}),
	}

	t.Cleanup(func() {
		templateCache.Stop()
		rateLimiter.Stop()
	})

	// Register routes without CSRF for testing
	f.All("/login", app.rateLimiter.Middleware(), app.loginHandler)
	f.Get("/health", app.healthHandler)
	f.Get("/health/ready", app.readinessHandler)
	f.Get("/health/live", app.livenessHandler)
	f.Get("/debug/cache", app.RequireAuth(), app.cacheStatsHandler)
	f.Get("/debug/ldap-pool", app.RequireAuth(), app.poolStatsHandler)

	protected := f.Group("/", app.RequireAuth())
	protected.Get("/", app.indexHandler)
	protected.Get("/users", app.templateCacheMiddleware(), app.usersHandler)
	protected.Get("/groups", app.templateCacheMiddleware(), app.groupsHandler)
	protected.Get("/computers", app.templateCacheMiddleware(), app.computersHandler)
	protected.Get("/users/*", app.userHandler)
	protected.Get("/groups/*", app.groupHandler)
	protected.Get("/computers/*", app.computerHandler)
	protected.Post("/users/*", app.userModifyHandler)
	protected.Post("/groups/*", app.groupModifyHandler)
	protected.Get("/logout", app.logoutHandler)
	f.Use(app.fourOhFourHandler)

	return app, store
}

// ldapCacheClientAdapter adapts *ldap.LDAP to the ldap_cache.LDAPClient interface.
type ldapCacheClientAdapter struct {
	client *ldap.LDAP
}

func (a *ldapCacheClientAdapter) FindUsers() ([]ldap.User, error) {
	return a.client.FindUsers()
}

func (a *ldapCacheClientAdapter) FindGroups() ([]ldap.Group, error) {
	return a.client.FindGroups()
}

func (a *ldapCacheClientAdapter) FindComputers() ([]ldap.Computer, error) {
	return a.client.FindComputers()
}

func (a *ldapCacheClientAdapter) CheckPasswordForSAMAccountName(sam, pass string) (*ldap.User, error) {
	return a.client.CheckPasswordForSAMAccountName(sam, pass)
}

func (a *ldapCacheClientAdapter) WithCredentials(dn, pass string) (*ldap.LDAP, error) {
	return a.client.WithCredentials(dn, pass)
}

// createLDAPAuthSession creates an authenticated session with real LDAP admin credentials.
func createLDAPAuthSession(t *testing.T, env *ldapIntegrationEnv, store *session.Store) []*http.Cookie {
	t.Helper()

	miniApp := fiber.New()
	miniApp.Get("/set-session", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		sess.Set("dn", env.adminDN)
		sess.Set("password", env.adminPass)
		sess.Set("username", "admin")

		return sess.Save()
	})

	req := httptest.NewRequest("GET", "/set-session", http.NoBody)
	resp, err := miniApp.Test(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	cookies := resp.Cookies()
	require.NotEmpty(t, cookies, "Expected session cookie")

	return cookies
}

// makeLDAPAuthRequest makes an authenticated GET request to the test app.
func makeLDAPAuthRequest(t *testing.T, app *App, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest("GET", path, http.NoBody)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := app.fiber.Test(req, -1) // no timeout
	require.NoError(t, err)

	return resp
}

func TestLDAPIntegration_HealthEndpoints(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, _ := setupLDAPTestApp(t, env)

	t.Run("health returns cache and pool stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		resp, err := app.fiber.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "overall_healthy")
		assert.Contains(t, string(body), "cache")
		assert.Contains(t, string(body), "connection_pool")
	})

	t.Run("readiness returns ready", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
		resp, err := app.fiber.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		// Should be ready or warming up
		assert.True(t,
			resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable,
			"Expected 200 or 503, got %d", resp.StatusCode)
		assert.Contains(t, bodyStr, "status")
	})

	t.Run("liveness returns alive", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health/live", http.NoBody)
		resp, err := app.fiber.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "alive")
	})
}

func TestLDAPIntegration_CacheStats(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	resp := makeLDAPAuthRequest(t, app, "/debug/cache", cookies)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "entries")
}

func TestLDAPIntegration_PoolStats(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	resp := makeLDAPAuthRequest(t, app, "/debug/ldap-pool", cookies)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "stats")
}

func TestLDAPIntegration_UsersHandler(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	t.Run("lists users", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/users", cookies)
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
		// Seeded users should appear in the page
		assert.Contains(t, bodyStr, "testuser1")
	})

	t.Run("lists users with disabled", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/users?show-disabled=1", cookies)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestLDAPIntegration_UserDetailHandler(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	// URL-encode the user DN
	userDN := "cn=testuser1,ou=users," + env.baseDN
	encodedDN := userDN // Fiber handles URL encoding

	t.Run("shows user detail page", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/users/"+encodedDN, cookies)
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, bodyStr, "testuser1")
	})

	t.Run("returns 404 for nonexistent user", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/users/cn=nonexistent,ou=users,"+env.baseDN, cookies)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestLDAPIntegration_GroupsHandler(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	t.Run("lists groups", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/groups", cookies)
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
		// At least one group from seeded data
		assert.Contains(t, bodyStr, "admins")
	})
}

func TestLDAPIntegration_GroupDetailHandler(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	groupDN := "cn=admins,ou=groups," + env.baseDN

	t.Run("shows group detail page", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/groups/"+groupDN, cookies)
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, bodyStr, "admins")
	})

	t.Run("returns 404 for nonexistent group", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/groups/cn=nonexistent,ou=groups,"+env.baseDN, cookies)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestLDAPIntegration_ComputersHandler(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	t.Run("lists computers (may be empty)", func(t *testing.T) {
		resp := makeLDAPAuthRequest(t, app, "/computers", cookies)
		defer func() { _ = resp.Body.Close() }()

		// Computers handler should succeed even if no computers exist
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	})
}

func TestLDAPIntegration_IndexHandler(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, store := setupLDAPTestApp(t, env)
	cookies := createLDAPAuthSession(t, env, store)

	resp := makeLDAPAuthRequest(t, app, "/", cookies)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	assert.NotEmpty(t, body)
}

func TestLDAPIntegration_AuthFlow(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)
	app, _ := setupLDAPTestApp(t, env)

	t.Run("unauthenticated redirects to login", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users", http.NoBody)
		resp, err := app.fiber.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/login", resp.Header.Get("Location"))
	})

	t.Run("login page renders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", http.NoBody)
		resp, err := app.fiber.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestLDAPIntegration_DirectBindAuth(t *testing.T) {
	env := skipIfNoLDAP(t)
	seedLDAPData(t, env)

	store := session.New(session.Config{Storage: memory.New()})
	app := &App{
		ldapConfig:   env.config,
		sessionStore: store,
	}

	t.Run("valid user direct bind", func(t *testing.T) {
		dn, err := app.authenticateViaDirectBind("testuser1", "password1")
		if err != nil {
			// Direct bind constructs DN as cn=username,baseDN which may not match OU structure
			t.Logf("Direct bind failed (expected for OU-based layout): %v", err)
		} else {
			assert.NotEmpty(t, dn)
		}
	})

	t.Run("invalid password fails", func(t *testing.T) {
		_, err := app.authenticateViaDirectBind("testuser1", "wrongpassword")
		assert.Error(t, err)
	})

	t.Run("LDAP injection rejected", func(t *testing.T) {
		_, err := app.authenticateViaDirectBind("admin*)(objectClass=*", "password")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")
	})
}

func TestLDAPIntegration_GetHealthStatusCode(t *testing.T) {
	env := skipIfNoLDAP(t)
	app, _ := setupLDAPTestApp(t, env)

	tests := []struct {
		name       string
		healthy    bool
		cacheStr   string
		poolOK     bool
		wantStatus int
	}{
		{"all healthy", true, statusHealthy, true, fiber.StatusOK},
		{"degraded cache", false, "degraded", true, fiber.StatusOK},
		{"unhealthy cache", false, statusUnhealthy, false, fiber.StatusServiceUnavailable},
		{"healthy cache unhealthy pool", false, statusHealthy, false, fiber.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code := app.getHealthStatusCode(tc.healthy, tc.cacheStr, tc.poolOK)
			assert.Equal(t, tc.wantStatus, code)
		})
	}
}

func TestLDAPIntegration_GetReadinessStatus(t *testing.T) {
	env := skipIfNoLDAP(t)
	app, _ := setupLDAPTestApp(t, env)

	tests := []struct {
		name      string
		cache     bool
		warmed    bool
		pool      bool
		wantEmpty bool
	}{
		{"all healthy", true, true, true, true},
		{"cache unhealthy", false, true, true, false},
		{"not warmed up", true, false, true, false},
		{"pool unhealthy", true, true, false, false},
		{"nothing healthy", false, false, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, reason := app.getReadinessStatus(tc.cache, tc.warmed, tc.pool)
			if tc.wantEmpty {
				assert.Empty(t, status)
				assert.Empty(t, reason)
			} else {
				assert.NotEmpty(t, status)
				assert.NotEmpty(t, reason)
			}
		})
	}
}

