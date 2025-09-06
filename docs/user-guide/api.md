# API Reference

Complete documentation for LDAP Manager's web API endpoints and usage patterns.

## API Overview

LDAP Manager provides a web-based interface for managing LDAP directory entries. The API follows RESTful conventions and uses session-based authentication with HTML responses for browser interaction.

### Base Configuration

- **Protocol**: HTTP/HTTPS
- **Default Port**: 3000
- **Content-Type**: `text/html; charset=utf-8`
- **Authentication**: Session-based with HTTP-only cookies

### API Characteristics

- **Session-based Authentication**: All protected endpoints require valid session
- **HTML Responses**: Designed for browser interaction, not JSON API
- **CSRF Protection**: SameSite=Strict cookie policy
- **Caching**: Static assets cached with 24-hour max-age
- **Compression**: Gzip compression enabled

## Authentication API

### Login Endpoint

#### `GET /login`

Display login form or authenticate user credentials.

**Parameters:**

| Parameter  | Type   | Required | Description                 |
| ---------- | ------ | -------- | --------------------------- |
| `username` | string | No       | Username for authentication |
| `password` | string | No       | Password for authentication |

**Response Codes:**

| Code | Description                                      |
| ---- | ------------------------------------------------ |
| 200  | Login form displayed or authentication failed    |
| 302  | Authentication successful, redirect to dashboard |

**Examples:**

```bash
# Display login form
curl -i http://localhost:3000/login

# Authenticate user (URL parameters)
curl -i "http://localhost:3000/login?username=john.doe&password=secret123"

# Authenticate user (POST form data)
curl -i -X POST \
  -d "username=john.doe&password=secret123" \
  http://localhost:3000/login
```

**Success Response:**

```
HTTP/1.1 302 Found
Location: /
Set-Cookie: session=abc123...; HttpOnly; SameSite=Strict; Path=/
```

**Authentication Flow:**

1. User submits credentials via GET parameters or POST form
2. Server validates against LDAP directory
3. On success: Create session, set cookie, redirect to `/`
4. On failure: Display login form with error message

### Logout Endpoint

#### `GET /logout`

Destroy user session and redirect to login page.

**Response:**

- **302 Found**: Redirect to `/login` with session cookie cleared

**Example:**

```bash
curl -i -b "session=abc123..." http://localhost:3000/logout
```

**Response:**

```
HTTP/1.1 302 Found
Location: /login
Set-Cookie: session=; HttpOnly; SameSite=Strict; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT
```

## Protected Endpoints

All endpoints in this section require valid session authentication. Requests without valid sessions are redirected to `/login`.

### Dashboard

#### `GET /`

Display user dashboard with authenticated user information.

**Authentication:** Required

**Response:** HTML page with user dashboard containing:

- Authenticated user details
- Navigation to user/group/computer management
- Session information

**Example:**

```bash
curl -i -b "session=abc123..." http://localhost:3000/
```

### User Management

#### `GET /users`

List all users in the LDAP directory.

**Authentication:** Required

**Response:** HTML page with paginated user listing including:

- User display names
- Account names (sAMAccountName for AD)
- Email addresses
- Account status
- Links to individual user detail pages

**Features:**

- Cached data with 30-second refresh
- Sorted by display name
- Responsive layout

#### `GET /users/:userDN`

Display detailed information for a specific user.

**Parameters:**

| Parameter | Type   | Required | Description                                |
| --------- | ------ | -------- | ------------------------------------------ |
| `userDN`  | string | Yes      | URL-encoded Distinguished Name of the user |

**Authentication:** Required

**Response:** HTML page with user details and edit form containing:

- All LDAP attributes
- Group memberships
- Account information
- Editable form fields

**Example:**

```bash
# Note: DN must be URL-encoded
USER_DN="CN=John%20Doe,OU=Users,DC=example,DC=com"
curl -i -b "session=abc123..." "http://localhost:3000/users/$USER_DN"
```

#### `POST /users/:userDN`

Modify user attributes in LDAP directory.

**Parameters:**

| Parameter | Type   | Required | Description                                |
| --------- | ------ | -------- | ------------------------------------------ |
| `userDN`  | string | Yes      | URL-encoded Distinguished Name of the user |

**Authentication:** Required

**Form Data:** Key-value pairs of LDAP attributes to modify

**Response Codes:**

| Code | Description                                   |
| ---- | --------------------------------------------- |
| 200  | User updated successfully or validation error |
| 302  | Redirect after successful update              |

**Example:**

```bash
USER_DN="CN=John%20Doe,OU=Users,DC=example,DC=com"
curl -i -X POST \
  -b "session=abc123..." \
  -d "givenName=Jonathan&sn=Doe&mail=jonathan.doe@example.com" \
  "http://localhost:3000/users/$USER_DN"
```

**Supported Attributes** (varies by LDAP schema):

- `givenName` - First name
- `sn` - Last name
- `mail` - Email address
- `telephoneNumber` - Phone number
- `description` - User description
- Standard LDAP and Active Directory attributes

### Group Management

#### `GET /groups`

List all groups in the LDAP directory.

**Authentication:** Required

**Response:** HTML page with group listing including:

- Group names
- Group types (security/distribution for AD)
- Member counts
- Group descriptions
- Links to group detail pages

#### `GET /groups/:groupDN`

Display detailed information for a specific group.

**Parameters:**

| Parameter | Type   | Required | Description                                 |
| --------- | ------ | -------- | ------------------------------------------- |
| `groupDN` | string | Yes      | URL-encoded Distinguished Name of the group |

**Authentication:** Required

**Response:** HTML page with group details including:

- Group attributes
- Member list with names and types
- Group membership management interface

**Example:**

```bash
GROUP_DN="CN=IT%20Department,OU=Groups,DC=example,DC=com"
curl -i -b "session=abc123..." "http://localhost:3000/groups/$GROUP_DN"
```

#### `POST /groups/:groupDN`

Modify group attributes and membership.

**Parameters:**

| Parameter | Type   | Required | Description                                 |
| --------- | ------ | -------- | ------------------------------------------- |
| `groupDN` | string | Yes      | URL-encoded Distinguished Name of the group |

**Authentication:** Required

**Form Data:** Group attributes and membership changes

**Example:**

```bash
GROUP_DN="CN=IT%20Department,OU=Groups,DC=example,DC=com"
curl -i -X POST \
  -b "session=abc123..." \
  -d "description=IT Department Staff&member=CN=John Doe,OU=Users,DC=example,DC=com" \
  "http://localhost:3000/groups/$GROUP_DN"
```

### Computer Management

#### `GET /computers`

List all computer accounts in the LDAP directory.

**Authentication:** Required

**Response:** HTML page with computer listing including:

- Computer names
- Operating system information
- Last logon timestamps
- Account status
- Links to computer detail pages

#### `GET /computers/:computerDN`

Display detailed information for a specific computer.

**Parameters:**

| Parameter    | Type   | Required | Description                                    |
| ------------ | ------ | -------- | ---------------------------------------------- |
| `computerDN` | string | Yes      | URL-encoded Distinguished Name of the computer |

**Authentication:** Required

**Response:** HTML page with computer details including:

- Computer attributes
- System information
- Network details
- Service information

**Example:**

```bash
COMPUTER_DN="CN=WORKSTATION01,OU=Computers,DC=example,DC=com"
curl -i -b "session=abc123..." "http://localhost:3000/computers/$COMPUTER_DN"
```

## Static Assets

### Static Content Endpoint

#### `GET /static/*`

Serve static assets including CSS, JavaScript, images, and icons.

**Caching:** 24-hour max-age header
**Compression:** Enabled with best speed level
**Content Types:** Automatic detection based on file extension

**Examples:**

```bash
# CSS files
curl -i http://localhost:3000/static/styles.css

# Images
curl -i http://localhost:3000/static/logo.webp

# Favicon
curl -i http://localhost:3000/static/favicon.ico
```

## Error Handling

### HTTP Error Responses

#### 404 Not Found

Custom 404 page for unmatched routes.

**Response:** HTML error page with navigation back to dashboard

#### 500 Internal Server Error

Custom error page with optional error details.

**Development Mode:** Shows detailed error information
**Production Mode:** Shows generic error message

**Example Error Response:**

```html
<!DOCTYPE html>
<html>
  <head>
    <title>Error - LDAP Manager</title>
  </head>
  <body>
    <h1>Internal Server Error</h1>
    <p>An error occurred while processing your request.</p>
    <!-- Error details shown only in debug mode -->
  </body>
</html>
```

## Request/Response Flow

### Authentication Flow

1. **Initial Request**: User accesses protected endpoint
2. **Session Check**: Server validates session cookie
3. **Redirect**: If no valid session, redirect to `/login`
4. **Login**: User submits credentials
5. **LDAP Validation**: Server validates against LDAP directory
6. **Session Creation**: On success, create session and set cookie
7. **Access Granted**: Redirect to originally requested resource

### Data Access Pattern

1. **Session Validation**: Verify user session
2. **User Context**: Extract user DN from session
3. **LDAP Client**: Create LDAP connection with user credentials
4. **Cache Check**: Check cache for requested data
5. **Directory Query**: Query LDAP if cache miss or expired
6. **Template Rendering**: Render HTML response using templ templates
7. **Response**: Return HTML page to client

## Security Features

### Authentication Security

- **Session-based Authentication**: HTTP-only, secure cookies
- **LDAP Credential Validation**: Direct validation against directory
- **User Context Operations**: All LDAP operations use authenticated user's credentials
- **Automatic Session Expiration**: Configurable timeout periods

### Request Security

- **SameSite Cookies**: CSRF protection with SameSite=Strict
- **HTTP-only Cookies**: XSS protection
- **Request Size Limits**: 4KB body limit per request
- **Connection Limits**: LDAP connection pooling and reuse

### HTTPS Support

While LDAP Manager doesn't handle TLS directly, it's designed to work behind reverse proxies:

```nginx
# Nginx HTTPS termination
location / {
    proxy_pass http://localhost:3000;
    proxy_set_header X-Forwarded-Proto https;
}
```

## Rate Limiting and Performance

### Built-in Limits

- **Body Size**: 4KB maximum request body
- **Session Management**: Configurable session storage and timeouts
- **LDAP Connection Pooling**: Automatic connection reuse
- **Caching**: 30-second LDAP data cache with automatic refresh

### Performance Monitoring

Enable debug logging to monitor performance:

```bash
LOG_LEVEL=debug ./ldap-manager
```

Monitor logs for:

- LDAP query execution times
- Cache hit/miss ratios
- Session creation/destruction
- Error rates and patterns

## Integration Examples

### Browser JavaScript

```javascript
// Form submission example
document.getElementById("userForm").addEventListener("submit", function (e) {
  e.preventDefault();

  const formData = new FormData(this);
  const userDN = encodeURIComponent(this.dataset.userDn);

  fetch(`/users/${userDN}`, {
    method: "POST",
    body: formData,
    credentials: "same-origin" // Include session cookie
  })
    .then((response) => {
      if (response.redirected) {
        window.location.href = response.url;
      }
      return response.text();
    })
    .then((html) => {
      // Update page content
      document.body.innerHTML = html;
    });
});
```

### Health Checks

```bash
#!/bin/bash
# Health check script
HEALTH_URL="http://localhost:3000/"
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$HEALTH_URL")

if [ "$RESPONSE" = "200" ] || [ "$RESPONSE" = "302" ]; then
    echo "LDAP Manager is healthy"
    exit 0
else
    echo "LDAP Manager health check failed: HTTP $RESPONSE"
    exit 1
fi
```

### Automated Testing

```bash
# Login and session test
SESSION_COOKIE=$(curl -s -c - "http://localhost:3000/login?username=testuser&password=testpass" | grep session | awk '{print $7}')

# Test protected endpoint
curl -s -b "session=$SESSION_COOKIE" "http://localhost:3000/users" | grep -q "Users" && echo "API test passed"
```

## Troubleshooting API Issues

### Common Problems

**Session Not Persisting:**

- Check cookie settings in browser
- Verify session configuration
- Check for clock synchronization issues

**LDAP Queries Failing:**

- Verify service account permissions
- Check LDAP server connectivity
- Review Base DN configuration

**Slow Response Times:**

- Monitor LDAP server performance
- Check network latency
- Review cache hit rates in debug logs

**Authentication Failures:**

- Test LDAP credentials manually
- Verify user account status
- Check for account lockouts

For detailed configuration and deployment information, see the [Configuration Reference](configuration.md) and [Installation Guide](installation.md).
