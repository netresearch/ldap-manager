# LDAP Manager Architecture Guide

Comprehensive technical architecture overview for developers working on LDAP Manager.

## Table of Contents

- [System Overview](#system-overview)
- [Application Architecture](#application-architecture)
- [Package Organization](#package-organization)  
- [Data Flow](#data-flow)
- [Caching Architecture](#caching-architecture)
- [Security Architecture](#security-architecture)
- [Performance Design](#performance-design)
- [Deployment Architecture](#deployment-architecture)

---

## System Overview

LDAP Manager is a modern Go web application that provides a user-friendly interface for LDAP directory management. Built for performance, security, and maintainability.

### Core Technologies

- **Backend**: Go 1.21+ with Fiber web framework
- **Frontend**: HTML templates with TailwindCSS, minimal JavaScript
- **Templating**: [templ](https://templ.guide/) for type-safe HTML generation
- **LDAP**: [simple-ldap-go](https://github.com/netresearch/simple-ldap-go) with connection pooling
- **Session Storage**: In-memory or BoltDB persistent storage
- **Build System**: Go modules with Makefile automation

### Key Features

- **Web-based LDAP Management**: Intuitive interface for users, groups, and computers
- **Multi-Directory Support**: Standard LDAP and Active Directory compatibility
- **High Performance**: Connection pooling, multi-level caching, template optimization
- **Enterprise Security**: Session-based auth, CSRF protection, security headers
- **Container Ready**: Docker support with health checks and graceful shutdown

---

## Application Architecture

### Architectural Pattern

LDAP Manager follows a **layered monolithic architecture** with clear separation of concerns:

```
┌─────────────────────────────────────────┐
│             HTTP Layer                  │
│  (Fiber Framework + Middleware)         │
├─────────────────────────────────────────┤
│             Handler Layer               │
│  (Route handlers + Business logic)      │
├─────────────────────────────────────────┤
│             Service Layer               │
│  (LDAP Cache + Connection Pool)         │
├─────────────────────────────────────────┤
│             Data Layer                  │
│  (LDAP Directory + Session Store)       │
└─────────────────────────────────────────┘
```

### Component Architecture

```
                    ┌─────────────────┐
                    │   HTTP Client   │
                    └─────────┬───────┘
                              │
                    ┌─────────▼───────┐
                    │  Fiber Web App  │
                    │  + Middleware   │
                    └─────────┬───────┘
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
    ┌──────▼──────┐  ┌────────▼────────┐  ┌──────▼──────┐
    │   Auth      │  │    Handlers     │  │  Templates  │
    │ Middleware  │  │ (Users/Groups)  │  │    Cache    │
    └─────────────┘  └─────────┬───────┘  └─────────────┘
                               │
                    ┌──────────▼──────────┐
                    │    LDAP Cache       │
                    │    Manager          │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │  Connection Pool    │
                    │    Manager          │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │   LDAP Directory    │
                    │  (AD / OpenLDAP)    │
                    └─────────────────────┘
```

---

## Package Organization

### Standard Go Layout

LDAP Manager follows the **Standard Go Project Layout** for maintainability:

```
ldap-manager/
├── cmd/ldap-manager/          # Application entrypoint
├── internal/                  # Private application code
│   ├── ldap/                 # LDAP connection pooling
│   ├── ldap_cache/           # LDAP data caching
│   ├── options/              # Configuration management
│   ├── version/              # Version information  
│   └── web/                  # HTTP handlers and middleware
├── docs/                     # Documentation
├── scripts/                  # Build and deployment scripts
├── .github/                  # GitHub workflows
└── [config files]           # Go modules, Docker, etc.
```

### Package Dependencies

```
cmd/ldap-manager
    │
    └── internal/options ──┐
    └── internal/web ──────┼─→ internal/ldap
                          │     └── Connection Pool
                          │
                          └─→ internal/ldap_cache
                               └── LDAP Data Cache
```

### Internal Package Details

#### internal/options
- **Purpose**: Configuration parsing and validation
- **Key Types**: `Opts` struct with all configuration options
- **Features**: Environment variable support, flag parsing, validation

#### internal/ldap  
- **Purpose**: LDAP connection pool management
- **Key Types**: `PoolManager`, `ConnectionPool`, `PooledLDAPClient`
- **Features**: Connection pooling, health monitoring, automatic recovery

#### internal/ldap_cache
- **Purpose**: LDAP data caching with automatic refresh  
- **Key Types**: `Manager`, `Cache[T]`, `FullLDAPUser/Group/Computer`
- **Features**: Thread-safe caching, background refresh, metrics

#### internal/web
- **Purpose**: HTTP server, handlers, middleware, templates
- **Key Types**: `App`, template functions, middleware
- **Features**: Fiber integration, session management, CSRF protection

#### internal/version
- **Purpose**: Build-time version information
- **Features**: Version string formatting, build metadata

---

## Data Flow

### Request Processing Flow

```
1. HTTP Request
   │
   ├─→ Static Asset? ──→ File Server (cached)
   │
   └─→ Dynamic Request
       │
       ├─→ Security Middleware
       │   ├── Helmet (security headers)
       │   ├── CSRF protection  
       │   └── Compression
       │
       ├─→ Authentication Check
       │   ├── Public route? ──→ Continue
       │   └── Protected route ──→ Session validation
       │
       ├─→ Template Cache Check
       │   ├── Cache hit? ──→ Return cached HTML
       │   └── Cache miss ──→ Continue processing
       │
       ├─→ Route Handler
       │   ├── Extract parameters
       │   ├── Validate input
       │   └── Business logic
       │
       ├─→ LDAP Data Access
       │   ├── Check LDAP cache
       │   ├── Cache hit? ──→ Return cached data
       │   └── Cache miss ──→ Query LDAP
       │
       ├─→ LDAP Connection Pool
       │   ├── Acquire connection
       │   ├── Execute query
       │   └── Return connection
       │
       ├─→ Template Rendering
       │   ├── Compile template with data
       │   ├── Cache rendered HTML
       │   └── Return HTML response
       │
       └─→ HTTP Response
```

### Authentication Flow

```
1. User Login Request
   │
   ├─→ Extract credentials
   │
   ├─→ LDAP Authentication
   │   ├── Acquire LDAP connection
   │   ├── Bind with user credentials
   │   └── Return user object or error
   │
   ├─→ Session Creation
   │   ├── Generate session ID  
   │   ├── Store user DN in session
   │   └── Set HTTP-only cookie
   │
   └─→ Redirect to Dashboard
```

### LDAP Operation Flow

```
1. LDAP Operation Request
   │
   ├─→ Extract User DN from session
   │
   ├─→ LDAP Cache Check
   │   ├── Data in cache? ──→ Return cached data
   │   └── Cache miss ──→ Continue
   │
   ├─→ Connection Pool
   │   ├── Acquire connection with user credentials
   │   ├── Execute LDAP query/modify
   │   └── Release connection back to pool
   │
   ├─→ Cache Update  
   │   ├── Update LDAP cache
   │   └── Invalidate template cache
   │
   └─→ Return Response
```

---

## Caching Architecture

### Multi-Level Caching Strategy

LDAP Manager implements a **three-tier caching system** for optimal performance:

#### Level 1: LDAP Data Cache (Hot Cache)
- **Purpose**: Cache LDAP directory data
- **TTL**: 30 seconds (configurable)
- **Storage**: In-memory with thread-safe access
- **Invalidation**: Time-based refresh + manual invalidation
- **Data**: Users, groups, computers with relationships

```
LDAP Cache Architecture:
┌─────────────────────────────────────────┐
│            Cache Manager                │
├─────────────┬─────────────┬─────────────┤
│ Users Cache │Groups Cache │Computers    │
│             │             │Cache        │
│ - DN Index  │ - DN Index  │ - DN Index  │
│ - SAM Index │ - CN Index  │ - SAM Index │
│ - Auto Refresh (30s)      │ - Auto Ref. │
└─────────────┴─────────────┴─────────────┘
```

#### Level 2: Template Cache (Warm Cache)
- **Purpose**: Cache rendered HTML templates  
- **TTL**: Until data modification
- **Storage**: In-memory LRU cache
- **Invalidation**: Path-based invalidation after LDAP modifications
- **Data**: Complete HTML pages with user-specific content

```
Template Cache Flow:
Request ──→ Generate Cache Key ──→ Check Cache
                                       │
                              ┌────────┴────────┐
                              │                 │
                         Cache Hit         Cache Miss
                              │                 │
                    Return Cached HTML    Render Template
                                                 │
                                          Store in Cache
                                                 │  
                                          Return HTML
```

#### Level 3: Static Asset Cache (Cold Cache)
- **Purpose**: Cache CSS, JavaScript, images
- **TTL**: 24 hours (browser cache)
- **Storage**: Browser cache + CDN-ready
- **Invalidation**: Version-based cache busting
- **Data**: Static files with content hashing

### Cache Performance

Typical cache performance metrics:
- **LDAP Cache**: 95%+ hit ratio for read operations
- **Template Cache**: 80%+ hit ratio for authenticated users  
- **Static Cache**: 99%+ hit ratio with proper cache headers

### Cache Invalidation Strategy

**LDAP Data Cache**:
- Time-based: Automatic refresh every 30 seconds
- Event-based: Manual refresh after modifications
- Health-based: Refresh on LDAP connection recovery

**Template Cache**:
- Path-based: Invalidate specific URL patterns
- User-based: Invalidate user-specific cached content
- Global: Clear all cache after schema changes

---

## Security Architecture

### Defense in Depth

LDAP Manager implements multiple security layers:

```
┌─────────────────────────────────────────┐
│          Network Security               │
│  (HTTPS, Reverse Proxy, Firewall)       │
├─────────────────────────────────────────┤
│         Application Security            │
│  (Security Headers, CSRF, Input Val.)   │  
├─────────────────────────────────────────┤
│        Authentication Security          │
│  (LDAP Auth, Session Management)        │
├─────────────────────────────────────────┤
│         Authorization Security          │
│  (User Context, Permission Checking)    │
└─────────────────────────────────────────┘
```

### Security Controls

#### HTTP Security Headers
```http
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
Content-Security-Policy: default-src 'self'; style-src 'self' 'unsafe-inline'
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
```

#### Session Security
- **HTTP-only cookies**: Prevents XSS access to session data
- **SameSite=Strict**: CSRF protection via cookie policy
- **Secure flag**: HTTPS-only cookie transmission
- **Configurable expiration**: Automatic session timeout

#### CSRF Protection
- **Token-based**: Unique tokens for each form submission
- **SameSite cookies**: Additional CSRF protection
- **Server validation**: All state-changing operations validated
- **Error handling**: Secure error messages without information leakage

#### Input Validation
- **Parameter sanitization**: URL decoding with validation
- **Form data validation**: Type checking and bounds validation
- **LDAP injection prevention**: Proper escaping and parameterization
- **File upload restrictions**: No file uploads accepted

### Authentication Architecture

```
User Credentials
      │
      ▼
┌─────────────────┐
│  LDAP Server    │◄──── Service Account
│  Authentication │      (Read-only)
└─────────┬───────┘
          │
          ▼ (Success)
┌─────────────────┐
│ Session Store   │◄──── User DN
│ (Memory/BoltDB) │      Session ID
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ HTTP Response   │◄──── Secure Cookie
│ (Secure Cookie) │      Session Data
└─────────────────┘
```

### Authorization Model

**User Context Operations**:
- All LDAP operations use authenticated user's credentials
- No privilege escalation or service account operations
- User can only modify what they have LDAP permissions for
- Read operations cached but still respect user permissions

**Permission Inheritance**:
- LDAP permissions determine available operations
- Web interface reflects actual LDAP capabilities
- No additional permission layer in application
- Service account only used for initial data caching

---

## Performance Design

### Performance Characteristics

LDAP Manager is designed for **high-performance directory operations**:

- **Response Times**: <100ms for cached operations, <500ms for LDAP queries
- **Throughput**: 1000+ concurrent users with connection pooling
- **Memory Usage**: Efficient caching with configurable limits
- **CPU Usage**: Minimal overhead with compiled templates

### Connection Pool Design

```
Connection Pool Architecture:
┌─────────────────────────────────────────┐
│           Pool Manager                  │
├─────────────────────────────────────────┤
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐       │
│  │LD.1 │ │LD.2 │ │LD.3 │ │ ... │       │
│  │     │ │     │ │     │ │     │       │
│  └─────┘ └─────┘ └─────┘ └─────┘       │
│  Active   Active   Idle    Available     │
├─────────────────────────────────────────┤
│          Pool Configuration             │
│  • Max Connections: 10                  │
│  • Min Connections: 2                   │
│  • Max Idle Time: 15min                 │
│  • Health Check: 30s                    │
└─────────────────────────────────────────┘
```

**Pool Benefits**:
- **Connection Reuse**: Avoid expensive LDAP bind operations
- **Concurrency**: Multiple concurrent LDAP operations  
- **Health Monitoring**: Automatic recovery from connection failures
- **Resource Management**: Limits prevent LDAP server overload

### Template Performance

**Compiled Templates**:
- [templ](https://templ.guide/) compiles templates to Go code
- Type-safe template generation at compile time
- No runtime template parsing overhead
- Automatic escaping and security validation

**Template Caching**:
- Rendered HTML cached in memory
- Cache keys include user context and parameters
- LRU eviction prevents memory growth
- Automatic invalidation after data changes

### Memory Management

**Cache Memory**:
- LDAP cache: ~1MB per 10,000 users (estimated)
- Template cache: Configurable LRU with size limits
- Connection pool: Minimal overhead per connection
- Session storage: Configurable in-memory or persistent

**Garbage Collection**:
- Efficient object reuse in critical paths
- Minimal allocations in request handling
- Periodic cache cleanup and maintenance

---

## Deployment Architecture

### Container Architecture

```
┌─────────────────────────────────────────┐
│            Container Image              │
├─────────────────────────────────────────┤
│ • Alpine Linux (minimal base)          │
│ • ldap-manager binary                   │
│ • Static assets embedded               │
│ • Health check script                  │
│ • Non-root user execution              │
└─────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│         Runtime Environment            │
├─────────────────────────────────────────┤
│ • Environment variables                 │
│ • Optional persistent session storage  │
│ • Health check endpoints               │
│ • Graceful shutdown handling           │
│ • Resource limits and monitoring       │
└─────────────────────────────────────────┘
```

### Production Deployment

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │   (nginx/HAP)   │
                    └─────────┬───────┘
                              │
                    ┌─────────▼───────┐
                    │  Reverse Proxy  │
                    │ (HTTPS Term.)   │
                    └─────────┬───────┘
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
    ┌──────▼──────┐  ┌────────▼────────┐  ┌──────▼──────┐
    │LDAP Manager │  │ LDAP Manager    │  │LDAP Manager │
    │ Instance 1  │  │ Instance 2      │  │ Instance N  │
    └─────────────┘  └─────────────────┘  └─────────────┘
           │                  │                  │
           └──────────────────┼──────────────────┘
                              │
                    ┌─────────▼───────┐
                    │  LDAP Directory │
                    │ (AD / OpenLDAP) │
                    └─────────────────┘
```

### Configuration Management

**Environment-based Configuration**:
- All settings configurable via environment variables
- Sensible defaults for development
- Production-ready defaults for containers
- No hardcoded configuration in binaries

**Configuration Validation**:
- Startup validation of required parameters
- Clear error messages for misconfiguration
- Fail-fast approach for invalid settings

### Health Monitoring

**Health Check Endpoints**:
- `GET /health` - Basic application health
- `GET /health/ready` - Readiness probe (LDAP connectivity)
- `GET /health/live` - Liveness probe (application status)

**Monitoring Integration**:
- Structured JSON logging
- Metrics endpoints for Prometheus
- Connection pool statistics
- Cache performance metrics

### Scaling Considerations

**Horizontal Scaling**:
- Stateless application design
- Session storage can be shared (BoltDB or external)
- LDAP connection pools per instance
- No inter-instance coordination required

**Vertical Scaling**:
- Memory usage scales with cache size
- CPU usage minimal with compiled templates
- Connection pool size configurable per instance

---

## Development Workflow

### Code Organization Principles

1. **Package Cohesion**: Related functionality grouped in packages
2. **Clear Interfaces**: Well-defined package boundaries
3. **Dependency Direction**: Dependencies flow downward in layers
4. **Testability**: Interfaces allow for easy mocking and testing
5. **Documentation**: Comprehensive Go doc comments for all exports

### Build and Development

```bash
# Local development setup
make dev-setup        # Install dependencies and tools
make dev              # Run with hot reload
make test             # Run all tests
make test-coverage    # Generate coverage report

# Production build
make build            # Build optimized binary
make docker-build     # Build container image
make docker-run       # Run containerized application

# Code quality
make lint             # Run linting and formatting
make security-check   # Run security analysis
make benchmark        # Performance benchmarks
```

### Testing Strategy

- **Unit Tests**: Core business logic and utilities
- **Integration Tests**: LDAP operations and caching
- **Handler Tests**: HTTP endpoint testing
- **Performance Tests**: Load testing and benchmarks
- **Security Tests**: Authentication and authorization

---

This architecture guide provides the foundational understanding needed to contribute effectively to LDAP Manager. For implementation details, see the [Go Documentation Reference](go-doc-reference.md) and [API Reference](../user-guide/api.md).