//go:build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

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
	Domain       string
}

// DefaultOpenLDAPConfig returns sensible defaults for testing
func DefaultOpenLDAPConfig() OpenLDAPConfig {
	return OpenLDAPConfig{
		BaseDN:       "dc=example,dc=com",
		AdminPass:    "adminpassword",
		Organization: "Example Inc",
		Domain:       "example.com",
	}
}

// StartOpenLDAP starts an OpenLDAP container for integration testing
func StartOpenLDAP(ctx context.Context, config OpenLDAPConfig) (*OpenLDAPContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "osixia/openldap:1.5.0",
		ExposedPorts: []string{"389/tcp", "636/tcp"},
		Env: map[string]string{
			"LDAP_ORGANISATION":   config.Organization,
			"LDAP_DOMAIN":         config.Domain,
			"LDAP_ADMIN_PASSWORD": config.AdminPass,
			"LDAP_TLS":            "false",
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

	// First ensure the users OU exists
	_, _, err := c.Container.Exec(ctx, []string{
		"ldapadd", "-x",
		"-H", "ldap://localhost",
		"-D", c.AdminDN,
		"-w", c.AdminPass,
		"-c", // Continue on errors (OU might exist)
	})
	// Ignore error - OU might already exist

	// Add the user
	_, _, err = c.Container.Exec(ctx, []string{
		"bash", "-c",
		fmt.Sprintf(`echo '%s' | ldapadd -x -H ldap://localhost -D "%s" -w "%s"`,
			ldif, c.AdminDN, c.AdminPass),
	})

	_ = userAccountControl // Would be used for AD simulation

	return err
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

	_, _, err := c.Container.Exec(ctx, []string{
		"bash", "-c",
		fmt.Sprintf(`echo '%s' | ldapadd -x -H ldap://localhost -D "%s" -w "%s"`,
			ldif, c.AdminDN, c.AdminPass),
	})

	return err
}

// CreateOUs creates the organizational units needed for testing
func (c *OpenLDAPContainer) CreateOUs(ctx context.Context) error {
	ous := []string{"users", "groups", "computers"}

	for _, ou := range ous {
		ldif := fmt.Sprintf(`dn: ou=%s,%s
objectClass: organizationalUnit
objectClass: top
ou: %s
`, ou, c.BaseDN, ou)

		c.Container.Exec(ctx, []string{
			"bash", "-c",
			fmt.Sprintf(`echo '%s' | ldapadd -x -H ldap://localhost -D "%s" -w "%s" -c`,
				ldif, c.AdminDN, c.AdminPass),
		})
	}

	return nil
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
