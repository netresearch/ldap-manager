// Package options provides configuration parsing and environment variable handling
// for the LDAP Manager application.
package options

import (
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
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

	// AdminGroupDN gates the password-expiry roster. A viewer is treated as an
	// admin when they are a member of this group OR carry AD's adminCount=1.
	// Empty means only adminCount counts, which is AD-only — set this to grant
	// admin on OpenLDAP, where adminCount does not exist.
	AdminGroupDN string

	PersistSessions bool
	SessionPath     string
	SessionDuration time.Duration

	// PinnedPath is the filesystem path of the bbolt file backing the
	// per-user pinned-items store (spec §6.5). Empty string enables
	// automatic placement: "<SessionPath>.pinned" when PersistSessions
	// is true, otherwise "pinned.bbolt" in the process cwd. Set
	// explicitly (--pinned-path / PINNED_PATH) when the cwd is
	// read-only or a different mount point is needed; set to "none" /
	// "disabled" to skip opening the store entirely (pin UI hides,
	// pinning becomes a no-op).
	PinnedPath string

	// Cookie security settings
	CookieSecure bool

	// TrustedProxies lists the peers whose X-Forwarded-* headers the app
	// believes, as IP addresses or CIDR ranges. Two things depend on it:
	// the scheme the app reports for itself (X-Forwarded-Proto), which the
	// CSRF middleware compares against the browser's Origin, and the client
	// IP the login rate limiter counts against (X-Forwarded-For).
	//
	// Trusting a peer therefore lets it name its own client IP, so the list
	// stays as narrow as the deployment allows. The default covers loopback
	// and the Docker bridge network, which is where a sidecar terminator
	// sits; a proxy on another network (a Kubernetes pod range, an external
	// load balancer) has to be named explicitly.
	TrustedProxies []string

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

// DefaultTrustedProxies is the trust list used when none is configured:
// loopback plus the Docker bridge range, which is where a sidecar TLS
// terminator sits.
var DefaultTrustedProxies = []string{"127.0.0.0/8", "::1/128", "172.16.0.0/12"}

// errIPv4Mapped refuses an IPv4-mapped entry and names the plain form to
// write instead, because the mapped spelling either widens the list far past
// what was intended or matches nothing at all.
func errIPv4Mapped(entry, plain string) error {
	return ValidationError{
		Field: "trusted-proxies",
		Message: fmt.Sprintf(
			"%q is an IPv4-mapped IPv6 entry, which does not mean what it looks like; write %q instead",
			entry, plain),
	}
}

// unmapPrefix renders an IPv4-mapped prefix in its plain IPv4 form, so the
// error can name the entry the operator should have written. A prefix shorter
// than /96 cuts into the mapping and has no IPv4 equivalent; it is reported
// as the whole-IPv4 wildcard, which is what such an entry gestures at.
func unmapPrefix(prefix netip.Prefix) string {
	const mappedPrefixBits = 96

	if prefix.Bits() < mappedPrefixBits {
		return "0.0.0.0/0"
	}

	return netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-mappedPrefixBits).Masked().String()
}

// parseTrustedProxies splits a comma-separated list of IP addresses and CIDR
// ranges and rejects any entry that is neither. An empty list is refused
// rather than treated as "trust nothing": an empty value in a deployment is
// far more likely to be a mistyped variable than an intent, and the failure it
// causes — the app reading its own scheme as plain HTTP behind a terminator —
// surfaces as a CSRF error nowhere near the cause.
func parseTrustedProxies(raw string) ([]string, error) {
	entries := strings.Split(raw, ",")
	proxies := make([]string, 0, len(entries))

	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(trimmed)
		if err != nil {
			addr, addrErr := netip.ParseAddr(trimmed)
			if addrErr != nil {
				return nil, ValidationError{
					Field:   "trusted-proxies",
					Message: fmt.Sprintf("%q is neither an IP address nor a CIDR range", trimmed),
				}
			}

			if addr.Is4In6() {
				return nil, errIPv4Mapped(trimmed, addr.Unmap().String())
			}

			proxies = append(proxies, trimmed)

			continue
		}

		// An IPv4-mapped prefix is never what the operator means. Fiber parses
		// the list with net.ParseCIDR, which folds a mapped prefix of /96 or
		// longer into its IPv4 form — so "::ffff:0:0/96" silently becomes
		// 0.0.0.0/0 and trusts every IPv4 peer on the internet, which is the
		// opposite of a narrow list. Shorter than /96 the prefix cuts into the
		// mapping itself and matches no IPv4 peer at all. Both cases are
		// refused in favour of the plain IPv4 form.
		if prefix.Addr().Is4In6() {
			return nil, errIPv4Mapped(trimmed, unmapPrefix(prefix))
		}

		proxies = append(proxies, trimmed)
	}

	if len(proxies) == 0 {
		return nil, ValidationError{
			Field:   "trusted-proxies",
			Message: "the list is empty; name at least one IP address or CIDR range",
		}
	}

	return proxies, nil
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
		fAdminGroup = flag.String("admin-group", envStringOrDefault("LDAP_ADMIN_GROUP", ""),
			"DN of the group whose members may view the password-expiry roster. "+
				"Members of this group, or accounts with AD adminCount=1, are admins. "+
				"Required to grant access on OpenLDAP, which has no adminCount.")

		fPersistSessions = flag.Bool("persist-sessions", persistSessions,
			"Whether or not to persist sessions into a Bolt database. Useful for development.")
		fSessionPath = flag.String("session-path", envStringOrDefault("SESSION_PATH", "db.bbolt"),
			"Path to the session database file. (Only required when --persist-sessions is set)")
		fSessionDuration = flag.Duration("session-duration", sessionDuration,
			"Duration of the session. (Only required when --persist-sessions is set)")
		fPinnedPath = flag.String("pinned-path", envStringOrDefault("PINNED_PATH", ""),
			"Filesystem path of the bbolt file for the per-user pinned-items store. "+
				"Empty enables auto-placement (<session-path>.pinned when --persist-sessions, "+
				"otherwise 'pinned.bbolt' in cwd). Set to 'none' to disable pinning entirely "+
				"(useful on read-only filesystems).")

		// Cookie security configuration
		fCookieSecure = flag.Bool("cookie-secure", cookieSecure,
			"Require HTTPS for session and CSRF cookies. "+
				"Set to false only for HTTP-only environments. Defaults to true for security.")

		fTrustedProxies = flag.String("trusted-proxies",
			envStringOrDefault("TRUSTED_PROXIES", strings.Join(DefaultTrustedProxies, ",")),
			"Comma-separated IP addresses or CIDR ranges whose X-Forwarded-* headers are believed. "+
				"Name the TLS terminator in front of the app; a proxy outside this list leaves the app "+
				"reading its own scheme as plain HTTP, which fails CSRF origin checks. The list also "+
				"decides whose X-Forwarded-For the login rate limiter counts, so keep it narrow.")

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
	// readonly-user and readonly-password are optional.
	// When not configured, the app uses per-user LDAP credentials
	// and the background cache is disabled.

	if *fPersistSessions {
		if err := validateRequired("session-path", fSessionPath); err != nil {
			return nil, err
		}
	}

	trustedProxies, err := parseTrustedProxies(*fTrustedProxies)
	if err != nil {
		return nil, err
	}

	ldapConfig := ldap.Config{
		Server:            *fLdapServer,
		BaseDN:            *fBaseDN,
		IsActiveDirectory: *fIsActiveDirectory,

		// simple-ldap-go up to v1.17.0 set EnableOptimizations itself, so its
		// cache and its performance monitor were on whatever we passed. From the
		// next release the flags are honoured, and a client that asks for nothing
		// gets nothing (netresearch/simple-ldap-go#243).
		//
		// We ask for the monitor: /debug/pool-stats serves GetPoolStats(), which
		// returns an empty PerformanceStats without it — the endpoint would answer
		// zeros indistinguishable from an idle server.
		EnableMetrics: true,
		// We do not ask for the library's cache. internal/ldap_cache is our cache
		// of users, groups and computers; the library's would sit underneath it
		// caching the same lookups a second time, for another 1000 entries and up
		// to 64 MB. Set EnableCache here if that changes.
	}

	return &Opts{
		LogLevel: logLevel,

		LDAP:             ldapConfig,
		ReadonlyUser:     *fReadonlyUser,
		ReadonlyPassword: *fReadonlyPassword,
		AdminGroupDN:     *fAdminGroup,

		PersistSessions: *fPersistSessions,
		SessionPath:     *fSessionPath,
		SessionDuration: *fSessionDuration,
		PinnedPath:      *fPinnedPath,

		CookieSecure:   *fCookieSecure,
		TrustedProxies: trustedProxies,
		TLSSkipVerify:  *fTLSSkipVerify,

		PoolMaxConnections:      *fPoolMaxConnections,
		PoolMinConnections:      *fPoolMinConnections,
		PoolMaxIdleTime:         *fPoolMaxIdleTime,
		PoolMaxLifetime:         *fPoolMaxLifetime,
		PoolHealthCheckInterval: *fPoolHealthCheckInterval,
		PoolConnectionTimeout:   *fPoolConnectionTimeout,
		PoolAcquireTimeout:      *fPoolAcquireTimeout,
	}, nil
}
