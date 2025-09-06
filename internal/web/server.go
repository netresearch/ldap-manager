package web

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
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
// It encapsulates LDAP client, cache manager, session store, and Fiber web framework.
// Provides centralized management of authentication, caching, and HTTP request handling.
type App struct {
	ldapClient   *ldap.LDAP
	ldapCache    *ldap_cache.Manager
	sessionStore *session.Store
	fiber        *fiber.App
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

// NewApp creates a new web application instance with the provided configuration options.
// It initializes the LDAP client, session management, Fiber web server, and registers all routes.
// Returns a configured App instance ready to start serving requests via Listen().
func NewApp(opts *options.Opts) (*App, error) {
	ldapClient, err := ldap.New(opts.LDAP, opts.ReadonlyUser, opts.ReadonlyPassword)
	if err != nil {
		return nil, err
	}

	sessionStore := session.New(session.Config{
		Storage:        getSessionStorage(opts),
		Expiration:     opts.SessionDuration,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",
	})

	f := fiber.New(fiber.Config{
		AppName:      "netresearch/ldap-manager",
		BodyLimit:    4 * 1024,
		ErrorHandler: handle500,
	})
	f.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	f.Use("/static", filesystem.New(filesystem.Config{
		Root:   http.FS(static.Static),
		MaxAge: 24 * 60 * 60,
	}))

	a := &App{
		ldapClient:   ldapClient,
		ldapCache:    ldap_cache.New(ldapClient),
		sessionStore: sessionStore,
		fiber:        f,
	}

	// Public routes (no authentication required)
	f.All("/login", a.loginHandler)

	// Health check endpoints (no authentication required)
	f.Get("/health", a.healthHandler)
	f.Get("/health/ready", a.readinessHandler)
	f.Get("/health/live", a.livenessHandler)

	// Protected routes (require authentication)
	protected := f.Group("/", a.RequireAuth())
	protected.Get("/", a.indexHandler)
	protected.Get("/users", a.usersHandler)
	protected.Get("/users/:userDN", a.userHandler)
	protected.Post("/users/:userDN", a.userModifyHandler)
	protected.Get("/groups", a.groupsHandler)
	protected.Get("/groups/:groupDN", a.groupHandler)
	protected.Post("/groups/:groupDN", a.groupModifyHandler)
	protected.Get("/computers", a.computersHandler)
	protected.Get("/computers/:computerDN", a.computerHandler)
	protected.Get("/logout", a.logoutHandler)

	f.Use(a.fourOhFourHandler)

	return a, nil
}

// Listen starts the web application server on the specified address.
// It launches the LDAP cache manager in a background goroutine and begins serving HTTP requests.
// This method blocks until the server is shutdown or encounters an error.
func (a *App) Listen(addr string) error {
	go a.ldapCache.Run()

	return a.fiber.Listen(addr)
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

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.Index(user).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) fourOhFourHandler(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)

	return templates.FourOhFour(c.Path()).Render(c.UserContext(), c.Response().BodyWriter())
}

func (a *App) authenticateLDAPClient(userDN, password string) (*ldap.LDAP, error) {
	executor, err := a.ldapCache.FindUserByDN(userDN)
	if err != nil {
		return nil, err
	}

	return a.ldapClient.WithCredentials(executor.DN(), password)
}
