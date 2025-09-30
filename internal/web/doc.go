// Package web provides the HTTP server and web interface for LDAP Manager.
//
// This package implements a complete web application using the Fiber v2 framework,
// providing HTTP handlers, middleware, session management, template rendering,
// and static asset serving for LDAP directory management operations.
//
// # Architecture
//
// The web package follows a layered architecture with clear separation of concerns:
//
//	┌─────────────────────────────────────┐
//	│  HTTP Layer (Fiber Handlers)       │
//	│  • Routing and request handling     │
//	│  • Session-based authentication     │
//	│  • Template rendering (Templ)       │
//	└─────────────────────────────────────┘
//	            ↓
//	┌─────────────────────────────────────┐
//	│  Business Logic Integration         │
//	│  • LDAP cache manager               │
//	│  • Connection pool management       │
//	│  • Data transformation              │
//	└─────────────────────────────────────┘
//
// # Core Components
//
// App is the central application structure that encapsulates all web server functionality:
//
//	type App struct {
//	    ldapClient    *ldap.LDAP          // LDAP client for directory operations
//	    ldapPool      *ldappool.PoolManager  // Connection pool manager
//	    ldapCache     *ldap_cache.Manager    // Cache layer for LDAP data
//	    sessionStore  *session.Store          // Session management
//	    templateCache *TemplateCache          // Template caching
//	    fiber         *fiber.App              // Fiber web framework
//	}
//
// # Request Handling
//
// The package organizes HTTP handlers into logical groups:
//
//   - Authentication: Login/logout with session management (auth.go)
//   - Users: List, view, and modify LDAP user accounts (users.go)
//   - Groups: List, view, and manage LDAP groups (groups.go)
//   - Computers: List and view computer accounts (computers.go)
//   - Health: Health checks and monitoring endpoints (health.go)
//   - Admin: Debug and statistics endpoints (server.go)
//
// # Middleware
//
// The package provides several middleware components for request processing:
//
//   - RequireAuth: Ensures user authentication for protected routes
//   - OptionalAuth: Provides user context without requiring authentication
//   - CSRF Protection: Prevents cross-site request forgery attacks
//   - Template Caching: Caches rendered templates for performance
//   - Security Headers: Sets appropriate security headers (Helmet)
//   - Compression: Compresses responses for bandwidth optimization
//
// # Session Management
//
// Sessions are managed using Fiber's session middleware with configurable storage:
//
//   - Memory Storage: In-memory sessions for development
//   - BBolt Storage: Persistent sessions using embedded database
//
// Session configuration includes:
//
//   - HTTP-only cookies (XSS protection)
//   - SameSite=Strict (CSRF protection)
//   - Secure flag for HTTPS
//   - Configurable expiration (default: 30 minutes)
//
// # Template System
//
// Templates use the Templ library for type-safe, compiled HTML generation:
//
//   - Compile-time type checking
//   - Automatic HTML escaping
//   - Component-based architecture
//   - Template caching with automatic invalidation
//
// Template caching behavior:
//
//   - GET requests: Cached until POST operation
//   - POST requests: Invalidate relevant cache entries
//   - Cache key: Based on URL path and user context
//   - Cache headers: X-Cache: HIT or MISS for debugging
//
// # Security
//
// The package implements multiple security measures:
//
//   - Session-based authentication (no tokens exposed)
//   - CSRF protection on all state-changing operations
//   - Input validation and sanitization
//   - LDAP injection prevention (query escaping)
//   - Security headers (CSP, X-Frame-Options, etc.)
//   - User-context LDAP operations (no privilege escalation)
//
// # Caching Strategy
//
// The package uses a multi-level caching approach for optimal performance:
//
//  1. LDAP Cache: 30-second TTL with background refresh
//  2. Template Cache: Until invalidated by POST operations
//  3. Connection Pool: Reused LDAP connections
//  4. Static Assets: Browser caching with max-age headers
//
// Cache invalidation rules:
//
//   - User modifications: Invalidate /users/* caches
//   - Group modifications: Invalidate /groups/* caches
//   - Any POST operation: Invalidate all GET template caches
//
// # Error Handling
//
// Error handling follows consistent patterns:
//
//   - 401 Unauthorized: Authentication required
//   - 403 Forbidden: CSRF token invalid or insufficient permissions
//   - 404 Not Found: Resource doesn't exist
//   - 500 Internal Server Error: Unexpected errors (logged)
//
// All errors are logged with structured logging using zerolog.
//
// # Health Checks
//
// The package provides multiple health check endpoints for monitoring:
//
//   - GET /health: Comprehensive health with cache and pool metrics
//   - GET /health/ready: Readiness probe for Kubernetes
//   - GET /health/live: Liveness probe for Kubernetes
//   - GET /debug/cache: Template cache statistics (authenticated)
//   - GET /debug/ldap-pool: Connection pool statistics (authenticated)
//
// # API Endpoints
//
// Public endpoints (no authentication):
//
//	POST /login              - User authentication
//	GET  /health             - Health check
//	GET  /health/ready       - Readiness probe
//	GET  /health/live        - Liveness probe
//
// Protected endpoints (authentication required):
//
//	GET  /                   - Dashboard/home page
//	GET  /logout             - Session termination
//	GET  /users              - List all users
//	GET  /users/:userDN      - View user details
//	POST /users/:userDN      - Modify user attributes
//	GET  /groups             - List all groups
//	GET  /groups/:groupDN    - View group details
//	POST /groups/:groupDN    - Modify group membership
//	GET  /computers          - List computer accounts
//	GET  /computers/:computerDN - View computer details
//
// Debug endpoints (authenticated, for monitoring):
//
//	GET  /debug/cache        - Template cache statistics
//	GET  /debug/ldap-pool    - Connection pool health
//
// # Usage Example
//
// Creating and starting the web application:
//
//	opts := options.Parse()
//	app, err := web.NewApp(opts)
//	if err != nil {
//	    log.Fatal().Err(err).Msg("Failed to create app")
//	}
//
//	if err := app.Listen(":3000"); err != nil {
//	    log.Fatal().Err(err).Msg("Failed to start server")
//	}
//
// # Configuration
//
// The web application is configured through the options package:
//
//   - PORT: HTTP listen port (default: 3000)
//   - SESSION_DURATION: Session timeout (default: 30m)
//   - PERSIST_SESSIONS: Use BBolt for persistent sessions
//   - SESSION_PATH: BBolt database path (default: /data/session.bbolt)
//   - LOG_LEVEL: Logging verbosity (debug/info/warn/error)
//
// # Performance Considerations
//
// The package is optimized for performance through:
//
//   - Connection pooling: Reuses LDAP connections
//   - LDAP caching: Reduces directory queries (30s TTL)
//   - Template caching: Avoids re-rendering GET requests
//   - Response compression: Reduces bandwidth usage
//   - Static asset caching: Browser-side caching (24h max-age)
//
// Typical performance characteristics:
//
//   - Cold start: ~5 seconds (cache warming)
//   - Cached requests: <10ms response time
//   - Uncached requests: ~100ms (LDAP query)
//   - Template rendering: ~5ms per page
//
// # Testing
//
// The package includes comprehensive tests:
//
//   - Unit tests: Handler logic and middleware
//   - Integration tests: Full request/response cycles
//   - Template tests: Rendering and caching behavior
//
// Run tests with:
//
//	go test ./internal/web/...
//	go test -race ./internal/web/...  # Race detection
//
// # Related Documentation
//
// See also:
//
//   - internal/ldap_cache: LDAP caching layer
//   - internal/ldap: Connection pool management
//   - internal/options: Configuration parsing
//   - internal/web/templates: Templ template components
//   - docs/API_REFERENCE.md: Complete API documentation
//   - docs/development/architecture.md: System architecture
package web
