package web

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/bbolt"
	"github.com/gofiber/template/html/v2"
	"github.com/netresearch/ldap-selfservice-password-changer/internal/ldap_cache"
	"github.com/netresearch/ldap-selfservice-password-changer/internal/options"
	"github.com/netresearch/ldap-selfservice-password-changer/internal/web/static"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

type App struct {
	ldap         *ldap.LDAP
	ldapCache    *ldap_cache.Cache
	sessionStore *session.Store
	fiber        *fiber.App
}

func NewApp(opts *options.Opts) (*App, error) {
	ldap, err := ldap.New(opts.LdapServer, opts.BaseDN, opts.ReadonlyUser, opts.ReadonlyPassword, opts.IsActiveDirectory)
	if err != nil {
		return nil, err
	}

	views := html.NewFileSystem(http.FS(templates), ".html")
	views.AddFunc("inputOpts", tplInputOpts)
	views.AddFunc("navbarActive", tplNavbarActive)

	sessionStorage := bbolt.New(bbolt.Config{
		Database: opts.DBPath,
		Bucket:   "sessions",
		Reset:    false,
	})
	sessionStore := session.New(session.Config{
		Storage: sessionStorage,
	})

	f := fiber.New(fiber.Config{
		AppName:   "netresearch/ldap-manager",
		BodyLimit: 4 * 1024,
		Views:     views,
	})
	f.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	f.Use("/static", filesystem.New(filesystem.Config{
		Root:   http.FS(static.Static),
		MaxAge: 24 * 60 * 60,
	}))

	a := &App{
		ldap:         ldap,
		ldapCache:    ldap_cache.New(ldap),
		sessionStore: sessionStore,
		fiber:        f,
	}

	f.Get("/", a.indexHandler)
	f.Get("/users", a.usersHandler)
	f.Get("/users/:userDN", a.userHandler)
	f.Get("/groups", a.groupsHandler)
	f.Get("/groups/:groupDN", a.groupHandler)
	f.Get("/login", a.loginHandler)
	f.Get("/logout", a.logoutHandler)

	f.Use(a.fourOhFourHandler)

	return a, nil
}

func (a *App) Listen(addr string) error {
	go a.ldapCache.Run()

	return a.fiber.Listen(addr)
}

func handle500(c *fiber.Ctx, err error) error {
	log.Error().Err(err).Msg("could not get session")

	return c.Render("views/500", fiber.Map{
		"title":       "error",
		"headscripts": "",
		"error":       err.Error(),
	}, "layouts/base")
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

	user, err := a.ldapCache.FindUserBySAMAccountName(sess.Get("username").(string))
	if err != nil {
		return handle500(c, err)
	}

	return c.Render("views/index", fiber.Map{
		"session":     sess,
		"title":       "List",
		"activePage":  "/",
		"headscripts": "",
		"user":        user,
	}, "layouts/logged-in")
}

func (a *App) fourOhFourHandler(c *fiber.Ctx) error {
	return c.Render("views/404", fiber.Map{
		"title":       "404",
		"headscripts": "",
	}, "layouts/base")
}
