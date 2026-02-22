# AGENTS.md — docs/

<!-- Managed by agent: keep sections & order; edit content, not structure. Last updated: 2026-02-22 -->

## Overview

Documentation and AI agent instructions for working with this codebase.

## Creating and Updating Screenshots

Screenshots are stored in `docs/assets/` and referenced in `README.md`. Follow these steps to create or update screenshots.

### Prerequisites

1. **Start the development environment:**

   ```bash
   docker compose --profile dev up -d
   ```

2. **Wait for services to be healthy:**

   ```bash
   docker compose --profile dev ps  # Check that ldap-server is healthy
   ```

3. **Note the application port:** The app runs on port 3000 internally, mapped to the host. Check `compose.yml` for the current port mapping (default: `3000:3000`).

### Setting Up Realistic Test Data

**Note:** The development LDAP server may already have pre-seeded test data with users (jsmith, mmueller, etc.) and groups. Check if data exists before creating:

```bash
docker exec ldap-server ldapsearch -x -H ldap://localhost \
  -D "cn=admin,dc=netresearch,dc=local" -w admin \
  -b "dc=netresearch,dc=local" "(uid=*)" dn
```

If users exist, you can skip the data creation steps. The pre-seeded password for all users is `password`.

If the container starts fresh, create realistic test data for better screenshots:

#### 1. Create Users OU and Users

```bash
cat << 'LDIF' | docker exec -i ldap-server ldapadd -x -H ldap://localhost \
  -D "cn=admin,dc=netresearch,dc=local" -w admin
dn: ou=Users,dc=netresearch,dc=local
objectClass: organizationalUnit
ou: Users

dn: uid=jsmith,ou=Users,dc=netresearch,dc=local
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
cn: John Smith
sn: Smith
givenName: John
uid: jsmith
mail: john.smith@netresearch.de
userPassword: password

dn: uid=mmueller,ou=Users,dc=netresearch,dc=local
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
cn: Maria Mueller
sn: Mueller
givenName: Maria
uid: mmueller
mail: maria.mueller@netresearch.de
userPassword: password

dn: uid=tschneider,ou=Users,dc=netresearch,dc=local
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
cn: Thomas Schneider
sn: Schneider
givenName: Thomas
uid: tschneider
mail: thomas.schneider@netresearch.de
userPassword: password

dn: uid=aweber,ou=Users,dc=netresearch,dc=local
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
cn: Anna Weber
sn: Weber
givenName: Anna
uid: aweber
mail: anna.weber@netresearch.de
userPassword: password

dn: uid=pfischer,ou=Users,dc=netresearch,dc=local
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
cn: Peter Fischer
sn: Fischer
givenName: Peter
uid: pfischer
mail: peter.fischer@netresearch.de
userPassword: password
LDIF
```

#### 2. Create Groups OU and Groups

**Important:** Use `groupOfNames` objectClass (not `posixGroup`) - the `simple-ldap-go` library filters on `(|(objectClass=group)(objectClass=groupOfNames))`.

```bash
cat << 'LDIF' | docker exec -i ldap-server ldapadd -x -H ldap://localhost \
  -D "cn=admin,dc=netresearch,dc=local" -w admin
dn: ou=Groups,dc=netresearch,dc=local
objectClass: organizationalUnit
ou: Groups

dn: cn=developers,ou=Groups,dc=netresearch,dc=local
objectClass: groupOfNames
cn: developers
description: Development Team
member: uid=jsmith,ou=Users,dc=netresearch,dc=local
member: uid=mmueller,ou=Users,dc=netresearch,dc=local
member: uid=tschneider,ou=Users,dc=netresearch,dc=local

dn: cn=operations,ou=Groups,dc=netresearch,dc=local
objectClass: groupOfNames
cn: operations
description: Operations Team
member: uid=pfischer,ou=Users,dc=netresearch,dc=local
member: uid=aweber,ou=Users,dc=netresearch,dc=local

dn: cn=administrators,ou=Groups,dc=netresearch,dc=local
objectClass: groupOfNames
cn: administrators
description: System Administrators
member: uid=jsmith,ou=Users,dc=netresearch,dc=local
member: uid=aweber,ou=Users,dc=netresearch,dc=local
LDIF
```

#### 3. Wait for Cache Refresh

The LDAP Manager caches data with a 30-second refresh interval:

```bash
sleep 35
```

### Taking Screenshots

Use Playwright MCP or browser automation to capture screenshots:

1. **Navigate to the application** (typically `http://localhost:3000`)

2. **Login** with test user credentials:
   - Username: `jsmith`
   - Password: `password`

3. **Capture required screenshots:**

| Screenshot                      | URL Path                                                  | Description                    |
| ------------------------------- | --------------------------------------------------------- | ------------------------------ |
| `ldap_manager_users.png`        | `/users`                                                  | Users list showing all users   |
| `ldap_manager_user_detail.png`  | `/users/uid=jsmith,ou=Users,dc=netresearch,dc=local`      | User detail page               |
| `ldap_manager_groups.png`       | `/groups`                                                 | Groups list showing all groups |
| `ldap_manager_group_detail.png` | `/groups/cn=developers,ou=Groups,dc=netresearch,dc=local` | Group detail with members      |

4. **Save screenshots to** `docs/assets/`

### Updating README.md

After capturing screenshots, ensure `README.md` references them correctly:

```markdown
## Screenshots

<img src="./docs/assets/ldap_manager_users.png" height="256" align="left" alt="LDAP Manager - Users List">
<img src="./docs/assets/ldap_manager_user_detail.png" height="256" align="left" alt="LDAP Manager - User Detail">
<br clear="all">
<img src="./docs/assets/ldap_manager_groups.png" height="256" align="left" alt="LDAP Manager - Groups List">
<img src="./docs/assets/ldap_manager_group_detail.png" height="256" align="left" alt="LDAP Manager - Group Detail">
<br clear="all">
```

### Troubleshooting

#### Groups not showing in UI

- Verify groups use `objectClass: groupOfNames` (not `posixGroup`)
- Wait for cache refresh (30 seconds)
- Check LDAP directly: `docker exec ldap-server ldapsearch -x -H ldap://localhost -D "cn=admin,dc=netresearch,dc=local" -w admin -b "ou=Groups,dc=netresearch,dc=local" "(objectClass=groupOfNames)"`

#### User group memberships not showing

- OpenLDAP requires the `memberOf` overlay for reverse lookups
- Groups show members correctly; user pages may show "No groups" without this overlay
- This is a known limitation of the development OpenLDAP setup

#### Port conflicts

- If port 3000 is in use, modify `compose.yml` temporarily (e.g., `3001:3000`)
- Remember to revert before committing

### Cleanup

After taking screenshots:

```bash
docker compose --profile dev down
```

## Setup

No special setup beyond the root-level dev environment. See root `README.md`.

## Build & Tests

```bash
# Verify documentation renders correctly
# Screenshots are in docs/assets/ - use Playwright MCP to capture
docker compose --profile dev up -d   # Start dev environment
docker compose --profile dev down    # Cleanup after screenshots
```

## Security

- Never include real credentials, passwords, or tokens in documentation or screenshots
- Use test data with generic passwords (e.g., `password`) for screenshots
- Redact sensitive information from example LDAP queries

## Code Style

- Follow existing patterns in the codebase
- Use conventional commits for commit messages
- Run `make check` before committing to ensure tests pass

## PR & Commit Checklist

- [ ] Screenshots are current and match the UI
- [ ] All example commands are tested and working
- [ ] No real credentials in documentation
- [ ] Image references in README.md point to correct paths

## Examples

### Taking screenshots

See "Creating and Updating Screenshots" section above for the complete workflow.

### Good: Referencing screenshots

```markdown
<img src="./docs/assets/ldap_manager_users.png" height="256" alt="Users List">
```

### Bad: Missing alt text or wrong paths

```markdown
![](screenshots/users.png) <!-- Wrong path, no alt text -->
```

## When stuck

1. **Screenshots not loading**: Check file paths in `docs/assets/` and README references
2. **LDAP data missing**: Verify dev LDAP server has test data seeded
3. **Groups not showing**: Ensure `objectClass: groupOfNames` (not `posixGroup`)
4. **Port conflicts**: Modify `compose.yml` port mapping temporarily
5. **Cache issues**: Wait 35 seconds for LDAP cache refresh after data changes
