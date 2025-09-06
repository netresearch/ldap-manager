# Contributing Guidelines

Welcome to LDAP Manager! This guide covers everything you need to know about contributing to the project.

## Getting Started

### Prerequisites

Before contributing, ensure you have:

1. **Development Environment**: Set up per [Development Setup Guide](setup.md)
2. **LDAP Test Environment**: Access to a test LDAP server
3. **Git Configuration**: Configured with your name and email
4. **GitHub Account**: For creating pull requests

### First Contribution Checklist

- [ ] Fork the repository on GitHub
- [ ] Clone your fork locally
- [ ] Set up development environment (`make setup`)
- [ ] Install pre-commit hooks (`make setup-hooks`)
- [ ] Create a feature branch
- [ ] Make your changes
- [ ] Run quality checks (`make check`)
- [ ] Submit a pull request

## Code Standards

### Go Code Style

We follow standard Go conventions plus additional project-specific standards:

#### Formatting

```bash
# Auto-format code (run before committing)
make fix

# This applies:
# - gofumpt (stricter than gofmt)
# - goimports (import organization)
# - golangci-lint fixes
```

#### Naming Conventions

**Functions and Methods:**

```go
// Good: Clear, descriptive names
func AuthenticateUser(username, password string) (*User, error)
func LoadGroupMembers(groupDN string) ([]User, error)

// Avoid: Abbreviated or unclear names
func AuthUser(u, p string) (*User, error)
func LoadMembers(dn string) ([]User, error)
```

**Variables:**

```go
// Good: Clear context
userDN := "CN=John Doe,OU=Users,DC=example,DC=com"
ldapClient := cache.GetClient()

// Good: Standard abbreviations
i, j, k := 0, 1, 2  // Loop counters
ctx := context.Background()

// Avoid: Non-standard abbreviations
dn := "CN=John Doe,OU=Users,DC=example,DC=com"  // Use userDN
c := cache.GetClient()  // Use ldapClient
```

**Constants:**

```go
// Good: Descriptive constants
const (
    DefaultSessionDuration = 30 * time.Minute
    CacheRefreshInterval   = 30 * time.Second
    MaxBodySize           = 4 << 10  // 4KB
)
```

#### Error Handling

**Wrap Errors with Context:**

```go
// Good: Contextual error wrapping
func loadUser(dn string) (*User, error) {
    user, err := ldap.GetUser(dn)
    if err != nil {
        return nil, fmt.Errorf("failed to load user %s: %w", dn, err)
    }
    return user, nil
}

// Avoid: Generic error messages
func loadUser(dn string) (*User, error) {
    user, err := ldap.GetUser(dn)
    if err != nil {
        return nil, err  // No context
    }
    return user, nil
}
```

**Handle Errors at Appropriate Level:**

```go
// Good: Handle errors where you can take action
func (h *Handler) HandleUserUpdate(c *fiber.Ctx) error {
    user, err := h.loadUser(userDN)
    if err != nil {
        log.Error().Err(err).Str("userDN", userDN).Msg("failed to load user")
        return c.Status(500).Render("error", fiber.Map{"error": "User not found"})
    }
    // ... continue processing
}
```

#### Function Complexity

Keep functions simple and focused:

```go
// Good: Single responsibility, low complexity
func authenticateUser(username, password string) (*User, error) {
    if username == "" || password == "" {
        return nil, errors.New("username and password required")
    }

    conn, err := ldap.Dial("tcp", ldapServer)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to LDAP: %w", err)
    }
    defer conn.Close()

    err = conn.Bind(username, password)
    if err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    return loadUserDetails(conn, username)
}

// Extract complex logic into helper functions
func loadUserDetails(conn *ldap.Conn, username string) (*User, error) {
    // Implementation here
}
```

### Frontend Standards

#### HTML Templates (templ)

```html
<!-- Good: Semantic, accessible HTML -->
templ UserForm(user *User) {
    <form class="space-y-4" method="post" action={ templ.SafeURL("/users/" + user.DN) }>
        <div class="form-group">
            <label for="givenName" class="block text-sm font-medium text-gray-700">
                First Name
            </label>
            <input
                type="text"
                id="givenName"
                name="givenName"
                value={ user.GivenName }
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm"
                required
            />
        </div>

        <button
            type="submit"
            class="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"
        >
            Update User
        </button>
    </form>
}
```

#### CSS/TailwindCSS

```css
/* Good: Consistent, utility-first approach */
.form-group {
  @apply space-y-2;
}

.btn-primary {
  @apply rounded bg-blue-500 px-4 py-2 font-bold text-white transition-colors hover:bg-blue-700;
}

/* Component-specific styles when needed */
.user-card {
  @apply rounded-lg bg-white p-6 shadow-md;
}

.user-card:hover {
  @apply scale-105 transform shadow-lg transition-all duration-200;
}
```

## Testing Requirements

### Test Coverage

- **Minimum Coverage**: 80% (enforced by CI)
- **New Code**: Should have >90% coverage
- **Critical Paths**: Authentication, LDAP operations must be 100% covered

### Test Categories

#### Unit Tests

Test individual functions in isolation:

```go
func TestAuthenticateUser(t *testing.T) {
    tests := []struct {
        name     string
        username string
        password string
        wantErr  bool
    }{
        {"valid credentials", "testuser", "testpass", false},
        {"empty username", "", "testpass", true},
        {"empty password", "testuser", "", true},
        {"invalid credentials", "testuser", "wrongpass", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := authenticateUser(tt.username, tt.password)
            if (err != nil) != tt.wantErr {
                t.Errorf("authenticateUser() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### Integration Tests

Test component interactions:

```go
func TestUserModificationFlow(t *testing.T) {
    // Setup test LDAP server
    testServer := setupTestLDAP(t)
    defer testServer.Close()

    // Create test app
    app := setupTestApp(t, testServer.URL)

    // Test the complete flow
    req := httptest.NewRequest("POST", "/users/"+testUserDN, strings.NewReader("givenName=UpdatedName"))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    resp, err := app.Test(req)
    require.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)

    // Verify changes persisted
    updatedUser := getTestUser(t, testServer)
    assert.Equal(t, "UpdatedName", updatedUser.GivenName)
}
```

#### Benchmark Tests

Test performance characteristics:

```go
func BenchmarkUserSearch(b *testing.B) {
    cache := setupTestCache(b)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := cache.SearchUsers("testuser")
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkConcurrentAccess(b *testing.B) {
    cache := setupTestCache(b)

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := cache.GetUsers()
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

### Testing Best Practices

**Use Table Tests:**

```go
func TestValidateInput(t *testing.T) {
    tests := []struct {
        name  string
        input string
        valid bool
    }{
        {"valid email", "user@example.com", true},
        {"invalid email", "not-an-email", false},
        {"empty input", "", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

**Use testify for Assertions:**

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestUserCreation(t *testing.T) {
    user, err := CreateUser("testuser")
    require.NoError(t, err)  // Fail immediately on error
    assert.Equal(t, "testuser", user.Username)
    assert.NotEmpty(t, user.ID)
}
```

**Mock External Dependencies:**

```go
type mockLDAPClient struct {
    users map[string]*User
}

func (m *mockLDAPClient) GetUser(dn string) (*User, error) {
    user, exists := m.users[dn]
    if !exists {
        return nil, errors.New("user not found")
    }
    return user, nil
}
```

## Commit Message Standards

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

- **feat**: New feature for users
- **fix**: Bug fix for users
- **docs**: Documentation changes
- **style**: Code formatting (no logic changes)
- **refactor**: Code restructuring (no behavior change)
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **chore**: Maintenance tasks, dependency updates

### Examples

```bash
# Simple feature
feat: add user search functionality

# Bug fix with scope
fix(auth): prevent session hijacking vulnerability

# Breaking change
feat!: migrate to new LDAP library

BREAKING CHANGE: The LDAP configuration format has changed.
Update your .env files to use the new LDAP_SERVER format.

# Multiple paragraphs
feat(ui): add dark mode support

Dark mode can be toggled using the theme switcher in the header.
The preference is saved in localStorage and persists across sessions.

Closes #123
```

## Pull Request Process

### Before Creating PR

1. **Ensure Quality**:

   ```bash
   make check  # Run all linting and tests
   ```

2. **Update Documentation**: If your changes affect user-facing features
3. **Add Tests**: For new functionality
4. **Update CHANGELOG**: For significant changes

### PR Title and Description

**Title Format**: Same as commit messages

```
feat(auth): add two-factor authentication support
```

**Description Template**:

```markdown
## Summary

Brief description of changes and motivation.

## Changes Made

- List of specific changes
- Another change
- Breaking changes (if any)

## Testing

- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Documentation

- [ ] API documentation updated
- [ ] User guide updated (if applicable)
- [ ] Code comments added for complex logic

## Screenshots (if applicable)

Before/after screenshots for UI changes.

Closes #issue_number
```

### Review Process

1. **Automated Checks**: All CI checks must pass
2. **Code Review**: At least one maintainer approval required
3. **Testing**: Manual testing for significant changes
4. **Documentation**: Verify documentation is complete and accurate

### Merge Requirements

- [ ] All CI checks pass
- [ ] Code review approval from maintainer
- [ ] No merge conflicts
- [ ] Branch is up-to-date with main
- [ ] Tests cover new functionality
- [ ] Documentation updated (if needed)

## Issue Reporting

### Bug Reports

Use the bug report template:

```markdown
**Describe the Bug**
Clear description of the bug.

**To Reproduce**
Steps to reproduce:

1. Go to '...'
2. Click on '...'
3. See error

**Expected Behavior**
What should have happened.

**Environment**

- OS: [e.g., Ubuntu 20.04]
- Go Version: [e.g., 1.23.1]
- LDAP Manager Version: [e.g., v1.0.0]
- LDAP Server: [e.g., Active Directory 2019]

**Additional Context**

- Configuration details (sanitized)
- Log output
- Screenshots (if applicable)
```

### Feature Requests

```markdown
**Feature Request**
Clear description of the desired feature.

**Motivation**
Why is this feature needed? What problem does it solve?

**Proposed Solution**
Detailed description of how the feature should work.

**Alternatives Considered**
Other approaches considered.

**Additional Context**

- Mockups or diagrams
- Similar features in other tools
- Impact on existing functionality
```

## Development Workflow

### Branch Naming

- **Feature branches**: `feat/descriptive-name`
- **Bug fixes**: `fix/issue-description`
- **Documentation**: `docs/topic-name`
- **Refactoring**: `refactor/component-name`

### Development Cycle

1. **Create Issue**: Describe the work to be done
2. **Create Branch**: From main branch
3. **Develop**: Make changes with frequent commits
4. **Test**: Run local tests and quality checks
5. **Document**: Update relevant documentation
6. **Submit PR**: Create pull request for review
7. **Address Feedback**: Make requested changes
8. **Merge**: Maintainer merges after approval

### Code Review Guidelines

#### As a Reviewer

**Focus Areas:**

- Code correctness and logic
- Test coverage and quality
- Performance implications
- Security considerations
- Documentation completeness

**Review Checklist:**

- [ ] Code follows project style guidelines
- [ ] Tests cover new functionality
- [ ] No obvious security vulnerabilities
- [ ] Documentation is clear and accurate
- [ ] Breaking changes are documented

**Providing Feedback:**

````markdown
# Good feedback

Consider using a more descriptive variable name here.
`userDN` would be clearer than `dn` in this context.

# Actionable suggestion

```go
// Instead of:
dn := getUserDN()

// Consider:
userDN := getUserDN()
```
````

# Positive reinforcement

Nice use of the builder pattern here! This makes the code much more readable.

```

#### As a Contributor

**Responding to Feedback:**
- Address all comments constructively
- Ask for clarification if feedback is unclear
- Make requested changes promptly
- Thank reviewers for their time and suggestions

**Common Review Issues:**
- Missing error handling
- Insufficient test coverage
- Unclear variable names
- Missing documentation
- Performance concerns

## Release Process

### Version Numbering

We use [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Checklist

1. **Update Version**: Update version in relevant files
2. **Update CHANGELOG**: Document all changes since last release
3. **Create Release PR**: Merge all changes to main
4. **Tag Release**: Create Git tag with version number
5. **Build Assets**: Generate release binaries
6. **Update Documentation**: Ensure docs reflect new version

## Getting Help

### Documentation

- **User Guide**: [Installation](../user-guide/installation.md), [Configuration](../user-guide/configuration.md), [API](../user-guide/api.md)
- **Development**: [Setup Guide](setup.md), [Architecture](architecture.md)
- **Operations**: [Deployment](../operations/deployment.md)

### Community Support

- **GitHub Issues**: Technical questions and bug reports
- **Discussions**: Feature requests and general questions
- **Code Review**: Ask for specific feedback in PR comments

### Project Maintainers

Current maintainers are listed in the project README. Feel free to tag them in issues or PRs when you need guidance.

## Recognition

We value all contributions to LDAP Manager:

- **Contributors**: Listed in project README
- **Significant Contributions**: Mentioned in release notes
- **Bug Reports**: Credited in fix commits
- **Documentation**: Recognized for improving project accessibility

Thank you for contributing to LDAP Manager! 🎉
```
