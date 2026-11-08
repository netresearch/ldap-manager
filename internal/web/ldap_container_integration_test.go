//go:build integration

package web

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/netresearch/ldap-manager/internal/integration"
)

// sharedLDAP is the one OpenLDAP testcontainer of this test binary. The
// seed in seedLDAPData is idempotent, so all integration tests share it.
var sharedLDAP struct {
	once      sync.Once
	container *integration.OpenLDAPContainer
	env       *ldapIntegrationEnv
	err       error
}

// TestMain terminates the shared OpenLDAP container after the run.
func TestMain(m *testing.M) {
	code := m.Run()
	if sharedLDAP.container != nil {
		_ = sharedLDAP.container.Stop(context.Background())
	}
	os.Exit(code)
}

// ldapContainerEnv starts (once per test binary) an OpenLDAP testcontainer
// with the dc=test,dc=local directory the integration tests expect. Under
// -tags=integration a missing directory is a failure, not a skip: this is
// the build the CI "Integration Tests" job runs, and a skip there would
// hide the tests again.
func ldapContainerEnv(t *testing.T, addr string, dialErr error) *ldapIntegrationEnv {
	t.Helper()

	sharedLDAP.once.Do(func() {
		c, err := integration.StartOpenLDAP(context.Background(), integration.OpenLDAPConfig{
			BaseDN:       ldapTestBaseDN,
			AdminPass:    ldapTestAdminPass,
			Organization: "Test Org",
			Domain:       ldapTestDomain,
		})
		if err != nil {
			sharedLDAP.err = err
			return
		}
		sharedLDAP.container = c

		// The Docker provider reports "localhost" on native Linux; dial the
		// IPv4 loopback the mapped port is published on, as skipIfNoLDAP does.
		host := c.Host
		if host == "localhost" {
			host = "127.0.0.1"
		}
		port, err := strconv.Atoi(c.Port)
		if err != nil {
			sharedLDAP.err = err
			return
		}
		sharedLDAP.env = newLDAPIntegrationEnv(host, port)
	})

	if sharedLDAP.err != nil {
		t.Fatalf("no OpenLDAP at %s (%v) and starting the testcontainer failed: %v",
			addr, dialErr, sharedLDAP.err)
	}

	return sharedLDAP.env
}
