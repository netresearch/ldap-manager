//go:build !integration

package web

import "testing"

// ldapContainerEnv skips the test: without -tags=integration no
// testcontainer is started, so the plain unit run (go test ./...) needs no
// Docker. Run make test-integration to execute these tests.
func ldapContainerEnv(t *testing.T, addr string, dialErr error) *ldapIntegrationEnv {
	t.Helper()
	t.Skipf("OpenLDAP not available at %s and no testcontainer without -tags=integration "+
		"(run make test-integration): %v", addr, dialErr)

	return nil
}
