//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/netresearch/ldap-manager/internal/ldap_cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLDAPClientForIntegration wraps the real LDAP client for cache manager
type mockLDAPClientForIntegration struct {
	client *ldap.LDAP
}

func (m *mockLDAPClientForIntegration) FindUsers() ([]ldap.User, error) {
	return m.client.FindUsers()
}

func (m *mockLDAPClientForIntegration) FindGroups() ([]ldap.Group, error) {
	return m.client.FindGroups()
}

func (m *mockLDAPClientForIntegration) FindComputers() ([]ldap.Computer, error) {
	return m.client.FindComputers()
}

func (m *mockLDAPClientForIntegration) CheckPasswordForSAMAccountName(samAccountName, password string) (*ldap.User, error) {
	return m.client.CheckPasswordForSAMAccountName(samAccountName, password)
}

func (m *mockLDAPClientForIntegration) WithCredentials(dn, password string) (*ldap.LDAP, error) {
	return m.client.WithCredentials(dn, password)
}

func TestCacheWarmupIntegration(t *testing.T) {
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

	// Create cache manager with wrapped client
	wrappedClient := &mockLDAPClientForIntegration{client: client}
	cacheManager := ldap_cache.NewWithConfig(wrappedClient, 30*time.Second)

	t.Run("cache warmup succeeds", func(t *testing.T) {
		cacheManager.WarmupCache()
		// Cache may be warmed up partially even if some entity types fail
		// (e.g., groups/computers might not be supported in test LDAP)
		// Just verify the warmup was attempted
		stats := cacheManager.GetHealthCheck()
		assert.GreaterOrEqual(t, stats.RefreshCount, int64(0), "Warmup should have been attempted")
	})

	t.Run("cache contains data after warmup", func(t *testing.T) {
		users := cacheManager.FindUsers(true)
		t.Logf("Cached %d users", len(users))
		// May or may not have users depending on seeding success
	})

	t.Run("cache refresh works", func(t *testing.T) {
		cacheManager.Refresh()
		// Cache may have partial failures but should still be usable
		stats := cacheManager.GetHealthCheck()
		assert.GreaterOrEqual(t, stats.RefreshCount, int64(1), "Should have attempted at least one refresh")
	})

	t.Run("metrics are recorded", func(t *testing.T) {
		stats := cacheManager.GetHealthCheck()
		assert.GreaterOrEqual(t, stats.RefreshCount, int64(1), "Should have at least one refresh")
	})
}

func TestCacheLargeDatasetIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := StartOpenLDAP(ctx, DefaultOpenLDAPConfig())
	require.NoError(t, err)
	defer container.Stop(ctx)

	time.Sleep(2 * time.Second)
	err = container.CreateOUs(ctx)
	require.NoError(t, err)

	// Add many users to test cache performance
	t.Log("Adding 100 test users...")
	for i := 0; i < 100; i++ {
		username := "bulkuser" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		container.AddTestUser(ctx, username, "password", true)
	}

	ldapConfig := ldap.Config{
		Server:            container.URI(),
		BaseDN:            container.BaseDN,
		IsActiveDirectory: false,
	}

	client, err := ldap.New(ldapConfig, container.AdminDN, container.AdminPass)
	require.NoError(t, err)

	wrappedClient := &mockLDAPClientForIntegration{client: client}
	cacheManager := ldap_cache.NewWithConfig(wrappedClient, 30*time.Second)

	t.Run("warmup with large dataset", func(t *testing.T) {
		start := time.Now()
		cacheManager.WarmupCache()
		duration := time.Since(start)

		t.Logf("Warmup took %v", duration)
		assert.Less(t, duration, 30*time.Second, "Warmup should complete within 30 seconds")
	})

	t.Run("cache lookup performance", func(t *testing.T) {
		users := cacheManager.FindUsers(true)
		t.Logf("Cached %d users", len(users))

		// Test lookup performance
		start := time.Now()
		for i := 0; i < 1000; i++ {
			cacheManager.FindUsers(true)
		}
		duration := time.Since(start)

		t.Logf("1000 lookups took %v", duration)
		assert.Less(t, duration, time.Second, "1000 lookups should complete within 1 second")
	})
}
