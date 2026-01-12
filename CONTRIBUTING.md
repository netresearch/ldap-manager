# Contributing to LDAP Manager

Thank you for your interest in contributing! This document provides guidelines for contributing to this project.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Community](#community)

---

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors, regardless of background, experience level, gender identity, sexual orientation, disability, personal appearance, race, ethnicity, age, religion, or nationality.

### Expected Behavior

- Use welcoming and inclusive language
- Be respectful of differing viewpoints and experiences
- Gracefully accept constructive criticism
- Focus on what is best for the community
- Show empathy towards other community members

### Unacceptable Behavior

- Harassment, discrimination, or intimidation
- Trolling, insulting/derogatory comments, and personal attacks
- Public or private harassment
- Publishing others' private information without permission
- Other conduct which could reasonably be considered inappropriate

### Enforcement

Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by opening an issue or contacting the project maintainers. All complaints will be reviewed and investigated promptly and fairly.

---

## Getting Started

### Prerequisites

- **Go** 1.25 or higher
- **Node.js** 24.x or higher
- **pnpm** 10.x or higher (install via `corepack enable`)
- **Git** for version control
- **Docker** (for integration tests and local LDAP)
- **templ** CLI (install via `go install github.com/a-h/templ/cmd/templ@latest`)

### Initial Setup

1. **Fork and clone the repository**:

   ```bash
   git clone https://github.com/YOUR-USERNAME/ldap-manager.git
   cd ldap-manager
   ```

2. **Install dependencies**:

   ```bash
   pnpm install
   go mod download
   ```

3. **Generate templates**:

   ```bash
   templ generate
   ```

4. **Build assets**:

   ```bash
   pnpm build:css
   ```

5. **Configure environment**:

   ```bash
   cp .env.local.example .env.local
   # Edit .env.local with your LDAP settings
   ```

6. **Start development server**:

   ```bash
   pnpm dev
   ```

7. **Verify setup**:
   - Application runs on `http://localhost:3000` (default)
   - Hot reload works for Go and CSS changes

---

## Development Workflow

### Branching Strategy

We use a feature branch workflow:

1. **Create a feature branch** from `main`:

   ```bash
   git checkout main
   git pull origin main
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** with descriptive commits:

   ```bash
   git add .
   git commit -m "feat: add user export functionality"
   ```

3. **Push to your fork**:

   ```bash
   git push -u origin feature/your-feature-name
   ```

4. **Open a Pull Request** against `main`

### Commit Message Format

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types**:

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks
- `perf`: Performance improvements

**Examples**:

```bash
feat(users): add bulk user import
fix(groups): correct member count display
docs(readme): update installation instructions
test(handlers): add integration tests for user endpoints
refactor(cache): extract caching logic to separate package
```

---

## Code Standards

### Go Code

**Style Guide**:

- Use `gofmt` for formatting (automatic)
- Follow [Effective Go](https://go.dev/doc/effective_go) principles
- Keep functions focused (single responsibility)
- Use descriptive names (avoid abbreviations)

**GoDoc Comments**:

```go
// UserHandler handles HTTP requests for user management operations.
// It provides CRUD operations and search functionality for LDAP users.
type UserHandler struct {
    // ...
}

// ListUsers returns all users matching the given filter.
// Returns an error if the LDAP connection fails.
func (h *UserHandler) ListUsers(ctx context.Context, filter string) ([]*User, error) {
    // Implementation
}
```

**Error Handling**:

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to fetch users: %w", err)
}

// Bad: Return raw errors
if err != nil {
    return err
}
```

### templ Templates

**Guidelines**:

- Use semantic HTML elements
- Include ARIA labels for accessibility
- Keep templates focused (atomic design)
- Use CSS classes from Tailwind

### CSS/Tailwind

**Guidelines**:

- Use Tailwind utility classes (avoid custom CSS)
- Support dark mode with `dark:` variants
- Use density variants: `comfortable:` and `compact:`
- Ensure WCAG 2.2 AAA contrast ratios

---

## Testing Requirements

### Test Types

- **Unit Tests**: Fast tests without external dependencies
- **Integration Tests**: Tests requiring Docker (OpenLDAP container)
- **E2E Tests**: Browser-based tests with Playwright
- **Fuzz Tests**: Randomized input testing
- **Mutation Tests**: Code coverage quality validation

### Running Tests

```bash
# Quick unit tests
make test-quick

# Full test suite with coverage
make test

# Integration tests (requires Docker)
make test-integration

# E2E tests (requires Playwright)
make test-e2e

# Fuzz tests
make test-fuzz

# Mutation tests
make test-mutation

# All tests
make test-all
```

### Test Coverage Goals

- Maintain overall coverage above 80%
- Critical paths (authentication, LDAP operations) should have >90% coverage
- New features must include tests

### Writing Tests

**Table-Driven Tests**:

```go
func TestUserHandler_Create(t *testing.T) {
    tests := []struct {
        name    string
        input   CreateUserRequest
        wantErr bool
    }{
        {"valid user", CreateUserRequest{...}, false},
        {"missing username", CreateUserRequest{}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := handler.Create(ctx, tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## Pull Request Process

### Before Submitting

1. **Run quality checks**:

   ```bash
   make qa
   ```

2. **Run tests**:

   ```bash
   make test
   ```

3. **Build assets**:

   ```bash
   pnpm build:css
   templ generate
   ```

4. **Update documentation** if needed:
   - API changes: Update relevant docs
   - New features: Update README.md
   - Breaking changes: Note in PR description

### PR Checklist

- [ ] Code follows project style guidelines
- [ ] All tests pass (`make test`)
- [ ] Test coverage maintained or improved
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow Conventional Commits format
- [ ] No sensitive data in code or commits
- [ ] Accessibility tested (if UI changes)
- [ ] Dark mode tested (if UI changes)

### Review Process

1. **Automated checks** run on PR (CI/CD)
2. **Code review** by maintainers
3. **Address feedback** and push updates
4. **Approval and merge** via merge queue

---

## Community

### Getting Help

- **Documentation**: Check [README.md](README.md)
- **Issues**: Search [existing issues](https://github.com/netresearch/ldap-manager/issues)

### Reporting Bugs

When reporting bugs, include:

- **Environment**: OS, Go version, browser (if frontend issue)
- **Steps to reproduce**: Clear, step-by-step instructions
- **Expected behavior**: What should happen
- **Actual behavior**: What actually happens
- **Logs/Screenshots**: Relevant error messages or screenshots

### Suggesting Features

For feature requests:

- **Use case**: Describe the problem you're solving
- **Proposed solution**: How would you implement it?
- **Alternatives**: What other approaches did you consider?
- **Scope**: Is this a small enhancement or major feature?

### Security Issues

**Do not open public issues for security vulnerabilities.**

Report security issues privately:

- See [SECURITY.md](SECURITY.md) for reporting process
- Use GitHub Security Advisories for responsible disclosure

---

## License

By contributing to this project, you agree that your contributions will be licensed under the project's [MIT License](LICENSE).

---

**Thank you for contributing!** Your efforts help make this project better for everyone.
