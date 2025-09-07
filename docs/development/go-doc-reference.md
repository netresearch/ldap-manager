# Go Documentation Reference

Complete Go package documentation for LDAP Manager, generated from inline doc comments.

## Core Packages

### cmd/ldap-manager

**Package main** provides the entry point for the LDAP Manager web application. It initializes logging, parses configuration options, and starts the web server.

#### Functions

- **main()** - Application entry point that configures logging, parses options, creates web app instance, and starts HTTP server on port 3000.

---

### internal/options

**Package options** provides configuration parsing and environment variable handling for the LDAP Manager application.

#### Types

**type Opts struct** - Holds all configuration options for the LDAP Manager application. Includes LDAP connection settings, session management, connection pooling, and logging configuration.

Fields:
- `LogLevel zerolog.Level` - Application log level
- `LDAP ldap.Config` - LDAP server configuration  
- `ReadonlyUser string` - Service account username for LDAP operations
- `ReadonlyPassword string` - Service account password
- `PersistSessions bool` - Whether to persist sessions to database
- `SessionPath string` - Path to session database file
- `SessionDuration time.Duration` - Session timeout duration
- `PoolMaxConnections int` - Maximum LDAP pool connections
- `PoolMinConnections int` - Minimum LDAP pool connections
- `PoolMaxIdleTime time.Duration` - Maximum connection idle time
- `PoolMaxLifetime time.Duration` - Maximum connection lifetime
- `PoolHealthCheckInterval time.Duration` - Health check interval
- `PoolAcquireTimeout time.Duration` - Connection acquisition timeout

#### Functions

**Parse() *Opts** - Parses command line flags and environment variables to build application configuration. Loads from .env files, parses flags, and validates required settings.

---

### internal/version

**Package version** provides build-time information and version management.

#### Variables

- **Version string** - Application version string, set during build time (default: "dev")
- **CommitHash string** - Git commit hash (default: "n/a")
- **BuildTimestamp string** - Build timestamp (default: "n/a")

#### Functions

**FormatVersion() string** - Returns a human-readable version string including build metadata. Returns "Development version" for dev builds, or formatted version with commit and timestamp.

---

### internal/ldap

**Package ldap** provides connection pool management for LDAP operations.

#### Types

**type PoolManager struct** - Provides a high-level interface for LDAP connection pool operations. Wraps the connection pool and provides convenient methods for common LDAP tasks.

**type PooledLDAPClient struct** - Represents an LDAP client obtained from the connection pool. Automatically returns the connection to the pool when closed.

**type ConnectionPool struct** - Manages a pool of LDAP connections for efficient reuse.

**type PoolConfig struct** - Contains configuration options for the LDAP connection pool.

#### Key Methods

**NewPoolManager(baseClient *ldap.LDAP, config *PoolConfig) (*PoolManager, error)** - Creates a new pool manager with the specified base client and configuration.

**WithCredentials(ctx context.Context, dn, password string) (*PooledLDAPClient, error)** - Gets an authenticated LDAP client from the connection pool. Replaces the simple-ldap-go WithCredentials method with pooled connections.

**GetReadOnlyClient(ctx context.Context) (*PooledLDAPClient, error)** - Gets a read-only LDAP client from the connection pool. Useful for operations that don't require specific user credentials.

**GetStats() PoolStats** - Returns connection pool statistics.

**GetHealthStatus() map[string]interface{}** - Returns the health status of the connection pool.

---

### internal/ldap_cache

**Package ldap_cache** provides efficient caching of LDAP directory data with automatic refresh capabilities. Maintains synchronized in-memory caches for users, groups, and computers with concurrent-safe operations.

#### Types

**type Manager struct** - Coordinates LDAP data caching with automatic background refresh. Maintains separate caches for users, groups, and computers with configurable refresh intervals.

Fields:
- `Users Cache[ldap.User]` - Cached user entries with O(1) indexed lookups
- `Groups Cache[ldap.Group]` - Cached group entries with O(1) indexed lookups  
- `Computers Cache[ldap.Computer]` - Cached computer entries with O(1) indexed lookups

**type Cache[T cacheable] struct** - Provides thread-safe storage for LDAP entities with O(1) indexed lookups. Maintains both slice storage for iteration and hash-based indexes for fast lookups.

**type FullLDAPUser struct** - Represents a user with populated group memberships.

**type FullLDAPGroup struct** - Represents a group with populated member list.

**type FullLDAPComputer struct** - Represents a computer with populated group memberships.

#### Key Methods

**New(client LDAPClient) *Manager** - Creates a new LDAP cache manager with 30-second refresh interval.

**NewWithConfig(client LDAPClient, refreshInterval time.Duration) *Manager** - Creates a new LDAP cache manager with configurable refresh interval.

**FindUsers(includeDisabled bool) []ldap.User** - Returns all cached users, optionally including disabled accounts.

**FindUserByDN(dn string) (ldap.User, error)** - Finds a specific user by Distinguished Name.

**PopulateGroupsForUser(user ldap.User) FullLDAPUser** - Returns user with all group memberships populated.

---

### internal/web

**Package web** provides the HTTP server and handlers for the LDAP Manager web interface.

#### Types

**type App struct** - Represents the main web application structure. Encapsulates LDAP client, connection pool, cache manager, session store, template cache, and Fiber web framework.

Fields:
- `ldapClient *ldap.LDAP` - LDAP client instance
- `ldapPool *ldappool.PoolManager` - Connection pool manager
- `ldapCache *ldap_cache.Manager` - Cache manager
- `sessionStore *session.Store` - Session storage
- `templateCache *TemplateCache` - Template caching system
- `fiber *fiber.App` - Fiber web framework instance

#### Key Methods

**NewApp(opts *options.Opts) (*App, error)** - Creates a new web application instance with provided configuration. Initializes LDAP client, connection pool, session management, template cache, and Fiber web server.

**Listen(addr string) error** - Starts the web application server on the specified address. Launches LDAP cache manager in background and begins serving HTTP requests.

**Shutdown() error** - Gracefully shuts down the application including template cache and LDAP connection pool.

#### Handler Methods

**usersHandler(c *fiber.Ctx) error** - Handles GET /users requests to list all user accounts. Supports show-disabled query parameter.

**userHandler(c *fiber.Ctx) error** - Handles GET /users/:userDN requests for specific user details.

**userModifyHandler(c *fiber.Ctx) error** - Handles POST /users/:userDN requests to modify user attributes.

**groupsHandler(c *fiber.Ctx) error** - Handles GET /groups requests to list all groups.

**groupHandler(c *fiber.Ctx) error** - Handles GET /groups/:groupDN requests for specific group details.

**computersHandler(c *fiber.Ctx) error** - Handles GET /computers requests to list all computer accounts.

**computerHandler(c *fiber.Ctx) error** - Handles GET /computers/:computerDN requests for specific computer details.

---

## Template System

### internal/web/templates

Uses [templ](https://templ.guide/) for type-safe HTML templating with Go. Templates are compiled to Go code for performance.

#### Key Templates

- `Base(title, content)` - Base HTML layout with navigation
- `Index(user)` - Dashboard page for authenticated users
- `Users(users, showDisabled, flashes)` - User listing page
- `User(user, groups, flashes, token)` - User detail and edit page
- `Groups(groups)` - Group listing page
- `Group(group, users)` - Group detail page
- `Computers(computers)` - Computer listing page
- `Computer(computer)` - Computer detail page
- `Login(flashes)` - Login form page
- `FourOhFour(path)` - 404 error page
- `FiveHundred(err)` - 500 error page

---

## Performance Features

### Connection Pooling

The LDAP connection pool provides:
- **Concurrent Connections**: Up to 10 simultaneous LDAP connections
- **Connection Reuse**: Automatic pooling and reuse of authenticated connections  
- **Health Monitoring**: Periodic health checks and automatic recovery
- **Graceful Degradation**: Handles connection failures and timeouts
- **Metrics**: Comprehensive statistics for monitoring

### Caching System

Multi-level caching architecture:
- **LDAP Data Cache**: 30-second refresh for users/groups/computers
- **Template Cache**: Rendered HTML templates cached with automatic invalidation
- **Static Asset Cache**: 24-hour browser cache for CSS/JS/images
- **Session Cache**: In-memory or persistent session storage

### Template Caching

The template caching system provides:
- **Automatic Cache Keys**: Generated from request path and parameters
- **Cache Invalidation**: Automatic invalidation after data modifications
- **Cache Statistics**: Hit/miss ratios and performance metrics
- **Memory Management**: LRU eviction and configurable size limits

---

## Security Architecture

### Authentication

- **LDAP Authentication**: Direct validation against directory server
- **Session Management**: HTTP-only, SameSite=Strict cookies
- **User Context**: All operations performed with authenticated user's credentials
- **Automatic Expiration**: Configurable session timeouts

### Security Headers

- **Content Security Policy**: Strict CSP with minimal allowed sources
- **HSTS**: HTTP Strict Transport Security enabled
- **X-Frame-Options**: Clickjacking protection
- **X-Content-Type-Options**: MIME sniffing protection
- **XSS Protection**: Cross-site scripting protection

### CSRF Protection

- **Token-based Protection**: Unique tokens for form submissions
- **SameSite Cookies**: Additional CSRF protection via cookie policy
- **Validation**: Server-side token validation for all state-changing operations

---

## Monitoring and Observability

### Health Checks

- **GET /health** - Basic application health
- **GET /health/ready** - Readiness probe for load balancers
- **GET /health/live** - Liveness probe for container orchestration

### Debug Endpoints

- **GET /debug/cache** - Template cache statistics (authenticated)
- **GET /debug/ldap-pool** - Connection pool statistics (authenticated)

### Metrics

Comprehensive metrics collection:
- **Cache Performance**: Hit/miss ratios, eviction counts
- **Connection Pool**: Active connections, acquisition times
- **Request Metrics**: Response times, error rates
- **LDAP Operations**: Query times, success/failure rates

### Logging

Structured logging with configurable levels:
- **Levels**: trace, debug, info, warn, error, fatal, panic
- **Format**: JSON logging for production, console for development
- **Context**: Request IDs and user context in all log entries

---

## Generated Documentation

This documentation is derived from Go doc comments in the source code. For the most up-to-date information, use:

```bash
# Generate and serve documentation locally
go doc -all ./cmd/ldap-manager
go doc -all ./internal/options  
go doc -all ./internal/ldap
go doc -all ./internal/ldap_cache
go doc -all ./internal/web

# Or use godoc server
godoc -http=:6060
```

Visit `http://localhost:6060/pkg/github.com/netresearch/ldap-manager/` for interactive documentation.