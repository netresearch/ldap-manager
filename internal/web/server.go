package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
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

	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web/static"
	"github.com/netresearch/ldap-manager/internal/web/templates"
)

// App represents the main web application structure.
// Encapsulates LDAP config, readonly client, cache, session store, template cache, Fiber framework.
// Provides centralized auth, caching, and HTTP request handling.
// When ReadonlyUser is not configured, ldapReadonly and ldapCache are nil;
// all interactive LDAP operations use the logged-in user's own credentials.
type App struct {
	ldapConfig    ldap.Config
	ldapOpts      []ldap.Option       // LDAP client options (TLS, logging)
	ldapReadonly  *ldap.LDAP          // Service account client (nil when not configured)
	ldapCache     *ldap_cache.Manager // Background cache (nil when no service account)
	sessionStore  *session.Store
	templateCache *TemplateCache
	csrfHandler   fiber.Handler
	fiber         *fiber.App
	assetManifest *AssetManifest // Asset manifest for cache-busted files
	rateLimiter   *RateLimiter   // Rate limiter for authentication endpoints
	stopCacheLog  chan struct{}  // Stops periodicCacheLogging goroutine
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

// createSessionStore creates session store with configuration from options
func createSessionStore(opts *options.Opts) *session.Store {
	return session.New(session.Config{
		Storage:        getSessionStorage(opts),
		Expiration:     opts.SessionDuration,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",          // Strict for maximum security with proxy trust enabled
		CookieSecure:   opts.CookieSecure, // Configurable based on HTTPS availability
	})
}

// createFiberApp creates and configures a new Fiber application
func createFiberApp() *fiber.App {
	f := fiber.New(fiber.Config{
		AppName:      "netresearch/ldap-manager",
		BodyLimit:    4 * 1024,
		ErrorHandler: handle500,
		// Trust proxy headers from Traefik (Docker bridge network)
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"127.0.0.0/8", "::1/128", "172.16.0.0/12"}, // Loopback and Docker internal networks
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
	setupMiddleware(f)

	return f
}

// NewApp creates a new web application instance with the provided configuration options.
// It initializes the LDAP configuration, readonly client (if configured), session management,
// template cache, Fiber web server, and registers all routes.
// Returns a configured App instance ready to start serving requests via Listen().
//
// When ReadonlyUser is not configured, the app operates without a service account:
// all LDAP operations use the logged-in user's own credentials, and
// the background cache is disabled.
func NewApp(opts *options.Opts) (*App, error) {
	logger := slog.Default()

	// Build LDAP client options
	ldapOpts := []ldap.Option{ldap.WithLogger(logger)}

	// Add TLS skip verify option if configured (for development with self-signed certs)
	if opts.TLSSkipVerify {
		logger.Warn("TLS certificate verification is disabled - use only for development!")
		ldapOpts = append(ldapOpts, ldap.WithTLS(&tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // Intentional for development
		}))
	}

	// Create readonly LDAP client only when service account is configured
	var ldapReadonly *ldap.LDAP
	var ldapCache *ldap_cache.Manager

	if opts.ReadonlyUser != "" && opts.ReadonlyPassword != "" {
		var err error
		ldapReadonly, err = ldap.New(
			opts.LDAP,
			opts.ReadonlyUser,
			opts.ReadonlyPassword,
			ldapOpts...,
		)
		if err != nil {
			return nil, err
		}

		ldapCache = ldap_cache.New(ldapReadonly)
		log.Info().Msg("Service account configured, background cache enabled")
	} else {
		log.Info().Msg("No service account configured, using per-user LDAP credentials")
	}

	sessionStore := createSessionStore(opts)
	templateCache := NewTemplateCache(DefaultTemplateCacheConfig())
	f := createFiberApp()
	csrfHandler := createCSRFConfig(opts, sessionStore)

	// Load asset manifest for cache-busted files
	manifestPath := "internal/web/static/manifest.json"
	manifest, err := LoadAssetManifest(manifestPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load asset manifest, using defaults")
		manifest = &AssetManifest{
			Assets:    map[string]string{"styles.css": "styles.css"},
			StylesCSS: "styles.css",
		}
	}

	a := &App{
		ldapConfig:    opts.LDAP,
		ldapOpts:      ldapOpts,
		ldapReadonly:  ldapReadonly,
		ldapCache:     ldapCache,
		templateCache: templateCache,
		sessionStore:  sessionStore,
		csrfHandler:   csrfHandler,
		fiber:         f,
		assetManifest: manifest,
		rateLimiter:   NewRateLimiter(DefaultRateLimiterConfig()),
		stopCacheLog:  make(chan struct{}),
	}

	// Setup all routes
	a.setupRoutes()

	return a, nil
}

// setupMiddleware configures all middleware for the Fiber app
func setupMiddleware(f *fiber.App) {
	// Security Headers Middleware with full protection
	// Proxy trust configuration ensures cookies work correctly with HTTPS termination
	f.Use(helmet.New(helmet.Config{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000, // 1 year
		HSTSExcludeSubdomains: false,
		HSTSPreloadEnabled:    true,
		ContentSecurityPolicy: "default-src 'self'; style-src 'self'; " +
			"script-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; " +
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self';",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		ReferrerPolicy:            "strict-origin-when-cross-origin", // Required for CSRF referer validation
		// Note: CrossOriginEmbedderPolicy defaults to "require-corp" which breaks browser extensions
		// We remove it in the middleware below since Fiber's helmet doesn't support disabling it
	}))

	// Remove Cross-Origin-Embedder-Policy header - "require-corp" breaks browser extensions (Bitwarden)
	// Fiber's helmet middleware doesn't support disabling COEP (empty string gets overwritten with default)
	f.Use(func(c *fiber.Ctx) error {
		err := c.Next()
		c.Response().Header.Del("Cross-Origin-Embedder-Policy")

		return err
	})

	f.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	f.Use("/static", filesystem.New(filesystem.Config{
		Root:   http.FS(static.Static),
		MaxAge: 24 * 60 * 60,
	}))
}

// createCSRFConfig creates and returns CSRF middleware configuration
// Uses session-based storage to ensure CSRF tokens persist across requests
// and survive container restarts when PersistSessions is enabled.
func createCSRFConfig(opts *options.Opts, sessionStore *session.Store) fiber.Handler {
	return csrf.New(csrf.Config{
		KeyLookup:      "form:csrf_token",
		CookieName:     "csrf_",
		CookieSameSite: "Strict",          // Strict for maximum security with proxy trust enabled
		CookieSecure:   opts.CookieSecure, // Configurable based on HTTPS availability
		CookieHTTPOnly: true,
		Expiration:     time.Hour,
		KeyGenerator:   csrf.ConfigDefault.KeyGenerator,
		Session:        sessionStore, // Use session-based CSRF storage for persistence
		SessionKey:     "csrf_token", // Key to store CSRF token in session
		ContextKey:     "token",      // Store token in c.Locals("token") for template access
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Warn().Err(err).Msg("CSRF validation failed")
			c.Status(fiber.StatusForbidden)
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

			return templates.FourOhThree("CSRF token validation failed").Render(c.UserContext(), c.Response().BodyWriter())
		},
	})
}

// setupRoutes configures all routes for the application
func (a *App) setupRoutes() {
	f := a.fiber

	// Public routes (no authentication required)
	// Rate limiting applied to login to prevent brute force attacks
	f.All("/login", a.rateLimiter.Middleware(), a.csrfHandler, a.loginHandler)

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

	// Apply template caching middleware to read-only list endpoints (no CSRF tokens)
	cacheable := protected.Group("/", a.templateCacheMiddleware())
	cacheable.Get("/", a.indexHandler)
	cacheable.Get("/users", a.usersHandler)
	cacheable.Get("/groups", a.groupsHandler)
	cacheable.Get("/computers", a.computersHandler)

	// Detail pages contain CSRF tokens in forms — must NOT be cached
	// to avoid serving stale tokens that cause 403 on form submission
	protected.Get("/users/*", a.userHandler)
	protected.Get("/groups/*", a.groupHandler)
	protected.Get("/computers/*", a.computerHandler)

	// POST routes without caching (these invalidate cache)
	protected.Post("/users/*", a.userModifyHandler)
	protected.Post("/groups/*", a.groupModifyHandler)

	protected.Get("/logout", a.logoutHandler)

	f.Use(a.fourOhFourHandler)

	// Log template cache stats periodically
	go a.periodicCacheLogging()
}

// Listen starts the web application server on the specified address.
// It launches the LDAP cache manager in a background goroutine and begins serving HTTP requests.
// The context is used for graceful shutdown signaling to background goroutines.
// This method blocks until the server is shutdown or encounters an error.
func (a *App) Listen(ctx context.Context, addr string) error {
	if a.ldapCache != nil {
		go a.ldapCache.Run(ctx)
	}

	return a.fiber.Listen(addr)
}

// Shutdown gracefully shuts down the application within the given context timeout.
// Order: stop accepting requests → drain in-flight → stop background goroutines → close connections.
func (a *App) Shutdown(ctx context.Context) error {
	log.Info().Msg("Stopping periodic cache logging...")
	close(a.stopCacheLog)

	// Drain in-flight HTTP requests first, before stopping caches they may be reading
	log.Info().Msg("Shutting down Fiber server...")
	shutdownErr := a.fiber.ShutdownWithContext(ctx)
	if shutdownErr != nil {
		log.Error().Err(shutdownErr).Msg("Error shutting down Fiber server")
	}

	// Now safe to stop background goroutines — no handlers are reading from caches
	log.Info().Msg("Stopping template cache...")
	a.templateCache.Stop()

	if a.ldapCache != nil {
		log.Info().Msg("Stopping LDAP cache manager...")
		a.ldapCache.Stop()
	}

	log.Info().Msg("Stopping rate limiter...")
	a.rateLimiter.Stop()

	// Close LDAP connections last
	log.Info().Msg("Closing LDAP connections...")
	if a.ldapReadonly != nil {
		if err := a.ldapReadonly.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close LDAP readonly client")
		}
	}

	return shutdownErr
}

// getUserLDAP creates a user-bound LDAP client from session credentials.
// The caller must close the returned client via defer client.Close().
// Returns a fiber.StatusUnauthorized error if session has no credentials,
// which handle500 will convert to a login redirect.
func (a *App) getUserLDAP(c *fiber.Ctx) (*ldap.LDAP, error) {
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return nil, fmt.Errorf("getUserLDAP: session error: %w", err)
	}

	dn, _ := sess.Get("dn").(string)
	password, _ := sess.Get("password").(string)

	if dn == "" || password == "" {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "session expired or missing credentials")
	}

	client, err := ldap.New(a.ldapConfig, dn, password, a.ldapOpts...)
	if err != nil {
		// LDAP bind failure likely means expired/changed password → redirect to login
		return nil, fiber.NewError(fiber.StatusUnauthorized, "LDAP connection failed, please re-login")
	}

	return client, nil
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

// cacheStatsHandler provides cache statistics for monitoring
func (a *App) cacheStatsHandler(c *fiber.Ctx) error {
	stats := a.templateCache.Stats()

	return c.JSON(stats)
}

// poolStatsHandler provides LDAP performance statistics for monitoring
func (a *App) poolStatsHandler(c *fiber.Ctx) error {
	if a.ldapReadonly == nil {
		return c.JSON(map[string]any{
			"message": "No service account configured - per-user LDAP credentials in use",
		})
	}

	stats := a.ldapReadonly.GetPoolStats()

	return c.JSON(map[string]any{
		"stats": stats,
	})
}

// periodicCacheLogging logs cache statistics periodically for monitoring
func (a *App) periodicCacheLogging() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.templateCache.LogStats()
		case <-a.stopCacheLog:
			return
		}
	}
}

func handle500(c *fiber.Ctx, err error) error {
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		switch fiberErr.Code {
		case fiber.StatusUnauthorized:
			log.Warn().Err(err).Msg("session expired or invalid, redirecting to login")

			return c.Redirect("/login")
		default:
			// Use the fiber error's status code instead of always 500
			c.Status(fiberErr.Code)
		}
	} else {
		c.Status(fiber.StatusInternalServerError)
	}

	log.Error().Err(err).Send()

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	// Use a generic error to avoid leaking internal details to the user
	displayErr := errors.New("an unexpected error occurred")
	renderErr := templates.FiveHundred(displayErr).
		Render(c.UserContext(), c.Response().BodyWriter())
	if renderErr != nil {
		// Fallback: plain text to avoid infinite recursion if template render fails
		return c.SendString("Internal Server Error")
	}

	return nil
}

func (a *App) indexHandler(c *fiber.Ctx) error {
	// Get authenticated user DN from middleware context
	userDN, err := RequireUserDN(c)
	if err != nil {
		return err
	}

	userLDAP, err := a.getUserLDAP(c)
	if err != nil {
		return handle500(c, err)
	}
	defer func() { _ = userLDAP.Close() }()

	// Get username from middleware context (stored during auth)
	username, _ := c.Locals("username").(string)

	var user *ldap.User

	if username != "" {
		user, err = userLDAP.FindUserBySAMAccountName(username)
		// Fail fast on real errors (not just "not found")
		if err != nil && !errors.Is(err, ldap.ErrUserNotFound) {
			return handle500(c, err)
		}
	}

	// Fall back to finding by DN if lookup by SAMAccountName was not attempted or user not found
	if user == nil {
		allUsers, findErr := userLDAP.FindUsers()
		if findErr != nil {
			return handle500(c, findErr)
		}

		user, err = findUserByDN(allUsers, userDN)
		if err != nil {
			return handle500(c, err)
		}
	}

	groups, err := userLDAP.FindGroups()
	if err != nil {
		return handle500(c, err)
	}

	fullUser := ldap_cache.PopulateGroupsForUserFromData(user, groups)

	// Use template caching
	return a.templateCache.RenderWithCache(c, templates.Index(fullUser))
}

func (a *App) fourOhFourHandler(c *fiber.Ctx) error {
	c.Status(fiber.StatusNotFound)
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
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

// GetStylesPath returns the cache-busted CSS file path from the asset manifest
func (a *App) GetStylesPath() string {
	if a.assetManifest != nil {
		return a.assetManifest.GetStylesPath()
	}

	return "styles.css"
}
