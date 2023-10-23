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

type Opts struct {
	LogLevel zerolog.Level

	LDAP             ldap.Config
	ReadonlyUser     string
	ReadonlyPassword string

	PersistSessions bool
	SessionPath     string
	SessionDuration time.Duration
}

func panicWhenEmpty(name string, value *string) {
	if *value == "" {
		log.Fatal().Msgf("err: The option --%s is required", name)
	}
}

func envStringOrDefault(name, d string) string {
	if v, exists := os.LookupEnv(name); exists && v != "" {
		return v
	}

	return d
}

func envDurationOrDefault(name string, d time.Duration) time.Duration {
	raw := envStringOrDefault(name, fmt.Sprintf("%v", d))

	v, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatal().Msgf("err: could not parse environment variable \"%s\" (containing \"%s\") as duration: %v", name, raw, err)
	}

	return v
}

func envLogLevelOrDefault(name string, d zerolog.Level) string {
	raw := envStringOrDefault(name, d.String())

	if _, err := zerolog.ParseLevel(raw); err != nil {
		log.Fatal().Msgf("err: could not parse environment variable \"%s\" (containing \"%s\") as log level: %v", name, raw, err)
	}

	return raw
}

func envBoolOrDefault(name string, d bool) bool {
	raw := envStringOrDefault(name, fmt.Sprintf("%v", d))

	v2, err := strconv.ParseBool(raw)
	if err != nil {
		log.Fatal().Msgf("err: could not parse environment variable \"%s\" (containing \"%s\") as bool: %v", name, raw, err)
	}

	return v2
}

func Parse() *Opts {
	if err := godotenv.Load(".env.local", ".env"); err != nil {
		log.Warn().Err(err).Msg("could not load .env file")
	}

	var (
		fLogLevel = flag.String("log-level", envLogLevelOrDefault("LOG_LEVEL", zerolog.InfoLevel), "Log level. Valid values are: trace, debug, info, warn, error, fatal, panic.")

		fLdapServer        = flag.String("ldap-server", envStringOrDefault("LDAP_SERVER", ""), "LDAP server URI, has to begin with `ldap://` or `ldaps://`. If this is an ActiveDirectory server, this *has* to be `ldaps://`.")
		fIsActiveDirectory = flag.Bool("active-directory", envBoolOrDefault("LDAP_IS_AD", false), "Mark the LDAP server as ActiveDirectory.")
		fBaseDN            = flag.String("base-dn", envStringOrDefault("LDAP_BASE_DN", ""), "Base DN of your LDAP directory.")
		fReadonlyUser      = flag.String("readonly-user", envStringOrDefault("LDAP_READONLY_USER", ""), "User that can read all users in your LDAP directory.")
		fReadonlyPassword  = flag.String("readonly-password", envStringOrDefault("LDAP_READONLY_PASSWORD", ""), "Password for the readonly user.")

		fPersistSessions = flag.Bool("persist-sessions", envBoolOrDefault("PERSIST_SESSIONS", false), "Whether or not to persist sessions into a Bolt database. Useful for development.")
		fSessionPath     = flag.String("session-path", envStringOrDefault("SESSION_PATH", "db.bbolt"), "Path to the session database file. (Only required when --persist-sessions is set)")
		fSessionDuration = flag.Duration("session-duration", envDurationOrDefault("SESSION_DURATION", 30*time.Minute), "Duration of the session. (Only required when --persist-sessions is set)")
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
	}
}
