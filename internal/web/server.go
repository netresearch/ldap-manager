package web

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/bbolt"
	"github.com/gofiber/template/html/v2"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web/static"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

type App struct {
	ldap         *ldap.LDAP
	ldapConfig   ldap.Config
	ldapCache    *ldap_cache.Manager
	sessionStore *session.Store
	fiber        *fiber.App
}

func NewApp(opts *options.Opts) (*App, error) {
	ldap, err := ldap.New(opts.LDAP, opts.ReadonlyUser, opts.ReadonlyPassword)
	if err != nil {
		return nil, err
	}

	views := html.NewFileSystem(http.FS(templates), ".html")
	views.AddFunc("inputOpts", tplInputOpts)
	views.AddFunc("navbar", tplNavbar)
	views.AddFunc("navbarActive", tplNavbarActive)
	views.AddFunc("disabledUsersHref", tplDisabledUsersHref)
	views.AddFunc("disabledUsersTooltip", tplDisabledUsersTooltip)
	views.AddFunc("disabledUsersClass", tplDisabledUsersClass)

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
		ldapConfig:   opts.LDAP,
		ldapCache:    ldap_cache.New(ldap),
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
		"flashes":     []Flash{},
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

	user, err := a.ldapCache.FindUserByDN(sess.Get("dn").(string))
	if err != nil {
		return handle500(c, err)
	}

	return c.Render("views/index", fiber.Map{
		"session":     sess,
		"title":       "List",
		"activePage":  "/",
		"headscripts": "",
		"flashes":     []Flash{},
		"user":        user,
	}, "layouts/logged-in")
}

func (a *App) fourOhFourHandler(c *fiber.Ctx) error {
	return c.Render("views/404", fiber.Map{
		"title":       "404",
		"headscripts": "",
		"flashes":     []Flash{},
	}, "layouts/base")
}

func (a *App) sessionToLDAPClient(sess *session.Session) (*ldap.LDAP, error) {
	executor, err := a.ldapCache.FindUserByDN(sess.Get("dn").(string))
	if err != nil {
		return nil, err
	}

	return a.ldap.WithCredentials(executor.DN(), sess.Get("password").(string))
}
