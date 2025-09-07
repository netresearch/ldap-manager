package web

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/bbolt/v2"
	"github.com/gofiber/storage/memory/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web/static"
	"github.com/netresearch/ldap-manager/internal/web/templates"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

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

	f.Get("/", a.indexHandler)
	f.Get("/users", a.usersHandler)
	f.Get("/users/:userDN", a.userHandler)
	f.Post("/users/:userDN", a.userModifyHandler)
	f.Get("/groups", a.groupsHandler)
	f.Get("/groups/:groupDN", a.groupHandler)
	f.Post("/groups/:groupDN", a.groupModifyHandler)
	f.Get("/computers", a.computersHandler)
	f.Get("/computers/:computerDN", a.computerHandler)
	f.All("/login", a.loginHandler)
	f.Get("/logout", a.logoutHandler)

	f.Use(a.fourOhFourHandler)

	return a, nil
}

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
	sess, err := a.sessionStore.Get(c)
	if err != nil {
		return handle500(c, err)
	}

	// TODO: put this into a middleware
	if sess.Fresh() {
		return c.Redirect("/login")
	}

	user, err := a.ldapCache.FindUserByDN(sess.Get("dn").(string))
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
