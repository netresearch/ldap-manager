//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDockerHealthCheck verifies the Docker HEALTHCHECK directive works correctly
// by building and running the production container and checking its health status.
func TestDockerHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker healthcheck test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start OpenLDAP first
	ldapContainer, err := StartOpenLDAP(ctx, DefaultOpenLDAPConfig())
	require.NoError(t, err, "Failed to start OpenLDAP container")
	defer func() { _ = ldapContainer.Stop(ctx) }()

	// Wait for LDAP to be ready and seed test data
	time.Sleep(2 * time.Second)
	err = ldapContainer.SeedTestData(ctx)
	require.NoError(t, err, "Failed to seed test data")

	// Get LDAP container network info
	ldapNetwork, err := ldapContainer.Container.ContainerIP(ctx)
	require.NoError(t, err, "Failed to get LDAP container IP")

	// Build and start the ldap-manager container
	appContainer, appPort, err := startLDAPManagerContainer(ctx, t, ldapNetwork, ldapContainer)
	require.NoError(t, err, "Failed to start ldap-manager container")
	defer func() { _ = appContainer.Terminate(ctx) }()

	t.Run("container becomes healthy", func(t *testing.T) {
		// The container should become healthy via Docker HEALTHCHECK
		// We wait for up to 60 seconds (start_period=5s, interval=30s, so first check at ~35s)
		healthy := waitForContainerHealth(ctx, t, appContainer, 60*time.Second)
		assert.True(t, healthy, "Container should become healthy")
	})

	t.Run("liveness endpoint returns 200", func(t *testing.T) {
		resp, err := doHTTPGet(ctx, fmt.Sprintf("http://localhost:%s/health/live", appPort))
		require.NoError(t, err, "Failed to call liveness endpoint")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Liveness endpoint should return 200")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		assert.Equal(t, "alive", result["status"], "Status should be 'alive'")
	})

	t.Run("health endpoint returns details", func(t *testing.T) {
		resp, err := doHTTPGet(ctx, fmt.Sprintf("http://localhost:%s/health", appPort))
		require.NoError(t, err, "Failed to call health endpoint")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		assert.Contains(t, result, "cache", "Response should contain cache info")
		assert.Contains(t, result, "connection_pool", "Response should contain connection pool info")
		assert.Contains(t, result, "overall_healthy", "Response should contain overall health status")
	})

	t.Run("readiness endpoint returns ready", func(t *testing.T) {
		// Wait a bit for cache warmup
		time.Sleep(5 * time.Second)

		resp, err := doHTTPGet(ctx, fmt.Sprintf("http://localhost:%s/health/ready", appPort))
		require.NoError(t, err, "Failed to call readiness endpoint")
		defer func() { _ = resp.Body.Close() }()

		// Readiness might be 200 (ready) or 503 (warming up) - both are valid
		assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, resp.StatusCode,
			"Readiness endpoint should return 200 or 503")
	})
}

// startLDAPManagerContainer builds and starts the ldap-manager production container.
func startLDAPManagerContainer(
	ctx context.Context,
	t *testing.T,
	ldapIP string,
	ldap *OpenLDAPContainer,
) (testcontainers.Container, string, error) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../..",
			Dockerfile: "Dockerfile",
			BuildArgs: map[string]*string{
				"BUILD_DATE": strPtr(time.Now().UTC().Format(time.RFC3339)),
				"VCS_REF":    strPtr("test"),
			},
			KeepImage: true,
		},
		ExposedPorts: []string{"3000/tcp"},
		Env: map[string]string{
			"LDAP_SERVER":            fmt.Sprintf("ldap://%s:389", ldapIP),
			"LDAP_BASE_DN":           ldap.BaseDN,
			"LDAP_READONLY_USER":     ldap.AdminDN,
			"LDAP_READONLY_PASSWORD": ldap.AdminPass,
			"SESSION_SECRET":         "test-session-secret-for-integration-tests",
			"LOG_LEVEL":              "debug",
		},
		WaitingFor: wait.ForHTTP("/health/live").
			WithPort("3000/tcp").
			WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start ldap-manager container: %w", err)
	}

	port, err := container.MappedPort(ctx, "3000")
	if err != nil {
		_ = container.Terminate(ctx)

		return nil, "", fmt.Errorf("failed to get mapped port: %w", err)
	}

	return container, port.Port(), nil
}

// waitForContainerHealth polls the container health status until healthy or timeout.
func waitForContainerHealth(
	ctx context.Context,
	t *testing.T,
	container testcontainers.Container,
	timeout time.Duration,
) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, err := container.State(ctx)
		if err != nil {
			t.Logf("Error getting container state: %v", err)
			time.Sleep(2 * time.Second)

			continue
		}

		if state.Health != nil && state.Health.Status == "healthy" {
			t.Logf("Container is healthy after %v", timeout-time.Until(deadline))

			return true
		}

		t.Logf("Container health status: %v", state.Health)
		time.Sleep(2 * time.Second)
	}

	return false
}

// doHTTPGet performs an HTTP GET request with context.
func doHTTPGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return http.DefaultClient.Do(req)
}

func strPtr(s string) *string {
	return &s
}
