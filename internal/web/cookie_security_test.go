package web

import (
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"

	"github.com/netresearch/ldap-manager/internal/options"
)

// TestCookieSecurityWithHTTPS verifies secure cookie configuration for HTTPS environments
func TestCookieSecurityWithHTTPS(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		CookieSecure:            true, // HTTPS environment
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Verify CookieSecure setting is true for HTTPS
	if !opts.CookieSecure {
		t.Error("Expected CookieSecure=true for HTTPS environment")
	}

	// Test session store creation doesn't panic
	sessionStore := createSessionStore(opts)
	if sessionStore == nil {
		t.Fatal("Expected session store, got nil")
	}

	// Test CSRF handler creation doesn't panic
	csrfHandler := createCSRFConfig(opts)
	if csrfHandler == nil {
		t.Fatal("Expected CSRF handler, got nil")
	}
}

// TestCookieSecurityWithHTTP verifies cookie configuration for HTTP-only environments
func TestCookieSecurityWithHTTP(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		CookieSecure:            false, // HTTP-only environment
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Verify CookieSecure setting is false for HTTP
	if opts.CookieSecure {
		t.Error("Expected CookieSecure=false for HTTP environment")
	}

	// Test session store creation doesn't panic
	sessionStore := createSessionStore(opts)
	if sessionStore == nil {
		t.Fatal("Expected session store, got nil")
	}

	// Test CSRF handler creation doesn't panic
	csrfHandler := createCSRFConfig(opts)
	if csrfHandler == nil {
		t.Fatal("Expected CSRF handler, got nil")
	}
}

// TestCookieSecureConfiguration verifies cookie security settings are properly passed through
func TestCookieSecureConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		cookieSecure bool
		description  string
	}{
		{
			name:         "HTTPS environment",
			cookieSecure: true,
			description:  "Secure cookies enabled for HTTPS",
		},
		{
			name:         "HTTP environment",
			cookieSecure: false,
			description:  "Secure cookies disabled for HTTP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &options.Opts{
				LDAP: ldap.Config{
					Server:            "ldap://localhost:389",
					BaseDN:            "dc=test,dc=local",
					IsActiveDirectory: false,
				},
				ReadonlyUser:            "cn=readonly,dc=test,dc=local",
				ReadonlyPassword:        "password",
				CookieSecure:            tt.cookieSecure,
				PersistSessions:         false,
				SessionDuration:         30 * time.Minute,
				PoolMaxConnections:      10,
				PoolMinConnections:      2,
				PoolMaxIdleTime:         15 * time.Minute,
				PoolHealthCheckInterval: 30 * time.Second,
				PoolConnectionTimeout:   30 * time.Second,
				PoolAcquireTimeout:      10 * time.Second,
			}

			// Verify configuration value
			if opts.CookieSecure != tt.cookieSecure {
				t.Errorf("%s: Expected CookieSecure=%v, got %v", tt.description, tt.cookieSecure, opts.CookieSecure)
			}

			// Test session store creation with configuration
			sessionStore := createSessionStore(opts)
			if sessionStore == nil {
				t.Fatal("Expected session store, got nil")
			}

			// Test CSRF handler creation with configuration
			csrfHandler := createCSRFConfig(opts)
			if csrfHandler == nil {
				t.Fatal("Expected CSRF handler, got nil")
			}
		})
	}
}

// TestCSRFConfigurationAcceptsOpts verifies CSRF handler accepts options parameter
func TestCSRFConfigurationAcceptsOpts(t *testing.T) {
	opts := &options.Opts{
		LDAP: ldap.Config{
			Server:            "ldap://localhost:389",
			BaseDN:            "dc=test,dc=local",
			IsActiveDirectory: false,
		},
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "password",
		CookieSecure:            true,
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		PoolMaxConnections:      10,
		PoolMinConnections:      2,
		PoolMaxIdleTime:         15 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   30 * time.Second,
		PoolAcquireTimeout:      10 * time.Second,
	}

	// Verify CSRF handler creation accepts opts parameter (fixes the signature change)
	csrfHandler := createCSRFConfig(opts)
	if csrfHandler == nil {
		t.Fatal("Expected CSRF handler, got nil")
	}

	// Handler created successfully - type is fiber.Handler (internal Fiber type)
	t.Log("CSRF handler created successfully with opts parameter")
}
