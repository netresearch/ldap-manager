//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ldapResultEntryAlreadyExists is the LDAP result code entryAlreadyExists
// (RFC 4511), which ldapadd also uses as its exit code for an entry that is
// already present.
const ldapResultEntryAlreadyExists = 68

// OpenLDAPContainer represents a running OpenLDAP container for testing
type OpenLDAPContainer struct {
	Container testcontainers.Container
	Host      string
	Port      string
	BaseDN    string
	AdminDN   string
	AdminPass string
}

// OpenLDAPConfig holds configuration for the OpenLDAP container
type OpenLDAPConfig struct {
	BaseDN       string
	AdminPass    string
	Organization string
}

// openLDAPImage is the osixia/openldap image the test containers run.
const openLDAPImage = "osixia/openldap:2.6.15"

// openLDAPURLs makes slapd listen on the standard ports 389 and 636 instead
// of the image's default 3890 and 6360. Docker lets the image's unprivileged
// ldap user bind them.
const openLDAPURLs = "ldap://:389 ldaps://:636 ldapi:///"

// DefaultOpenLDAPConfig returns sensible defaults for testing
func DefaultOpenLDAPConfig() OpenLDAPConfig {
	return OpenLDAPConfig{
		BaseDN:       "dc=example,dc=com",
		AdminPass:    "adminpassword",
		Organization: "Example Inc",
	}
}

// StartOpenLDAP starts an OpenLDAP container for integration testing
func StartOpenLDAP(ctx context.Context, config OpenLDAPConfig) (*OpenLDAPContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        openLDAPImage,
		ExposedPorts: []string{"389/tcp"},
		Env: map[string]string{
			"OPENLDAP_URLS":                   openLDAPURLs,
			"OPENLDAP_BOOTSTRAP_ORGANIZATION": config.Organization,
			"OPENLDAP_BOOTSTRAP_SUFFIX":       config.BaseDN,
			// Despite its name, the variable takes any userPassword value;
			// a value without {SCHEME} prefix is stored as cleartext.
			"OPENLDAP_BOOTSTRAP_DATA_ROOT_PASSWORD_HASHED": config.AdminPass,
		},
		WaitingFor: wait.ForLog("slapd starting").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start OpenLDAP container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "389")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	return &OpenLDAPContainer{
		Container: container,
		Host:      host,
		Port:      port.Port(),
		BaseDN:    config.BaseDN,
		AdminDN:   "cn=admin," + config.BaseDN,
		AdminPass: config.AdminPass,
	}, nil
}

// Stop terminates the OpenLDAP container
func (c *OpenLDAPContainer) Stop(ctx context.Context) error {
	if c.Container != nil {
		return c.Container.Terminate(ctx)
	}

	return nil
}

// URI returns the LDAP URI for connecting to this container
func (c *OpenLDAPContainer) URI() string {
	return fmt.Sprintf("ldap://%s:%s", c.Host, c.Port)
}

// AddTestUser adds a test user to the LDAP directory
func (c *OpenLDAPContainer) AddTestUser(ctx context.Context, username, password string, enabled bool) error {
	// Use ldapadd via container exec
	userAccountControl := "512" // Normal account
	if !enabled {
		userAccountControl = "514" // Disabled account
	}

	ldif := fmt.Sprintf(`dn: cn=%s,ou=users,%s
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
objectClass: top
cn: %s
sn: %s
uid: %s
userPassword: %s
description: Test user
`, username, c.BaseDN, username, username, username, password)

	// First ensure the users OU exists; it may already be there.
	if err := c.ensureOU(ctx, "users"); err != nil {
		return err
	}

	_ = userAccountControl // Would be used for AD simulation

	if err := c.ldapAdd(ctx, ldif); err != nil {
		return fmt.Errorf("failed to add user %s: %w", username, err)
	}

	return nil
}

// AddTestGroup adds a test group to the LDAP directory
func (c *OpenLDAPContainer) AddTestGroup(ctx context.Context, groupname string, members []string) error {
	memberDNs := ""
	for _, member := range members {
		memberDNs += fmt.Sprintf("member: cn=%s,ou=users,%s\n", member, c.BaseDN)
	}

	ldif := fmt.Sprintf(`dn: cn=%s,ou=groups,%s
objectClass: groupOfNames
objectClass: top
cn: %s
%s`, groupname, c.BaseDN, groupname, memberDNs)

	if err := c.ldapAdd(ctx, ldif); err != nil {
		return fmt.Errorf("failed to add group %s: %w", groupname, err)
	}

	return nil
}

// CreateOUs creates the organizational units needed for testing
func (c *OpenLDAPContainer) CreateOUs(ctx context.Context) error {
	ous := []string{"users", "groups", "computers"}

	for _, ou := range ous {
		if err := c.ensureOU(ctx, ou); err != nil {
			return err
		}
	}

	return nil
}

// ensureOU creates ou=<ou> below the base DN. An OU that already exists is
// not an error.
func (c *OpenLDAPContainer) ensureOU(ctx context.Context, ou string) error {
	ldif := fmt.Sprintf(`dn: ou=%s,%s
objectClass: organizationalUnit
objectClass: top
ou: %s
`, ou, c.BaseDN, ou)

	if err := c.ldapAdd(ctx, ldif, ldapResultEntryAlreadyExists); err != nil {
		return fmt.Errorf("failed to create OU %s: %w", ou, err)
	}

	return nil
}

// userPasswordValue matches the value of a userPassword line, so that ldapAdd
// can drop it from the output it returns.
var userPasswordValue = regexp.MustCompile(`(?i)(userPassword:).*`)

// ldapAdd pipes ldif into ldapadd inside the container as the admin DN. It
// fails when the command cannot be run or exits non-zero, unless the exit
// code is one of tolerated. The output in the error has the admin password
// and userPassword values replaced, because bash echoes the whole command
// line when it cannot parse it.
func (c *OpenLDAPContainer) ldapAdd(ctx context.Context, ldif string, tolerated ...int) error {
	code, out, err := c.Container.Exec(ctx, []string{
		"bash", "-c",
		fmt.Sprintf(`echo '%s' | ldapadd -x -H ldap://localhost -D "%s" -w "%s"`,
			ldif, c.AdminDN, c.AdminPass),
	}, tcexec.Multiplexed())
	if err != nil {
		return fmt.Errorf("exec ldapadd: %w", err)
	}

	if code == 0 || slices.Contains(tolerated, code) {
		return nil
	}

	output, readErr := io.ReadAll(out)
	if readErr != nil {
		return fmt.Errorf("ldapadd exited with code %d (reading its output failed: %w)", code, readErr)
	}

	redacted := strings.ReplaceAll(string(output), c.AdminPass, "[REDACTED]")
	redacted = userPasswordValue.ReplaceAllString(redacted, "${1} [REDACTED]")

	return fmt.Errorf("ldapadd exited with code %d: %s", code, redacted)
}

// SeedTestData populates the LDAP directory with test data
func (c *OpenLDAPContainer) SeedTestData(ctx context.Context) error {
	// Create OUs first
	if err := c.CreateOUs(ctx); err != nil {
		return fmt.Errorf("failed to create OUs: %w", err)
	}

	// Add test users
	testUsers := []struct {
		username string
		password string
		enabled  bool
	}{
		{"testuser1", "password1", true},
		{"testuser2", "password2", true},
		{"testuser3", "password3", false}, // disabled
		{"admin", "adminpass", true},
	}

	for _, user := range testUsers {
		if err := c.AddTestUser(ctx, user.username, user.password, user.enabled); err != nil {
			// Continue on error - user might exist
			continue
		}
	}

	// Add test groups
	testGroups := []struct {
		name    string
		members []string
	}{
		{"admins", []string{"admin", "testuser1"}},
		{"users", []string{"testuser1", "testuser2", "testuser3"}},
		{"readonly", []string{"testuser2"}},
	}

	for _, group := range testGroups {
		if err := c.AddTestGroup(ctx, group.name, group.members); err != nil {
			// Continue on error - group might exist
			continue
		}
	}

	return nil
}
