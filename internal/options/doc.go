// Package options provides comprehensive configuration management for the LDAP Manager application,
// supporting multiple configuration sources with priority-based resolution.
//
// # Overview
//
// This package handles all application configuration parsing from environment variables,
// command-line flags, and .env files. It provides type-safe configuration with validation,
// default values, and clear error messages for missing or invalid settings.
//
// Configuration sources are processed in priority order:
//
//  1. Command-line flags (highest priority)
//  2. Environment variables
//  3. .env files (.env.local, .env)
//  4. Default values (lowest priority)
//
// # Usage
//
// Basic usage in main.go:
//
//	import (
//	    "github.com/netresearch/ldap-manager/internal/options"
//	    "github.com/rs/zerolog/log"
//	)
//
//	func main() {
//	    // Parse configuration from all sources
//	    opts := options.Parse()
//
//	    // Configure logging with parsed log level
//	    zerolog.SetGlobalLevel(opts.LogLevel)
//
//	    // Use configuration throughout application
//	    ldapClient, err := ldap.New(opts.LDAP, opts.ReadonlyUser, opts.ReadonlyPassword)
//	    if err != nil {
//	        log.Fatal().Err(err).Msg("Failed to create LDAP client")
//	    }
//	}
//
// # Configuration Options
//
// ## Required Settings
//
// The following settings MUST be provided (via flags, env vars, or .env):
//
//	LDAP_SERVER           LDAP server URI (ldap:// or ldaps://)
//	LDAP_BASE_DN          Base Distinguished Name for directory
//	LDAP_READONLY_USER    Read-only service account username
//	LDAP_READONLY_PASSWORD Read-only service account password
//
// Example .env file for required settings:
//
//	LDAP_SERVER=ldaps://dc1.example.com:636
//	LDAP_BASE_DN=DC=example,DC=com
//	LDAP_READONLY_USER=cn=readonly,DC=example,DC=com
//	LDAP_READONLY_PASSWORD=SecurePassword123
//
// ## Optional LDAP Settings
//
// Additional LDAP connection configuration:
//
//	LDAP_IS_AD=true                       # Mark server as Active Directory (default: false)
//
// ## Session Management
//
// Session storage and lifetime configuration:
//
//	PERSIST_SESSIONS=false                # Enable BBolt session persistence (default: false)
//	SESSION_PATH=./session.bbolt          # Database file path (default: db.bbolt)
//	SESSION_DURATION=30m                  # Session timeout (default: 30 minutes)
//
// When PERSIST_SESSIONS=true, sessions survive application restarts.
// When PERSIST_SESSIONS=false, sessions are stored in memory only.
//
// ## Connection Pool Settings (PR #267)
//
// LDAP connection pool configuration for performance and resource management:
//
//	LDAP_POOL_MAX_CONNECTIONS=10          # Maximum pool size (default: 10)
//	LDAP_POOL_MIN_CONNECTIONS=2           # Minimum pool size (default: 2)
//	LDAP_POOL_MAX_IDLE_TIME=15m          # Maximum idle time (default: 15 minutes)
//	LDAP_POOL_MAX_LIFETIME=1h             # Maximum connection lifetime (default: 1 hour)
//	LDAP_POOL_HEALTH_CHECK_INTERVAL=30s   # Health check frequency (default: 30 seconds)
//	LDAP_POOL_ACQUIRE_TIMEOUT=10s         # Pool acquisition timeout (default: 10 seconds)
//
// ## Logging Configuration
//
// Control application logging verbosity:
//
//	LOG_LEVEL=info                        # Log level: trace, debug, info, warn, error, fatal, panic
//
// # Configuration Priority
//
// Configuration values are resolved using the following priority order (highest to lowest):
//
//  1. Command-line flags: --ldap-server, --base-dn, etc.
//  2. Environment variables: LDAP_SERVER, LDAP_BASE_DN, etc.
//  3. .env files: .env.local (overrides .env), .env
//  4. Default values: Built-in sensible defaults
//
// Example demonstrating priority:
//
//	# In .env file
//	LOG_LEVEL=info
//
//	# In environment
//	export LOG_LEVEL=debug
//
//	# Command-line flag (highest priority)
//	./ldap-manager --log-level trace
//
//	# Result: LOG_LEVEL=trace (command-line flag wins)
//
// # Environment File Format
//
// The .env file uses KEY=VALUE format (loaded via github.com/joho/godotenv):
//
//	# Required LDAP settings
//	LDAP_SERVER=ldaps://dc1.example.com:636
//	LDAP_BASE_DN=DC=example,DC=com
//	LDAP_READONLY_USER=cn=readonly,DC=example,DC=com
//	LDAP_READONLY_PASSWORD=SecurePassword123
//
//	# Optional settings
//	LDAP_IS_AD=true
//	LOG_LEVEL=debug
//
//	# Session management
//	PERSIST_SESSIONS=true
//	SESSION_PATH=./session.bbolt
//	SESSION_DURATION=1h
//
//	# Connection pool configuration
//	LDAP_POOL_MAX_CONNECTIONS=20
//	LDAP_POOL_MIN_CONNECTIONS=5
//	LDAP_POOL_MAX_IDLE_TIME=10m
//	LDAP_POOL_HEALTH_CHECK_INTERVAL=30s
//	LDAP_POOL_ACQUIRE_TIMEOUT=10s
//
// Two .env files are supported:
//
//   - .env.local: Local overrides (not committed to version control)
//   - .env: Default settings (can be committed as .env.example)
//
// # Validation
//
// The package performs comprehensive validation:
//
//   - Required fields: Application exits with error if missing
//   - Type validation: Duration, boolean, integer values validated at parse time
//   - Format validation: Log levels, LDAP URIs validated
//   - Conditional requirements: SESSION_PATH required when PERSIST_SESSIONS=true
//
// Validation errors cause application to exit with descriptive messages:
//
//	FATAL the option --ldap-server is required
//	FATAL could not parse environment variable "SESSION_DURATION" (containing "invalid") as duration: ...
//
// # Type Conversions
//
// The package provides type-safe conversions for all configuration values:
//
//   - Strings: Direct environment variable values
//   - Durations: time.ParseDuration() for values like "30m", "1h", "10s"
//   - Booleans: strconv.ParseBool() for values like "true", "false", "1", "0"
//   - Integers: strconv.Atoi() for numeric values
//   - Log Levels: zerolog.ParseLevel() for "trace", "debug", "info", etc.
//
// All conversions include error handling with descriptive failure messages.
//
// # Command-Line Flags
//
// All settings can be provided via command-line flags:
//
//	./ldap-manager \
//	  --ldap-server ldaps://dc1.example.com:636 \
//	  --base-dn DC=example,DC=com \
//	  --readonly-user cn=readonly,DC=example,DC=com \
//	  --readonly-password SecurePassword123 \
//	  --log-level debug \
//	  --persist-sessions \
//	  --session-path ./session.bbolt \
//	  --session-duration 1h \
//	  --pool-max-connections 20 \
//	  --pool-min-connections 5
//
// Run with --help to see all available flags and their descriptions.
//
// # Integration Points
//
// The Opts struct is used throughout the application:
//
//   - cmd/ldap-manager/main.go: Initial Parse() call
//   - internal/web/server.go: Session and pool configuration
//   - internal/ldap/pool.go: Connection pool settings
//
// # Best Practices
//
// Recommended configuration approach:
//
//  1. Use .env files for local development (add .env.local to .gitignore)
//  2. Use environment variables for production deployments
//  3. Use command-line flags for quick testing and overrides
//  4. Never commit .env files with real credentials to version control
//  5. Provide .env.example as a template with placeholder values
//
// For more details on configuration options, see: docs/user-guide/configuration.md
package options
