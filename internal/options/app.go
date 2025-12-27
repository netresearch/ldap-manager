// Package options provides configuration parsing and environment variable handling
// for the LDAP Manager application.
package options

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Opts holds all configuration options for the LDAP Manager application.
// It includes LDAP connection settings, session management, connection pooling, and logging configuration.
type Opts struct {
	LogLevel zerolog.Level

	LDAP             ldap.Config
	ReadonlyUser     string
	ReadonlyPassword string

	PersistSessions bool
	SessionPath     string
	SessionDuration time.Duration

	// Cookie security settings
	CookieSecure bool

	// TLS settings
	TLSSkipVerify bool

	// LDAP Connection Pool settings
	PoolMaxConnections      int
	PoolMinConnections      int
	PoolMaxIdleTime         time.Duration
	PoolMaxLifetime         time.Duration
	PoolHealthCheckInterval time.Duration
	PoolConnectionTimeout   time.Duration
	PoolAcquireTimeout      time.Duration
}

func panicWhenEmpty(name string, value *string) {
	if *value == "" {
		log.Fatal().Msgf("the option --%s is required", name)
	}
}

func envStringOrDefault(name, d string) string {
	if v, exists := os.LookupEnv(name); exists && v != "" {
		return v
	}

	return d
}

func envDurationOrDefault(name string, d time.Duration) time.Duration {
	raw := envStringOrDefault(name, d.String())

	v, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatal().Msgf("could not parse environment variable \"%s\" (containing \"%s\") as duration: %v", name, raw, err)
	}

	return v
}

func envLogLevelOrDefault(name string, d zerolog.Level) string {
	raw := envStringOrDefault(name, d.String())

	if _, err := zerolog.ParseLevel(raw); err != nil {
		log.Fatal().Msgf("could not parse environment variable \"%s\" (containing \"%s\") as log level: %v", name, raw, err)
	}

	return raw
}

func envBoolOrDefault(name string, d bool) bool {
	raw := envStringOrDefault(name, strconv.FormatBool(d))

	v2, err := strconv.ParseBool(raw)
	if err != nil {
		log.Fatal().Msgf("could not parse environment variable \"%s\" (containing \"%s\") as bool: %v", name, raw, err)
	}

	return v2
}

func envIntOrDefault(name string, d int) int {
	raw := envStringOrDefault(name, strconv.Itoa(d))

	v, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatal().Msgf("could not parse environment variable \"%s\" (containing \"%s\") as int: %v", name, raw, err)
	}

	return v
}

// Parse parses command line flags and environment variables to build application configuration.
// It loads from .env files, parses flags, and validates required settings.
func Parse() *Opts {
	if err := godotenv.Load(".env.local", ".env"); err != nil {
		log.Warn().Err(err).Msg("could not load .env file")
	}

	var (
		fLogLevel = flag.String("log-level", envLogLevelOrDefault("LOG_LEVEL", zerolog.InfoLevel),
			"Log level. Valid values are: trace, debug, info, warn, error, fatal, panic.")

		fLdapServer = flag.String("ldap-server", envStringOrDefault("LDAP_SERVER", ""),
			"LDAP server URI, has to begin with `ldap://` or `ldaps://`. "+
				"If this is an ActiveDirectory server, this *has* to be `ldaps://`.")
		fIsActiveDirectory = flag.Bool("active-directory", envBoolOrDefault("LDAP_IS_AD", false),
			"Mark the LDAP server as ActiveDirectory.")
		fBaseDN       = flag.String("base-dn", envStringOrDefault("LDAP_BASE_DN", ""), "Base DN of your LDAP directory.")
		fReadonlyUser = flag.String("readonly-user", envStringOrDefault("LDAP_READONLY_USER", ""),
			"User that can read all users in your LDAP directory.")
		fReadonlyPassword = flag.String("readonly-password", envStringOrDefault("LDAP_READONLY_PASSWORD", ""),
			"Password for the readonly user.")

		fPersistSessions = flag.Bool("persist-sessions", envBoolOrDefault("PERSIST_SESSIONS", false),
			"Whether or not to persist sessions into a Bolt database. Useful for development.")
		fSessionPath = flag.String("session-path", envStringOrDefault("SESSION_PATH", "db.bbolt"),
			"Path to the session database file. (Only required when --persist-sessions is set)")
		fSessionDuration = flag.Duration("session-duration", envDurationOrDefault("SESSION_DURATION", 30*time.Minute),
			"Duration of the session. (Only required when --persist-sessions is set)")

		// Cookie security configuration
		fCookieSecure = flag.Bool("cookie-secure", envBoolOrDefault("COOKIE_SECURE", true),
			"Require HTTPS for session and CSRF cookies. "+
				"Set to false only for HTTP-only environments. Defaults to true for security.")

		// TLS configuration
		fTLSSkipVerify = flag.Bool("tls-skip-verify", envBoolOrDefault("LDAP_TLS_SKIP_VERIFY", false),
			"Skip TLS certificate verification. Use only for development with self-signed certificates.")

		// LDAP Connection Pool configuration
		fPoolMaxConnections = flag.Int("pool-max-connections", envIntOrDefault("LDAP_POOL_MAX_CONNECTIONS", 10),
			"Maximum number of connections in the LDAP connection pool.")
		fPoolMinConnections = flag.Int("pool-min-connections", envIntOrDefault("LDAP_POOL_MIN_CONNECTIONS", 2),
			"Minimum number of connections to maintain in the LDAP connection pool.")
		fPoolMaxIdleTime = flag.Duration("pool-max-idle-time",
			envDurationOrDefault("LDAP_POOL_MAX_IDLE_TIME", 15*time.Minute),
			"Maximum time a connection can be idle in the pool before being closed.")
		fPoolMaxLifetime = flag.Duration("pool-max-lifetime", envDurationOrDefault("LDAP_POOL_MAX_LIFETIME", 1*time.Hour),
			"Maximum lifetime of a connection in the pool.")
		fPoolHealthCheckInterval = flag.Duration("pool-health-check-interval",
			envDurationOrDefault("LDAP_POOL_HEALTH_CHECK_INTERVAL", 30*time.Second),
			"Interval for connection health checks in the pool.")
		fPoolConnectionTimeout = flag.Duration("pool-connection-timeout",
			envDurationOrDefault("LDAP_POOL_CONNECTION_TIMEOUT", 30*time.Second),
			"Timeout for establishing new LDAP server connections (TCP + TLS).")
		fPoolAcquireTimeout = flag.Duration("pool-acquire-timeout",
			envDurationOrDefault("LDAP_POOL_ACQUIRE_TIMEOUT", 10*time.Second),
			"Timeout for acquiring a connection from the pool.")
	)

	if !flag.Parsed() {
		flag.Parse()
	}

	logLevel, err := zerolog.ParseLevel(*fLogLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse log level")
	}

	panicWhenEmpty("ldap-server", fLdapServer)
	panicWhenEmpty("base-dn", fBaseDN)
	panicWhenEmpty("readonly-user", fReadonlyUser)
	panicWhenEmpty("readonly-password", fReadonlyPassword)

	if *fPersistSessions {
		panicWhenEmpty("session-path", fSessionPath)
	}

	ldapConfig := ldap.Config{
		Server:            *fLdapServer,
		BaseDN:            *fBaseDN,
		IsActiveDirectory: *fIsActiveDirectory,
	}

	return &Opts{
		LogLevel: logLevel,

		LDAP:             ldapConfig,
		ReadonlyUser:     *fReadonlyUser,
		ReadonlyPassword: *fReadonlyPassword,

		PersistSessions: *fPersistSessions,
		SessionPath:     *fSessionPath,
		SessionDuration: *fSessionDuration,

		CookieSecure:  *fCookieSecure,
		TLSSkipVerify: *fTLSSkipVerify,

		PoolMaxConnections:      *fPoolMaxConnections,
		PoolMinConnections:      *fPoolMinConnections,
		PoolMaxIdleTime:         *fPoolMaxIdleTime,
		PoolMaxLifetime:         *fPoolMaxLifetime,
		PoolHealthCheckInterval: *fPoolHealthCheckInterval,
		PoolConnectionTimeout:   *fPoolConnectionTimeout,
		PoolAcquireTimeout:      *fPoolAcquireTimeout,
	}
}
