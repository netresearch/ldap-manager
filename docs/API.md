# LDAP Manager API Documentation

## Overview

LDAP Manager provides a web-based interface for managing LDAP directory entries including users, groups, and computers. The application is built using Go Fiber framework with session-based authentication.

## Base Configuration

- **Host**: localhost
- **Port**: 3000
- **Base URL**: `http://localhost:3000`
- **Content-Type**: `text/html; charset=utf-8`

## Authentication

All protected endpoints require session-based authentication. Users must authenticate via the `/login` endpoint before accessing management features.

### Session Management

- **Cookie**: Session stored in HTTP-only, SameSite=Strict cookie
- **Storage**: Configurable (Memory or BBolt database)
- **Duration**: Configurable (default: 30 minutes)

## Endpoints

### Authentication Endpoints

#### `GET /login`

Display login form and handle authentication.

**Query Parameters:**

- `username` (string, optional): Username for authentication
- `password` (string, optional): Password for authentication

**Response:**

- **Success with credentials**: Redirect to `/` with session established
- **Invalid credentials**: Login form with error message
- **No credentials**: Empty login form

**Example:**

```bash
# Display login form
curl http://localhost:3000/login

# Authenticate user
curl "http://localhost:3000/login?username=john.doe&password=secret123"
```

#### `GET /logout`

Destroy user session and redirect to login.

**Response:**

- **Success**: Redirect to `/login`

### Protected Endpoints

#### `GET /`

Display dashboard with user information.

**Authentication**: Required
**Response**: User dashboard with authenticated user details

#### `GET /users`

List all users in the LDAP directory.

**Authentication**: Required
**Response**: HTML page with user listing

#### `GET /users/:userDN`

Display detailed information for a specific user.

**Parameters:**

- `userDN` (string): Distinguished Name of the user

**Authentication**: Required
**Response**: User detail page with edit capabilities

#### `POST /users/:userDN`

Modify user attributes.

**Parameters:**

- `userDN` (string): Distinguished Name of the user

**Authentication**: Required
**Body**: Form data with user attributes to modify
**Response**: User detail page with success/error messages

#### `GET /groups`

List all groups in the LDAP directory.

**Authentication**: Required
**Response**: HTML page with group listing

#### `GET /groups/:groupDN`

Display detailed information for a specific group.

**Parameters:**

- `groupDN` (string): Distinguished Name of the group

**Authentication**: Required
**Response**: Group detail page with edit capabilities

#### `POST /groups/:groupDN`

Modify group attributes.

**Parameters:**

- `groupDN` (string): Distinguished Name of the group

**Authentication**: Required
**Body**: Form data with group attributes to modify
**Response**: Group detail page with success/error messages

#### `GET /computers`

List all computers in the LDAP directory.

**Authentication**: Required
**Response**: HTML page with computer listing

#### `GET /computers/:computerDN`

Display detailed information for a specific computer.

**Parameters:**

- `computerDN` (string): Distinguished Name of the computer

**Authentication**: Required
**Response**: Computer detail page

### Static Assets

#### `GET /static/*`

Serve static assets (CSS, images, icons).

**Caching**: 24 hour max-age
**Compression**: Enabled with best speed level

### Error Handling

#### `404 Not Found`

Custom 404 page for unmatched routes.

#### `500 Internal Server Error`

Custom error page with error details (in debug mode).

## Request/Response Flow

### Authentication Flow

1. User accesses protected endpoint
2. Check for valid session
3. If no session: redirect to `/login`
4. If session exists: proceed with request
5. For login: validate credentials against LDAP
6. On success: create session and redirect to `/`

### Data Access Flow

1. Session validation
2. Extract user DN from session
3. Create LDAP client with user credentials
4. Query LDAP directory via cache manager
5. Render response with templ templates

## Error Codes

| Code | Description                            |
| ---- | -------------------------------------- |
| 200  | Success                                |
| 302  | Redirect (authentication, post-action) |
| 404  | Resource not found                     |
| 500  | Internal server error                  |

## Security Features

- Session-based authentication
- HTTP-only cookies
- SameSite=Strict cookie policy
- LDAP credential validation
- User-context LDAP operations
- Automatic session expiration
- HTTPS support (via reverse proxy)

## Rate Limiting

- Body limit: 4KB per request
- Session-based access control
- LDAP connection pooling via cache

## Dependencies

- Go Fiber v2 web framework
- simple-ldap-go for LDAP operations
- templ for type-safe HTML templates
- BBolt for optional session persistence
- Zerolog for structured logging
