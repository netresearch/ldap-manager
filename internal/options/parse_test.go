package options

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// Test constants for invalid values
const (
	notABool     = "not_a_bool"
	notADuration = "not_a_duration"
	notAnInt     = "not_an_int"
	trueStr      = "true"
)

// setEnvVars sets multiple environment variables and returns a cleanup function
func setEnvVars(t *testing.T, vars map[string]string) func() {
	t.Helper()
	for k, v := range vars {
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Failed to set env var %s: %v", k, err)
		}
	}

	return func() {
		for k := range vars {
			_ = os.Unsetenv(k)
		}
	}
}

// resetFlags resets the flag package to allow re-parsing while preserving test flags
func resetFlags() {
	// Save test framework flags that were already registered
	testFlags := make(map[string]*flag.Flag)
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "test.") {
			testFlags[f.Name] = f
		}
	})

	// Create new FlagSet with ContinueOnError to avoid os.Exit on unknown flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Re-register test framework flags
	for _, f := range testFlags {
		flag.CommandLine.Var(f.Value, f.Name, f.Usage)
	}
}

// validEnvVarsForParse returns environment variables needed for successful Parse()
func validEnvVarsForParse() map[string]string {
	return map[string]string{
		"LDAP_SERVER":            "ldap://localhost:389",
		"LDAP_BASE_DN":           "dc=example,dc=com",
		"LDAP_READONLY_USER":     "cn=readonly,dc=example,dc=com",
		"LDAP_READONLY_PASSWORD": "secret",
	}
}

func TestParse_InvalidEnvVars(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		invalidValue string
	}{
		{"InvalidLogLevel", "LOG_LEVEL", "invalid_level"},
		{"InvalidLDAPIsAD", "LDAP_IS_AD", notABool},
		{"InvalidPersistSessions", "PERSIST_SESSIONS", notABool},
		{"InvalidSessionDuration", "SESSION_DURATION", notADuration},
		{"InvalidCookieSecure", "COOKIE_SECURE", notABool},
		{"InvalidTLSSkipVerify", "LDAP_TLS_SKIP_VERIFY", notABool},
		{"InvalidPoolMaxConnections", "LDAP_POOL_MAX_CONNECTIONS", notAnInt},
		{"InvalidPoolMinConnections", "LDAP_POOL_MIN_CONNECTIONS", notAnInt},
		{"InvalidPoolMaxIdleTime", "LDAP_POOL_MAX_IDLE_TIME", notADuration},
		{"InvalidPoolMaxLifetime", "LDAP_POOL_MAX_LIFETIME", notADuration},
		{"InvalidPoolHealthCheckInterval", "LDAP_POOL_HEALTH_CHECK_INTERVAL", notADuration},
		{"InvalidPoolConnectionTimeout", "LDAP_POOL_CONNECTION_TIMEOUT", notADuration},
		{"InvalidPoolAcquireTimeout", "LDAP_POOL_ACQUIRE_TIMEOUT", notADuration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			vars := validEnvVarsForParse()
			vars[tt.envKey] = tt.invalidValue
			defer setEnvVars(t, vars)()

			_, err := Parse()
			if err == nil {
				t.Errorf("Expected error for invalid %s", tt.envKey)
			}
		})
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		removeKey string
		wantField string
	}{
		{"MissingLDAPServer", "LDAP_SERVER", "ldap-server"},
		{"MissingBaseDN", "LDAP_BASE_DN", "base-dn"},
		{"MissingReadonlyUser", "LDAP_READONLY_USER", "readonly-user"},
		{"MissingReadonlyPassword", "LDAP_READONLY_PASSWORD", "readonly-password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			vars := validEnvVarsForParse()
			delete(vars, tt.removeKey)
			defer setEnvVars(t, vars)()

			_, err := Parse()
			if err == nil {
				t.Errorf("Expected error for missing %s", tt.removeKey)

				return
			}
			// Verify error message contains expected field name
			if !strings.Contains(err.Error(), tt.wantField) {
				t.Errorf("Expected error to contain field %q, got: %v", tt.wantField, err)
			}
		})
	}
}

func TestParse_PersistSessionsWithPath(t *testing.T) {
	// Verifies that PERSIST_SESSIONS=true uses the default SessionPath "db.bbolt" when SESSION_PATH is not set.
	resetFlags()
	vars := validEnvVarsForParse()
	vars["PERSIST_SESSIONS"] = trueStr
	// Not setting SESSION_PATH - uses default "db.bbolt"
	defer setEnvVars(t, vars)()

	opts, err := Parse()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !opts.PersistSessions {
		t.Error("Expected PersistSessions to be true")
	}
	if opts.SessionPath != "db.bbolt" {
		t.Errorf("Expected default SessionPath 'db.bbolt', got %q", opts.SessionPath)
	}
}

func TestParse_Success(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LOG_LEVEL"] = "debug"
	vars["LDAP_IS_AD"] = trueStr
	vars["PERSIST_SESSIONS"] = trueStr
	vars["SESSION_PATH"] = "/tmp/sessions.db"
	vars["SESSION_DURATION"] = "1h"
	vars["COOKIE_SECURE"] = "false"
	vars["LDAP_TLS_SKIP_VERIFY"] = trueStr
	vars["LDAP_POOL_MAX_CONNECTIONS"] = "20"
	vars["LDAP_POOL_MIN_CONNECTIONS"] = "5"
	vars["LDAP_POOL_MAX_IDLE_TIME"] = "10m"
	vars["LDAP_POOL_MAX_LIFETIME"] = "2h"
	vars["LDAP_POOL_HEALTH_CHECK_INTERVAL"] = "1m"
	vars["LDAP_POOL_CONNECTION_TIMEOUT"] = "45s"
	vars["LDAP_POOL_ACQUIRE_TIMEOUT"] = "15s"
	defer setEnvVars(t, vars)()

	opts, err := Parse()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify all parsed options
	if opts.LogLevel != zerolog.DebugLevel {
		t.Errorf("LogLevel: expected DebugLevel, got %v", opts.LogLevel)
	}
	if opts.LDAP.Server != "ldap://localhost:389" {
		t.Errorf("LDAP.Server: expected ldap://localhost:389, got %s", opts.LDAP.Server)
	}
	if opts.LDAP.BaseDN != "dc=example,dc=com" {
		t.Errorf("LDAP.BaseDN: expected dc=example,dc=com, got %s", opts.LDAP.BaseDN)
	}
	if !opts.LDAP.IsActiveDirectory {
		t.Error("LDAP.IsActiveDirectory: expected true")
	}
	if opts.ReadonlyUser != "cn=readonly,dc=example,dc=com" {
		t.Errorf("ReadonlyUser: expected cn=readonly,dc=example,dc=com, got %s", opts.ReadonlyUser)
	}
	if opts.ReadonlyPassword != "secret" {
		t.Errorf("ReadonlyPassword: expected secret, got %s", opts.ReadonlyPassword)
	}
	if !opts.PersistSessions {
		t.Error("PersistSessions: expected true")
	}
	if opts.SessionPath != "/tmp/sessions.db" {
		t.Errorf("SessionPath: expected /tmp/sessions.db, got %s", opts.SessionPath)
	}
	if opts.SessionDuration.String() != "1h0m0s" {
		t.Errorf("SessionDuration: expected 1h0m0s, got %s", opts.SessionDuration)
	}
	if opts.CookieSecure {
		t.Error("CookieSecure: expected false")
	}
	if !opts.TLSSkipVerify {
		t.Error("TLSSkipVerify: expected true")
	}
	if opts.PoolMaxConnections != 20 {
		t.Errorf("PoolMaxConnections: expected 20, got %d", opts.PoolMaxConnections)
	}
	if opts.PoolMinConnections != 5 {
		t.Errorf("PoolMinConnections: expected 5, got %d", opts.PoolMinConnections)
	}
	if opts.PoolMaxIdleTime.String() != "10m0s" {
		t.Errorf("PoolMaxIdleTime: expected 10m0s, got %s", opts.PoolMaxIdleTime)
	}
	if opts.PoolMaxLifetime.String() != "2h0m0s" {
		t.Errorf("PoolMaxLifetime: expected 2h0m0s, got %s", opts.PoolMaxLifetime)
	}
	if opts.PoolHealthCheckInterval.String() != "1m0s" {
		t.Errorf("PoolHealthCheckInterval: expected 1m0s, got %s", opts.PoolHealthCheckInterval)
	}
	if opts.PoolConnectionTimeout.String() != "45s" {
		t.Errorf("PoolConnectionTimeout: expected 45s, got %s", opts.PoolConnectionTimeout)
	}
	if opts.PoolAcquireTimeout.String() != "15s" {
		t.Errorf("PoolAcquireTimeout: expected 15s, got %s", opts.PoolAcquireTimeout)
	}
}
