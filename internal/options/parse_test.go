package options

import (
	"flag"
	"os"
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

// Helper to set multiple environment variables and return cleanup function
func setEnvVarsForParse(t *testing.T, vars map[string]string) func() {
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
		if len(f.Name) >= 5 && f.Name[:5] == "test." {
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

func TestParse_InvalidLogLevel(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LOG_LEVEL"] = "invalid_level"
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LOG_LEVEL")
	}
}

func TestParse_InvalidLDAPIsAD(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_IS_AD"] = notABool
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_IS_AD")
	}
}

func TestParse_InvalidPersistSessions(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["PERSIST_SESSIONS"] = notABool
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid PERSIST_SESSIONS")
	}
}

func TestParse_InvalidSessionDuration(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["SESSION_DURATION"] = notADuration
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid SESSION_DURATION")
	}
}

func TestParse_InvalidCookieSecure(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["COOKIE_SECURE"] = notABool
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid COOKIE_SECURE")
	}
}

func TestParse_InvalidTLSSkipVerify(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_TLS_SKIP_VERIFY"] = notABool
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_TLS_SKIP_VERIFY")
	}
}

func TestParse_InvalidPoolMaxConnections(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_MAX_CONNECTIONS"] = notAnInt
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_MAX_CONNECTIONS")
	}
}

func TestParse_InvalidPoolMinConnections(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_MIN_CONNECTIONS"] = notAnInt
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_MIN_CONNECTIONS")
	}
}

func TestParse_InvalidPoolMaxIdleTime(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_MAX_IDLE_TIME"] = notADuration
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_MAX_IDLE_TIME")
	}
}

func TestParse_InvalidPoolMaxLifetime(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_MAX_LIFETIME"] = notADuration
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_MAX_LIFETIME")
	}
}

func TestParse_InvalidPoolHealthCheckInterval(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_HEALTH_CHECK_INTERVAL"] = notADuration
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_HEALTH_CHECK_INTERVAL")
	}
}

func TestParse_InvalidPoolConnectionTimeout(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_CONNECTION_TIMEOUT"] = notADuration
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_CONNECTION_TIMEOUT")
	}
}

func TestParse_InvalidPoolAcquireTimeout(t *testing.T) {
	resetFlags()
	vars := validEnvVarsForParse()
	vars["LDAP_POOL_ACQUIRE_TIMEOUT"] = notADuration
	defer setEnvVarsForParse(t, vars)()

	_, err := Parse()
	if err == nil {
		t.Error("Expected error for invalid LDAP_POOL_ACQUIRE_TIMEOUT")
	}
}

func TestParse_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name       string
		removeKey  string
		wantField  string
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
			defer setEnvVarsForParse(t, vars)()

			_, err := Parse()
			if err == nil {
				t.Errorf("Expected error for missing %s", tt.removeKey)
			}
		})
	}
}

func TestParse_PersistSessionsWithPath(t *testing.T) {
	// When PERSIST_SESSIONS is true and SESSION_PATH is empty,
	// the default "db.bbolt" is used, so no error occurs.
	// This test verifies that PERSIST_SESSIONS=true works with the default path.
	resetFlags()
	vars := validEnvVarsForParse()
	vars["PERSIST_SESSIONS"] = trueStr
	// Not setting SESSION_PATH - uses default "db.bbolt"
	defer setEnvVarsForParse(t, vars)()

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
	defer setEnvVarsForParse(t, vars)()

	opts, err := Parse()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if opts.LogLevel != zerolog.DebugLevel {
		t.Errorf("Expected DebugLevel, got %v", opts.LogLevel)
	}
	if opts.LDAP.Server != "ldap://localhost:389" {
		t.Errorf("Expected ldap://localhost:389, got %s", opts.LDAP.Server)
	}
	if !opts.LDAP.IsActiveDirectory {
		t.Error("Expected IsActiveDirectory to be true")
	}
	if !opts.PersistSessions {
		t.Error("Expected PersistSessions to be true")
	}
	if opts.PoolMaxConnections != 20 {
		t.Errorf("Expected 20, got %d", opts.PoolMaxConnections)
	}
}
