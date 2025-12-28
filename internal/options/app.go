// Package options provides configuration parsing and environment variable handling
// for the LDAP Manager application.
package options

import (
	"flag"
	"fmt"
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

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("configuration error for %s: %s", e.Field, e.Message)
}

// validateRequired checks if a required value is provided.
func validateRequired(name string, value *string) error {
	if *value == "" {
		return ValidationError{Field: name, Message: "this option is required"}
	}

	return nil
}

func envStringOrDefault(name, d string) string {
	if v, exists := os.LookupEnv(name); exists && v != "" {
		return v
	}

	return d
}

func envDurationOrDefault(name string, d time.Duration) (time.Duration, error) {
	raw := envStringOrDefault(name, d.String())

	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, ValidationError{
			Field:   name,
			Message: fmt.Sprintf("could not parse %q as duration: %v", raw, err),
		}
	}

	return v, nil
}

func envLogLevelOrDefault(name string, d zerolog.Level) (string, error) {
	raw := envStringOrDefault(name, d.String())

	if _, err := zerolog.ParseLevel(raw); err != nil {
		return "", ValidationError{
			Field:   name,
			Message: fmt.Sprintf("could not parse %q as log level: %v", raw, err),
		}
	}

	return raw, nil
}

func envBoolOrDefault(name string, d bool) (bool, error) {
	raw := envStringOrDefault(name, strconv.FormatBool(d))

	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, ValidationError{
			Field:   name,
			Message: fmt.Sprintf("could not parse %q as bool: %v", raw, err),
		}
	}

	return v, nil
}

func envIntOrDefault(name string, d int) (int, error) {
	raw := envStringOrDefault(name, strconv.Itoa(d))

	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, ValidationError{
			Field:   name,
			Message: fmt.Sprintf("could not parse %q as int: %v", raw, err),
		}
	}

	return v, nil
}

// Parse parses command line flags and environment variables to build application configuration.
// It loads from .env files, parses flags, and validates required settings.
// Returns an error if any configuration is invalid or missing required values.
func Parse() (*Opts, error) {
	if err := godotenv.Load(".env.local", ".env"); err != nil {
		log.Warn().Err(err).Msg("could not load .env file")
	}

	// Parse environment variables with error handling
	logLevelStr, err := envLogLevelOrDefault("LOG_LEVEL", zerolog.InfoLevel)
	if err != nil {
		return nil, err
	}

	isActiveDirectory, err := envBoolOrDefault("LDAP_IS_AD", false)
	if err != nil {
		return nil, err
	}

	persistSessions, err := envBoolOrDefault("PERSIST_SESSIONS", false)
	if err != nil {
		return nil, err
	}

	sessionDuration, err := envDurationOrDefault("SESSION_DURATION", 30*time.Minute)
	if err != nil {
		return nil, err
	}

	cookieSecure, err := envBoolOrDefault("COOKIE_SECURE", true)
	if err != nil {
		return nil, err
	}

	tlsSkipVerify, err := envBoolOrDefault("LDAP_TLS_SKIP_VERIFY", false)
	if err != nil {
		return nil, err
	}

	poolMaxConnections, err := envIntOrDefault("LDAP_POOL_MAX_CONNECTIONS", 10)
	if err != nil {
		return nil, err
	}

	poolMinConnections, err := envIntOrDefault("LDAP_POOL_MIN_CONNECTIONS", 2)
	if err != nil {
		return nil, err
	}

	poolMaxIdleTime, err := envDurationOrDefault("LDAP_POOL_MAX_IDLE_TIME", 15*time.Minute)
	if err != nil {
		return nil, err
	}

	poolMaxLifetime, err := envDurationOrDefault("LDAP_POOL_MAX_LIFETIME", 1*time.Hour)
	if err != nil {
		return nil, err
	}

	poolHealthCheckInterval, err := envDurationOrDefault("LDAP_POOL_HEALTH_CHECK_INTERVAL", 30*time.Second)
	if err != nil {
		return nil, err
	}

	poolConnectionTimeout, err := envDurationOrDefault("LDAP_POOL_CONNECTION_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}

	poolAcquireTimeout, err := envDurationOrDefault("LDAP_POOL_ACQUIRE_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}

	var (
		fLogLevel = flag.String("log-level", logLevelStr,
			"Log level. Valid values are: trace, debug, info, warn, error, fatal, panic.")

		fLdapServer = flag.String("ldap-server", envStringOrDefault("LDAP_SERVER", ""),
			"LDAP server URI, has to begin with `ldap://` or `ldaps://`. "+
				"If this is an ActiveDirectory server, this *has* to be `ldaps://`.")
		fIsActiveDirectory = flag.Bool("active-directory", isActiveDirectory,
			"Mark the LDAP server as ActiveDirectory.")
		fBaseDN       = flag.String("base-dn", envStringOrDefault("LDAP_BASE_DN", ""), "Base DN of your LDAP directory.")
		fReadonlyUser = flag.String("readonly-user", envStringOrDefault("LDAP_READONLY_USER", ""),
			"User that can read all users in your LDAP directory.")
		fReadonlyPassword = flag.String("readonly-password", envStringOrDefault("LDAP_READONLY_PASSWORD", ""),
			"Password for the readonly user.")

		fPersistSessions = flag.Bool("persist-sessions", persistSessions,
			"Whether or not to persist sessions into a Bolt database. Useful for development.")
		fSessionPath = flag.String("session-path", envStringOrDefault("SESSION_PATH", "db.bbolt"),
			"Path to the session database file. (Only required when --persist-sessions is set)")
		fSessionDuration = flag.Duration("session-duration", sessionDuration,
			"Duration of the session. (Only required when --persist-sessions is set)")

		// Cookie security configuration
		fCookieSecure = flag.Bool("cookie-secure", cookieSecure,
			"Require HTTPS for session and CSRF cookies. "+
				"Set to false only for HTTP-only environments. Defaults to true for security.")

		// TLS configuration
		fTLSSkipVerify = flag.Bool("tls-skip-verify", tlsSkipVerify,
			"Skip TLS certificate verification. Use only for development with self-signed certificates.")

		// LDAP Connection Pool configuration
		fPoolMaxConnections = flag.Int("pool-max-connections", poolMaxConnections,
			"Maximum number of connections in the LDAP connection pool.")
		fPoolMinConnections = flag.Int("pool-min-connections", poolMinConnections,
			"Minimum number of connections to maintain in the LDAP connection pool.")
		fPoolMaxIdleTime = flag.Duration("pool-max-idle-time", poolMaxIdleTime,
			"Maximum time a connection can be idle in the pool before being closed.")
		fPoolMaxLifetime = flag.Duration("pool-max-lifetime", poolMaxLifetime,
			"Maximum lifetime of a connection in the pool.")
		fPoolHealthCheckInterval = flag.Duration("pool-health-check-interval", poolHealthCheckInterval,
			"Interval for connection health checks in the pool.")
		fPoolConnectionTimeout = flag.Duration("pool-connection-timeout", poolConnectionTimeout,
			"Timeout for establishing new LDAP server connections (TCP + TLS).")
		fPoolAcquireTimeout = flag.Duration("pool-acquire-timeout", poolAcquireTimeout,
			"Timeout for acquiring a connection from the pool.")
	)

	if !flag.Parsed() {
		flag.Parse()
	}

	logLevel, err := zerolog.ParseLevel(*fLogLevel)
	if err != nil {
		return nil, ValidationError{Field: "log-level", Message: err.Error()}
	}

	// Validate required fields
	if err := validateRequired("ldap-server", fLdapServer); err != nil {
		return nil, err
	}
	if err := validateRequired("base-dn", fBaseDN); err != nil {
		return nil, err
	}
	if err := validateRequired("readonly-user", fReadonlyUser); err != nil {
		return nil, err
	}
	if err := validateRequired("readonly-password", fReadonlyPassword); err != nil {
		return nil, err
	}

	if *fPersistSessions {
		if err := validateRequired("session-path", fSessionPath); err != nil {
			return nil, err
		}
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
	}, nil
}
