package options

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/rs/zerolog/log"
)

type Opts struct {
	LDAP             ldap.Config
	ReadonlyUser     string
	ReadonlyPassword string

	DBPath string
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

func envIntOrDefault(name string, d uint64) uint {
	raw := envStringOrDefault(name, fmt.Sprintf("%v", d))

	v, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		log.Fatal().Msgf("err: could not parse environment variable \"%s\" (containing \"%s\") as uint: %v", name, raw, err)
	}

	return uint(v)
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
		fLdapServer        = flag.String("ldap-server", envStringOrDefault("LDAP_SERVER", ""), "LDAP server URI, has to begin with `ldap://` or `ldaps://`. If this is an ActiveDirectory server, this *has* to be `ldaps://`.")
		fIsActiveDirectory = flag.Bool("active-directory", envBoolOrDefault("LDAP_IS_AD", false), "Mark the LDAP server as ActiveDirectory.")
		fBaseDN            = flag.String("base-dn", envStringOrDefault("LDAP_BASE_DN", ""), "Base DN of your LDAP directory.")
		fReadonlyUser      = flag.String("readonly-user", envStringOrDefault("LDAP_READONLY_USER", ""), "User that can read all users in your LDAP directory.")
		fReadonlyPassword  = flag.String("readonly-password", envStringOrDefault("LDAP_READONLY_PASSWORD", ""), "Password for the readonly user.")

		fDBPath = flag.String("db-path", envStringOrDefault("DB_PATH", "db.bbolt"), "Path to the SQLite database file.")
	)

	if !flag.Parsed() {
		flag.Parse()
	}

	panicWhenEmpty("ldap-server", fLdapServer)
	panicWhenEmpty("base-dn", fBaseDN)
	panicWhenEmpty("readonly-user", fReadonlyUser)
	panicWhenEmpty("readonly-password", fReadonlyPassword)
	panicWhenEmpty("db-path", fDBPath)

	ldapConfig := ldap.Config{
		Server:            *fLdapServer,
		BaseDN:            *fBaseDN,
		IsActiveDirectory: *fIsActiveDirectory,
	}

	return &Opts{
		LDAP:             ldapConfig,
		ReadonlyUser:     *fReadonlyUser,
		ReadonlyPassword: *fReadonlyPassword,

		DBPath: *fDBPath,
	}
}
