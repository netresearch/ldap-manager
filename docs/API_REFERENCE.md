# API Reference

**Version:** 1.0.8
**Base URL:** `http://localhost:3000` (development) or configured domain
**Authentication:** Session-based (HTTP-only cookies)
**Content-Type:** `application/x-www-form-urlencoded` (forms), `application/json` (API responses)

---

## Table of Contents

- [Authentication](#authentication)
  - [POST /login](#post-login)
  - [GET /logout](#get-logout)
- [Health & Monitoring](#health--monitoring)
  - [GET /health](#get-health)
  - [GET /health/ready](#get-healthready)
  - [GET /health/live](#get-healthlive)
  - [GET /debug/cache](#get-debugcache)
  - [GET /debug/ldap-pool](#get-debugldap-pool)
- [Users](#users)
  - [GET /](#get-)
  - [GET /users](#get-users)
  - [GET /users/:userDN](#get-usersuserdn)
  - [POST /users/:userDN](#post-usersuserdn)
- [Groups](#groups)
  - [GET /groups](#get-groups)
  - [GET /groups/:groupDN](#get-groupsgroupdn)
  - [POST /groups/:groupDN](#post-groupsgroupdn)
- [Computers](#computers)
  - [GET /computers](#get-computers)
  - [GET /computers/:computerDN](#get-computerscomputerdn)
- [Error Responses](#error-responses)
- [Rate Limiting & Caching](#rate-limiting--caching)

---

## Authentication

### POST /login

Authenticate user and create session.

**URL:** `/login`
**Method:** `POST`
**Auth Required:** ❌ No
**CSRF Protection:** ✅ Yes

#### Request

**Form Data:**

```
username=jdoe
password=SecurePass123
csrf_token=<generated-token>
```

**CSRF Token:** Must be obtained from the login page form. Automatically included in rendered login form.

#### Success Response

**Code:** `302 Found`
**Headers:**

```
Location: /
Set-Cookie: session_id=...; HttpOnly; Secure; SameSite=Strict
```

#### Error Response

**Code:** `401 Unauthorized`
**Content:**

```json
{
  "error": "invalid_credentials",
  "message": "Invalid username or password"
}
```

**Code:** `400 Bad Request`
**Content:**

```json
{
  "error": "missing_fields",
  "message": "Username and password are required"
}
```

#### Notes

- Password must match LDAP directory entry
- Session duration configurable (default: 30 minutes)
- Failed login attempts logged for security monitoring
- LDAP connection validated before session creation

---

### GET /logout

Terminate user session.

**URL:** `/logout`
**Method:** `GET`
**Auth Required:** ✅ Yes
**CSRF Protection:** ❌ No (safe GET operation)

#### Success Response

**Code:** `302 Found`
**Headers:**

```
Location: /login
Set-Cookie: session_id=; Expires=Thu, 01 Jan 1970 00:00:00 GMT
```

#### Notes

- Destroys server-side session data
- Clears session cookie
- Redirects to login page
- Safe to call multiple times

---

## Health & Monitoring

### GET /health

Basic health check endpoint.

**URL:** `/health`
**Method:** `GET`
**Auth Required:** ❌ No
**Cache:** ❌ No

#### Success Response

**Code:** `200 OK`
**Content:**

```json
{
  "status": "healthy",
  "version": "1.0.8",
  "timestamp": "2025-09-30T12:34:56Z"
}
```

#### Error Response

**Code:** `503 Service Unavailable`
**Content:**

```json
{
  "status": "unhealthy",
  "error": "ldap_connection_failed"
}
```

#### Notes

- Use for load balancer health checks
- Fast response time (<10ms typically)
- Checks basic application liveness
- Does not validate LDAP connectivity

---

### GET /health/ready

Readiness probe with dependency checks.

**URL:** `/health/ready`
**Method:** `GET`
**Auth Required:** ❌ No
**Cache:** ❌ No

#### Success Response

**Code:** `200 OK`
**Content:**

```json
{
  "status": "ready",
  "checks": {
    "ldap": "ok",
    "cache": "ok",
    "session_store": "ok"
  },
  "timestamp": "2025-09-30T12:34:56Z"
}
```

#### Error Response

**Code:** `503 Service Unavailable`
**Content:**

```json
{
  "status": "not_ready",
  "checks": {
    "ldap": "failed",
    "cache": "ok",
    "session_store": "ok"
  },
  "error": "LDAP connection timeout"
}
```

#### Notes

- Use for Kubernetes readiness probes
- Validates all critical dependencies
- May have slower response time (up to 3s)
- Application not ready to serve traffic if fails

---

### GET /health/live

Liveness probe for restart detection.

**URL:** `/health/live`
**Method:** `GET`
**Auth Required:** ❌ No
**Cache:** ❌ No

#### Success Response

**Code:** `200 OK`
**Content:**

```json
{
  "status": "alive",
  "uptime_seconds": 3600
}
```

#### Notes

- Use for Kubernetes liveness probes
- Simple check - just verifies server responding
- Never fails unless server completely hung
- Triggers pod restart if fails repeatedly

---

### GET /debug/cache

Template cache statistics (authenticated endpoint for monitoring).

**URL:** `/debug/cache`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ❌ No

#### Success Response

**Code:** `200 OK`
**Content:**

```json
{
  "total_entries": 42,
  "size_bytes": 1048576,
  "hit_rate": 0.85,
  "hits": 1234,
  "misses": 217,
  "evictions": 5
}
```

#### Notes

- Requires authenticated session
- For performance monitoring and tuning
- Cache invalidated on POST operations
- Statistics reset on application restart

---

### GET /debug/ldap-pool

LDAP connection pool statistics.

**URL:** `/debug/ldap-pool`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ❌ No

#### Success Response

**Code:** `200 OK`
**Content:**

```json
{
  "active_connections": 3,
  "idle_connections": 2,
  "max_connections": 10,
  "total_acquired": 456,
  "total_released": 453,
  "wait_count": 0,
  "wait_duration_ms": 0
}
```

#### Notes

- Requires authenticated session
- For LDAP performance monitoring
- Helps diagnose connection pool exhaustion
- Use for capacity planning

---

## Users

### GET /

Dashboard/home page (redirects to users list).

**URL:** `/`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes (template cache)

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`
**Headers:**

```
X-Cache: HIT | MISS
```

#### Notes

- Redirects to `/users` or renders dashboard
- Template cached for performance
- Cache invalidated on any POST operation

---

### GET /users

List all LDAP users.

**URL:** `/users`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes (template + LDAP cache)

#### Query Parameters

| Parameter | Type    | Required | Description                              |
| --------- | ------- | -------- | ---------------------------------------- |
| `search`  | string  | ❌ No    | Filter users by name/email               |
| `page`    | integer | ❌ No    | Pagination page (default: 1)             |
| `limit`   | integer | ❌ No    | Results per page (default: 50, max: 100) |

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`
**Headers:**

```
X-Cache: HIT | MISS
```

**Rendered HTML with user table**

#### Notes

- Data cached for 30 seconds (LDAP cache)
- Template cached until POST operation
- Supports search filtering
- Sorted alphabetically by username

---

### GET /users/:userDN

Display single user details.

**URL:** `/users/:userDN`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes

#### URL Parameters

| Parameter | Description                         | Example                                            |
| --------- | ----------------------------------- | -------------------------------------------------- |
| `userDN`  | URL-encoded user Distinguished Name | `cn%3Djdoe%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom` |

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`

**Rendered HTML with:**

- User attributes (cn, mail, telephoneNumber, etc.)
- Group memberships
- Modification form with CSRF token

#### Error Response

**Code:** `404 Not Found`
**Content:**

```json
{
  "error": "user_not_found",
  "message": "User does not exist"
}
```

#### Notes

- UserDN must be URL-encoded
- Shows all user attributes from LDAP
- Includes CSRF-protected modification form

---

### POST /users/:userDN

Modify user attributes.

**URL:** `/users/:userDN`
**Method:** `POST`
**Auth Required:** ✅ Yes
**CSRF Protection:** ✅ Yes
**Cache:** ❌ No (invalidates cache)

#### URL Parameters

| Parameter | Description                         |
| --------- | ----------------------------------- |
| `userDN`  | URL-encoded user Distinguished Name |

#### Request

**Form Data:**

```
csrf_token=<token>
mail=newemail@example.com
telephoneNumber=+1-555-0123
description=Updated description
```

#### Success Response

**Code:** `302 Found`
**Headers:**

```
Location: /users/:userDN
Set-Cookie: flash_message=User updated successfully
```

#### Error Response

**Code:** `400 Bad Request`
**Content:**

```json
{
  "error": "validation_failed",
  "message": "Invalid email format",
  "field": "mail"
}
```

**Code:** `403 Forbidden`
**Content:**

```json
{
  "error": "insufficient_permissions",
  "message": "You don't have permission to modify this user"
}
```

#### Notes

- Requires valid CSRF token
- Invalidates template cache for `/users` endpoints
- LDAP modifications use authenticated user's credentials
- Validation performed before LDAP operation

---

## Groups

### GET /groups

List all LDAP groups.

**URL:** `/groups`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`

**Rendered HTML with:**

- Group list table
- Member counts
- Links to group details

#### Notes

- Cached for 30 seconds (LDAP cache)
- Template cached until POST operation
- Sorted alphabetically

---

### GET /groups/:groupDN

Display single group details.

**URL:** `/groups/:groupDN`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes

#### URL Parameters

| Parameter | Description                          |
| --------- | ------------------------------------ |
| `groupDN` | URL-encoded group Distinguished Name |

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`

**Rendered HTML with:**

- Group attributes
- Member list
- Modification form

---

### POST /groups/:groupDN

Modify group (add/remove members).

**URL:** `/groups/:groupDN`
**Method:** `POST`
**Auth Required:** ✅ Yes
**CSRF Protection:** ✅ Yes

#### Request

**Form Data:**

```
csrf_token=<token>
action=add_member | remove_member
member_dn=cn=jdoe,ou=users,dc=example,dc=com
```

#### Success Response

**Code:** `302 Found`
**Headers:**

```
Location: /groups/:groupDN
Set-Cookie: flash_message=Member added successfully
```

#### Notes

- Supports add_member and remove_member actions
- Invalidates cache
- Validates member DN exists before modification

---

## Computers

### GET /computers

List all computer accounts (Active Directory).

**URL:** `/computers`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`

**Rendered HTML with computer account table**

#### Notes

- Active Directory specific endpoint
- May return empty list for standard LDAP
- Cached for performance

---

### GET /computers/:computerDN

Display computer account details.

**URL:** `/computers/:computerDN`
**Method:** `GET`
**Auth Required:** ✅ Yes
**Cache:** ✅ Yes

#### Success Response

**Code:** `200 OK`
**Content-Type:** `text/html`

**Rendered HTML with computer attributes**

---

## Error Responses

### Standard Error Format

All error responses follow this structure:

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "field": "fieldname" // Optional, for validation errors
}
```

### Common Error Codes

| Code                  | HTTP Status | Description                        |
| --------------------- | ----------- | ---------------------------------- |
| `invalid_credentials` | 401         | Authentication failed              |
| `session_expired`     | 401         | Session timeout, re-login required |
| `unauthorized`        | 403         | Insufficient permissions           |
| `not_found`           | 404         | Resource doesn't exist             |
| `validation_failed`   | 400         | Input validation error             |
| `csrf_invalid`        | 403         | CSRF token missing or invalid      |
| `ldap_error`          | 500         | LDAP operation failed              |
| `internal_error`      | 500         | Unexpected server error            |

---

## Rate Limiting & Caching

### Template Cache

- **TTL:** Until POST operation (invalidated)
- **Scope:** GET requests only
- **Header:** `X-Cache: HIT` or `X-Cache: MISS`
- **Invalidation:** All POST operations clear cache

### LDAP Cache

- **TTL:** 30 seconds
- **Scope:** Directory data (users, groups, computers)
- **Refresh:** Background automatic refresh
- **Behavior:** Stale data acceptable for performance

### Session Cache

- **TTL:** Configurable (default 30 minutes)
- **Storage:** Memory or BBolt database
- **Security:** HTTP-only, Secure, SameSite=Strict cookies

---

## Security Considerations

### Authentication

- Session-based with HTTP-only cookies
- CSRF protection on all state-changing operations
- Session timeout configurable
- No API key or token-based auth (by design)

### Input Validation

- All user inputs validated and sanitized
- LDAP injection prevention (special char escaping)
- Email format validation
- DN format validation

### HTTPS Requirement

- **Production:** HTTPS required (Secure cookie flag)
- **Development:** HTTP acceptable (Secure flag disabled)
- **Recommendation:** Always use HTTPS with valid certificates

---

## Code Examples

### cURL Examples

**Login:**

```bash
# Get CSRF token from login page first
curl -c cookies.txt http://localhost:3000/login

# Login with credentials
curl -b cookies.txt -X POST http://localhost:3000/login \
  -d "username=admin" \
  -d "password=secret" \
  -d "csrf_token=<token>"
```

**List Users:**

```bash
curl -b cookies.txt http://localhost:3000/users
```

**Modify User:**

```bash
curl -b cookies.txt -X POST \
  "http://localhost:3000/users/cn%3Djdoe%2Cou%3Dusers%2Cdc%3Dexample%2Cdc%3Dcom" \
  -d "csrf_token=<token>" \
  -d "mail=newemail@example.com"
```

---

## Related Documentation

- **[User Guide](user-guide/api.md)** - High-level API usage
- **[Architecture](development/architecture.md)** - Technical design
- **[Security Configuration](operations/security-configuration.md)** - Hardening guide
- **[Web AGENTS.md](../internal/web/AGENTS.md)** - Handler implementation patterns

---

_Last Updated: 2025-09-30 | Version: 1.0.8_
