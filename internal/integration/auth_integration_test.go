//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start OpenLDAP container
	config := DefaultOpenLDAPConfig()
	container, err := StartOpenLDAP(ctx, config)
	require.NoError(t, err, "Failed to start OpenLDAP container")
	defer container.Stop(ctx)

	// Wait for LDAP to be fully ready
	time.Sleep(2 * time.Second)

	// Seed test data
	err = container.SeedTestData(ctx)
	require.NoError(t, err, "Failed to seed test data")

	// Create LDAP client
	ldapConfig := ldap.Config{
		Server:            container.URI(),
		BaseDN:            container.BaseDN,
		IsActiveDirectory: false,
	}

	t.Run("valid admin credentials", func(t *testing.T) {
		client, err := ldap.New(ldapConfig, container.AdminDN, container.AdminPass)
		assert.NoError(t, err, "Should authenticate with valid admin credentials")
		if client != nil {
			// Verify connection works
			_, err = client.FindUsers()
			assert.NoError(t, err, "Should be able to query LDAP")
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		_, err := ldap.New(ldapConfig, container.AdminDN, "wrongpassword")
		assert.Error(t, err, "Should fail with invalid password")
	})

	t.Run("invalid DN", func(t *testing.T) {
		_, err := ldap.New(ldapConfig, "cn=nonexistent,"+container.BaseDN, "anypassword")
		assert.Error(t, err, "Should fail with invalid DN")
	})

	t.Run("empty password", func(t *testing.T) {
		_, err := ldap.New(ldapConfig, container.AdminDN, "")
		assert.Error(t, err, "Should fail with empty password")
	})

	t.Run("empty DN", func(t *testing.T) {
		_, err := ldap.New(ldapConfig, "", container.AdminPass)
		assert.Error(t, err, "Should fail with empty DN")
	})
}

func TestUserLookupIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := StartOpenLDAP(ctx, DefaultOpenLDAPConfig())
	require.NoError(t, err)
	defer container.Stop(ctx)

	time.Sleep(2 * time.Second)
	err = container.SeedTestData(ctx)
	require.NoError(t, err)

	ldapConfig := ldap.Config{
		Server:            container.URI(),
		BaseDN:            container.BaseDN,
		IsActiveDirectory: false,
	}

	client, err := ldap.New(ldapConfig, container.AdminDN, container.AdminPass)
	require.NoError(t, err)

	t.Run("find all users", func(t *testing.T) {
		users, err := client.FindUsers()
		assert.NoError(t, err)
		assert.NotEmpty(t, users, "Should find at least one user")
	})

	t.Run("find all groups", func(t *testing.T) {
		groups, err := client.FindGroups()
		assert.NoError(t, err)
		// Groups might or might not exist depending on seeding
		t.Logf("Found %d groups", len(groups))
	})
}
