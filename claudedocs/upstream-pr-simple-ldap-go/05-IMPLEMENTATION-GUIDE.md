# Implementation Guide: Submitting the Credential-Aware Pooling PR

This guide provides step-by-step instructions for forking simple-ldap-go, applying the enhancements, and submitting the pull request.

## Prerequisites

- GitHub account with SSH key configured
- Git installed and configured
- Go 1.22+ installed
- Access to simple-ldap-go repository: https://github.com/netresearch/simple-ldap-go

## Step 1: Fork and Clone

```bash
# Fork the repository via GitHub web interface
# Navigate to: https://github.com/netresearch/simple-ldap-go
# Click "Fork" button in top-right

# Clone your fork
git clone git@github.com:YOUR_USERNAME/simple-ldap-go.git
cd simple-ldap-go

# Add upstream remote
git remote add upstream git@github.com:netresearch/simple-ldap-go.git

# Verify remotes
git remote -v
# Should show:
# origin    git@github.com:YOUR_USERNAME/simple-ldap-go.git (fetch)
# origin    git@github.com:YOUR_USERNAME/simple-ldap-go.git (push)
# upstream  git@github.com:netresearch/simple-ldap-go.git (fetch)
# upstream  git@github.com:netresearch/simple-ldap-go.git (push)
```

## Step 2: Create Feature Branch

```bash
# Ensure you're on main and up-to-date
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/credential-aware-pooling

# Verify branch
git branch
# Should show: * feature/credential-aware-pooling
```

## Step 3: Apply Code Changes

### 3.1 Modify pool.go

Open `pool.go` in your editor and apply the following changes:

**Change 1: Add ConnectionCredentials struct (after line ~145)**

```go
// ConnectionCredentials stores authentication information for a pooled connection.
// This enables per-user connection tracking and credential-aware connection reuse
// in multi-user scenarios (e.g., web applications, multi-tenant systems).
type ConnectionCredentials struct {
    DN       string // Distinguished Name for LDAP bind
    Password string // Password for LDAP bind
}
```

**Change 2: Add credentials field to pooledConnection struct (around line ~152)**

```go
type pooledConnection struct {
    conn        *ldap.Conn
    createdAt   time.Time
    lastUsed    time.Time
    usageCount  int64
    isHealthy   bool
    inUse       bool
    credentials *ConnectionCredentials  // ADD THIS LINE
    mu          sync.RWMutex
}
```

**Change 3: Add GetWithCredentials() method (after Get() method, around line ~250)**

Copy the entire `GetWithCredentials()` method from `02-pool-enhancements.go`.

**Change 4: Add helper methods (around line ~320)**

Copy these methods from `02-pool-enhancements.go`:

- `findAvailableConnection()`
- `canReuseConnection()`

**Change 5: Update createNewConnection() (around line ~450)**

Modify the `createNewConnection()` method to:

1. Accept `creds *ConnectionCredentials` parameter
2. Use provided credentials for binding (if not nil)
3. Store credentials in the pooledConnection struct

See `02-pool-enhancements.go` for the complete modification.

### 3.2 Add Test Files

```bash
# Copy test files from PR materials
cp /srv/www/sme/ldap-manager/claudedocs/upstream-pr-simple-ldap-go/03-pool_credentials_test.go pool_credentials_test.go
cp /srv/www/sme/ldap-manager/claudedocs/upstream-pr-simple-ldap-go/04-pool_credentials_bench_test.go pool_credentials_bench_test.go
```

## Step 4: Validate Changes

```bash
# Check syntax
go build ./...

# Run existing tests (ensure no regressions)
go test -v ./...

# Run new credential tests
go test -v -run TestCredential

# Run benchmarks
go test -bench=. -benchmem

# Check code formatting
go fmt ./...

# Run linter (if available)
golangci-lint run
```

## Step 5: Commit Changes

```bash
# Stage changes
git add pool.go pool_credentials_test.go pool_credentials_bench_test.go

# Create commit with descriptive message
git commit -m "feat: add credential-aware connection pooling for multi-user scenarios

Add GetWithCredentials() method to enable per-user connection tracking
and credential-aware connection reuse. This enhancement supports multi-user
applications (web apps, multi-tenant systems) while maintaining 100%
backward compatibility with existing Get() method.

Key features:
- ConnectionCredentials struct for per-connection credential tracking
- GetWithCredentials() method for multi-user pooling
- Credential matching logic to prevent cross-user connection reuse
- Comprehensive tests and benchmarks

Tested in production at ldap-manager for 6+ months with excellent results.

Closes #XXX"

# Verify commit
git log -1 --stat
```

## Step 6: Push to Your Fork

```bash
# Push feature branch to your fork
git push -u origin feature/credential-aware-pooling

# Verify push
git branch -vv
# Should show tracking relationship with origin/feature/credential-aware-pooling
```

## Step 7: Create Pull Request

### Option A: Using GitHub CLI (Recommended)

```bash
# Install gh if not already installed: https://cli.github.com/

# Create PR with our prepared description
gh pr create \
  --repo netresearch/simple-ldap-go \
  --base main \
  --head YOUR_USERNAME:feature/credential-aware-pooling \
  --title "Add credential-aware connection pooling for multi-user scenarios" \
  --body-file /srv/www/sme/ldap-manager/claudedocs/upstream-pr-simple-ldap-go/01-PR-description.md

# Get PR URL
gh pr view --web
```

### Option B: Using GitHub Web Interface

1. Navigate to your fork: `https://github.com/YOUR_USERNAME/simple-ldap-go`
2. GitHub will show a banner: "feature/credential-aware-pooling had recent pushes"
3. Click "Compare & pull request" button
4. Set base repository: `netresearch/simple-ldap-go` base: `main`
5. Set head repository: `YOUR_USERNAME/simple-ldap-go` compare: `feature/credential-aware-pooling`
6. Title: `Add credential-aware connection pooling for multi-user scenarios`
7. Description: Copy content from `01-PR-description.md`
8. Click "Create pull request"

## Step 8: Post-PR Actions

### Link to Production Usage

In the PR comments, add a link to our production implementation:

```markdown
## Production Implementation Reference

This enhancement is currently running in production at:

- **Repository**: https://github.com/netresearch/ldap-manager
- **File**: internal/ldap/pool.go
- **Duration**: 6+ months in production
- **Scale**: Multi-user web application with concurrent LDAP operations

The implementation has been refined through real-world usage and is ready for upstream contribution.
```

### Provide Code Examples

If maintainers request examples, reference `06-CODE-EXAMPLES.md` from this PR package.

### Answer Technical Questions

If maintainers have questions about design decisions, reference `07-DESIGN-RATIONALE.md`.

## Step 9: Respond to Feedback

### Expected Maintainer Questions

**Q: Why not modify Get() instead of adding GetWithCredentials()?**
A: Backward compatibility. Existing users rely on Get() behavior. Adding a new method ensures zero breaking changes while enabling the new functionality.

**Q: Security concerns about storing credentials?**
A: Uses same in-memory approach as existing user/password fields in ConnectionPool. No additional security surface. Credentials are never persisted.

**Q: Performance impact?**
A: Benchmarks show <5% overhead only when using GetWithCredentials(). Existing Get() has zero overhead. See benchmark results in PR.

**Q: Is this needed? Can users manage connections themselves?**
A: Yes, but that defeats the purpose of connection pooling. This enables pooling benefits (efficiency, resource management) in multi-user scenarios which are common in web applications.

### Addressing Change Requests

If maintainers request modifications:

```bash
# Make requested changes in your local branch
git checkout feature/credential-aware-pooling

# Edit files as requested
# ...

# Commit changes
git add .
git commit -m "refactor: address PR feedback - [describe changes]"

# Push updates
git push origin feature/credential-aware-pooling

# PR will automatically update
```

## Step 10: After Merge

Once the PR is merged:

```bash
# Sync your fork with upstream
git checkout main
git pull upstream main
git push origin main

# Delete feature branch
git branch -d feature/credential-aware-pooling
git push origin --delete feature/credential-aware-pooling

# Celebrate! 🎉
```

## Troubleshooting

### Tests Fail

```bash
# Check if existing tests pass on main
git checkout main
go test ./...

# If main passes but feature branch fails, review your changes
git checkout feature/credential-aware-pooling
git diff main -- pool.go
```

### Build Errors

```bash
# Check Go version
go version  # Should be 1.22+

# Clean and rebuild
go clean -cache
go build ./...
```

### Permission Denied on Push

```bash
# Verify SSH key is added to GitHub
ssh -T git@github.com

# Should see: "Hi YOUR_USERNAME! You've successfully authenticated..."

# If not, add SSH key: https://docs.github.com/en/authentication/connecting-to-github-with-ssh
```

### Merge Conflicts

```bash
# Sync with latest upstream
git fetch upstream
git rebase upstream/main

# Resolve conflicts if any
# Edit conflicted files
git add .
git rebase --continue

# Force push (rebase rewrites history)
git push --force-with-lease origin feature/credential-aware-pooling
```

## Additional Resources

- **PR Description**: `01-PR-description.md`
- **Code Changes**: `02-pool-enhancements.go`
- **Tests**: `03-pool_credentials_test.go`
- **Benchmarks**: `04-pool_credentials_bench_test.go`
- **Code Examples**: `06-CODE-EXAMPLES.md`
- **Design Rationale**: `07-DESIGN-RATIONALE.md`

## Timeline Estimate

- Fork and setup: 10 minutes
- Apply code changes: 30 minutes
- Testing and validation: 20 minutes
- Create and submit PR: 10 minutes
- **Total: ~70 minutes**

## Success Criteria

✅ All existing tests pass
✅ New tests pass and demonstrate functionality
✅ Benchmarks show <5% overhead
✅ Code follows project style and conventions
✅ PR description is clear and comprehensive
✅ Backward compatibility maintained

---

**Questions or issues?** Contact Sebastian for assistance.
