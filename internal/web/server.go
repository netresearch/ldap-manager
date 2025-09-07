package web

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/bbolt/v2"
	"github.com/gofiber/storage/memory/v2"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"

	ldappool "github.com/netresearch/ldap-manager/internal/ldap"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web/static"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// App represents the main web application structure.
// It encapsulates LDAP client, connection pool, cache manager, session store, template cache, and Fiber web framework.
// Provides centralized management of authentication, caching, connection pooling, and HTTP request handling.
type App struct {
	ldapClient    *ldap.LDAP
	ldapPool      *ldappool.PoolManager
	ldapCache     *ldap_cache.Manager
	sessionStore  *session.Store
	templateCache *TemplateCache
	csrfHandler   fiber.Handler
	fiber         *fiber.App
}

func getSessionStorage(opts *options.Opts) fiber.Storage {
	if opts.PersistSessions {
		return bbolt.New(bbolt.Config{
			Database: opts.SessionPath,
			Bucket:   "sessions",
			Reset:    false,
		})
	}

	return memory.New()
}

// createPoolConfig creates LDAP connection pool configuration from options
func createPoolConfig(opts *options.Opts) *ldappool.PoolConfig {
	return &ldappool.PoolConfig{
		MaxConnections:      opts.PoolMaxConnections,
		MinConnections:      opts.PoolMinConnections,
		MaxIdleTime:         opts.PoolMaxIdleTime,
		MaxLifetime:         opts.PoolMaxLifetime,
		HealthCheckInterval: opts.PoolHealthCheckInterval,
		AcquireTimeout:      opts.PoolAcquireTimeout,
	}
}

// createSessionStore creates session store with configuration from options
func createSessionStore(opts *options.Opts) *session.Store {
	return session.New(session.Config{
		Storage:        getSessionStorage(opts),
		Expiration:     opts.SessionDuration,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",
		CookieSecure:   true, // Enable secure flag for HTTPS
	})
}

// createFiberApp creates and configures a new Fiber application
func createFiberApp() *fiber.App {
	f := fiber.New(fiber.Config{
		AppName:      "netresearch/ldap-manager",
		BodyLimit:    4 * 1024,
		ErrorHandler: handle500,
	})
	setupMiddleware(f)

	return f
}

// NewApp creates a new web application instance with the provided configuration options.
// It initializes the LDAP client, connection pool, session management, template cache,
// Fiber web server, and registers all routes.
// Returns a configured App instance ready to start serving requests via Listen().
func NewApp(opts *options.Opts) (*App, error) {
	ldapClient, err := ldap.New(opts.LDAP, opts.ReadonlyUser, opts.ReadonlyPassword)
	if err != nil {
		return nil, err
	}

	ldapPool, err := ldappool.NewPoolManager(ldapClient, createPoolConfig(opts))
	if err != nil {
		return nil, err
	}

	sessionStore := createSessionStore(opts)
	templateCache := NewTemplateCache(DefaultTemplateCacheConfig())
	f := createFiberApp()
	csrfHandler := *createCSRFConfig()

	a := &App{
		ldapClient:    ldapClient,
		ldapPool:      ldapPool,
		ldapCache:     ldap_cache.New(ldapClient),
		templateCache: templateCache,
		sessionStore:  sessionStore,
		csrfHandler:   csrfHandler,
		fiber:         f,
	}

	// Setup all routes
	a.setupRoutes()

	return a, nil
}

// setupMiddleware configures all middleware for the Fiber app
func setupMiddleware(f *fiber.App) {
	// Security Headers Middleware
	f.Use(helmet.New(helmet.Config{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000, // 1 year
		HSTSExcludeSubdomains: false,    // Include subdomains
		HSTSPreloadEnabled:    true,
		ContentSecurityPolicy: "default-src 'self'; style-src 'self' 'unsafe-inline'; " +
			"script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; " +
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self';",
	}))

	f.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	f.Use("/static", filesystem.New(filesystem.Config{
		Root:   http.FS(static.Static),
		MaxAge: 24 * 60 * 60,
	}))
}

// createCSRFConfig creates and returns CSRF middleware configuration
func createCSRFConfig() *fiber.Handler {
	csrfHandler := csrf.New(csrf.Config{
		KeyLookup:      "form:csrf_token",
		CookieName:     "csrf_",
		CookieSameSite: "Strict",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		Expiration:     3600, // 1 hour
		KeyGenerator:   csrf.ConfigDefault.KeyGenerator,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Warn().Err(err).Msg("CSRF validation failed")
			c.Status(fiber.StatusForbidden)
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

			return templates.FourOhThree("CSRF token validation failed").Render(c.UserContext(), c.Response().BodyWriter())
		},
	})

	return &csrfHandler
}

// setupRoutes configures all routes for the application
func (a *App) setupRoutes() {
	f := a.fiber

	// Public routes (no authentication required)
	f.All("/login", a.csrfHandler, a.loginHandler)

	// Health check endpoints (no authentication required, no CSRF needed)
	f.Get("/health", a.healthHandler)
	f.Get("/health/ready", a.readinessHandler)
	f.Get("/health/live", a.livenessHandler)

	// Template cache stats endpoint for monitoring (authenticated)
	f.Get("/debug/cache", a.RequireAuth(), a.cacheStatsHandler)

	// LDAP connection pool stats endpoint for monitoring (authenticated)
	f.Get("/debug/ldap-pool", a.RequireAuth(), a.poolStatsHandler)

	// Protected routes with template caching for GET requests
	protected := f.Group("/", a.RequireAuth(), a.csrfHandler)

	// Apply template caching middleware to read-only endpoints
	cacheable := protected.Group("/", a.templateCacheMiddleware())
	cacheable.Get("/", a.indexHandler)
	cacheable.Get("/users", a.usersHandler)
	cacheable.Get("/users/:userDN", a.userHandler)
	cacheable.Get("/groups", a.groupsHandler)
	cacheable.Get("/groups/:groupDN", a.groupHandler)
	cacheable.Get("/computers", a.computersHandler)
	cacheable.Get("/computers/:computerDN", a.computerHandler)

	// POST routes without caching (these invalidate cache)
	protected.Post("/users/:userDN", a.userModifyHandler)
	protected.Post("/groups/:groupDN", a.groupModifyHandler)

	protected.Get("/logout", a.logoutHandler)

	f.Use(a.fourOhFourHandler)

	// Log template cache stats periodically
	go a.periodicCacheLogging()
}

// Listen starts the web application server on the specified address.
// It launches the LDAP cache manager in a background goroutine and begins serving HTTP requests.
// This method blocks until the server is shutdown or encounters an error.
func (a *App) Listen(addr string) error {
	go a.ldapCache.Run()

	return a.fiber.Listen(addr)
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown() error {
	a.templateCache.Stop()

	if a.ldapPool != nil {
		if err := a.ldapPool.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close LDAP connection pool")
		}
	}

	return nil
}

// templateCacheMiddleware creates middleware for template caching
func (a *App) templateCacheMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only cache GET requests
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		// Generate cache key
		cacheKey := a.templateCache.generateCacheKey(c)

		// Try to serve from cache
		if cachedContent, found := a.templateCache.Get(cacheKey); found {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			c.Set("X-Cache", "HIT")

			return c.Send(cachedContent)
		}

		// Set cache miss header for debugging
		c.Set("X-Cache", "MISS")

		return c.Next()
	}
}

// invalidateTemplateCache invalidates cache entries after data modifications
func (a *App) invalidateTemplateCache(paths ...string) {
	for _, path := range paths {
		count := a.templateCache.InvalidateByPath(path)
		log.Debug().Str("path", path).Int("invalidated", count).Msg("Template cache invalidated")
	}
}

// cacheStatsHandler provides cache statistics for monitoring
func (a *App) cacheStatsHandler(c *fiber.Ctx) error {
	stats := a.templateCache.Stats()

	return c.JSON(stats)
}

// poolStatsHandler provides LDAP connection pool statistics for monitoring
func (a *App) poolStatsHandler(c *fiber.Ctx) error {
	stats := a.ldapPool.GetStats()
	healthStatus := a.ldapPool.GetHealthStatus()

	response := map[string]interface{}{
		"stats":  stats,
		"health": healthStatus,
	}

	return c.JSON(response)
}

// periodicCacheLogging logs cache statistics periodically for monitoring
func (a *App) periodicCacheLogging() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		a.templateCache.LogStats()
	}
}

func handle500(c *fiber.Ctx, err error) error {
	log.Error().Err(err).Send()

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.FiveHundred(err).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) indexHandler(c *fiber.Ctx) error {
	// Get authenticated user DN from middleware context
	userDN, err := RequireUserDN(c)
	if err != nil {
		return err
	}

	user, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return handle500(c, err)
	}

	// Use template caching
	return a.templateCache.RenderWithCache(c, templates.Index(user))
}

func (a *App) fourOhFourHandler(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) authenticateLDAPClient(ctx context.Context, userDN, password string) (*ldappool.PooledLDAPClient, error) {
	executor, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return nil, err
	}

	return a.ldapPool.WithCredentials(ctx, executor.DN(), password)
}

// GetCSRFToken extracts the CSRF token from the context
func (a *App) GetCSRFToken(c *fiber.Ctx) string {
	if token := c.Locals("token"); token != nil {
		if tokenStr, ok := token.(string); ok {
			return tokenStr
		}
	}

	return ""
}
