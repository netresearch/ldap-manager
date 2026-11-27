//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDockerHealthCheck verifies the Docker HEALTHCHECK directive of the
// production image. It builds that image the way the release does (BuildKit,
// the shipped Dockerfile, a pre-built bin/ldap-manager-linux-<arch> in the
// build context), runs it against an OpenLDAP container and waits for Docker to
// report the container healthy.
//
// Requirements: a Docker daemon, the docker CLI with the buildx plugin, and the
// Go toolchain on PATH. Run with: make test-integration
func TestDockerHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker healthcheck test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	image := buildProductionImage(ctx, t)

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

	// Start the ldap-manager container from the production image
	appContainer, appPort, err := startLDAPManagerContainer(ctx, t, image, ldapNetwork, ldapContainer)
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

// buildProductionImage builds the production image from the shipped
// Dockerfile and returns its tag. Like release.yml, it compiles the linux
// binary outside Docker and hands it to the Dockerfile's binary-selector stage
// through bin/ in the build context, and it builds with BuildKit (docker
// buildx), which sets TARGETARCH and supports COPY --chmod. The image is
// removed when the test ends.
func buildProductionImage(ctx context.Context, t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err, "Failed to resolve repository root")

	// Build for the daemon's architecture, which may differ from the test
	// binary's (e.g. a remote daemon).
	archOut, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Arch}}").Output()
	require.NoError(t, err, "Failed to read the Docker daemon architecture")
	arch := strings.TrimSpace(string(archOut))
	switch arch {
	case "386", "amd64", "arm64":
	default:
		t.Fatalf("Docker daemon architecture %q is not covered by this test", arch)
	}

	buildContext := t.TempDir()
	binary := filepath.Join(buildContext, "bin", "ldap-manager-linux-"+arch)

	goBuild := exec.CommandContext(ctx, "go", "build", "-trimpath",
		"-ldflags", "-s -w -X main.version=healthcheck-test",
		"-o", binary, "./cmd/ldap-manager")
	goBuild.Dir = repoRoot
	goBuild.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
	out, err := goBuild.CombinedOutput()
	require.NoError(t, err, "Failed to build %s:\n%s", binary, out)

	tag := fmt.Sprintf("ldap-manager-healthcheck-test:%d", time.Now().UnixNano())
	dockerBuild := exec.CommandContext(ctx, "docker", "buildx", "build",
		"--load",
		"--platform", "linux/"+arch,
		"--file", filepath.Join(repoRoot, "Dockerfile"),
		"--tag", tag,
		"--build-arg", "BUILD_DATE="+time.Now().UTC().Format(time.RFC3339),
		"--build-arg", "VCS_REF=test",
		buildContext)
	out, err = dockerBuild.CombinedOutput()
	require.NoError(t, err, "Failed to build the production image:\n%s", out)

	t.Cleanup(func() {
		rm := exec.CommandContext(context.Background(), "docker", "image", "rm", "--force", tag)
		if rmOut, rmErr := rm.CombinedOutput(); rmErr != nil {
			t.Logf("Failed to remove image %s: %v\n%s", tag, rmErr, rmOut)
		}
	})

	return tag
}

// startLDAPManagerContainer starts the ldap-manager production image.
func startLDAPManagerContainer(
	ctx context.Context,
	t *testing.T,
	image string,
	ldapIP string,
	ldap *OpenLDAPContainer,
) (testcontainers.Container, string, error) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        image,
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

		if state.Health != nil && state.Health.Status == "unhealthy" {
			for _, probe := range state.Health.Log {
				t.Logf("Health probe exit=%d output=%q", probe.ExitCode, probe.Output)
			}

			return false
		}

		if state.Health != nil && len(state.Health.Log) > 0 {
			last := state.Health.Log[len(state.Health.Log)-1]
			t.Logf("Container health: %s, failing streak %d, last probe exit=%d output=%q",
				state.Health.Status, state.Health.FailingStreak, last.ExitCode, last.Output)
		} else {
			t.Logf("Container health status: %v", state.Health)
		}
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
