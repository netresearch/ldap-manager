package options

import (
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// poolOpts builds Opts with a service account and pool values that differ
// from every library default, so a field that is not mapped cannot pass by
// coincidence.
func poolOpts() *Opts {
	return &Opts{
		ReadonlyUser:            "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:        "readonly", // pragma: allowlist secret
		PoolMaxConnections:      17,
		PoolMinConnections:      3,
		PoolMaxIdleTime:         7 * time.Minute,
		PoolHealthCheckInterval: 41 * time.Second,
		PoolConnectionTimeout:   13 * time.Second,
		PoolAcquireTimeout:      9 * time.Second,
	}
}

// TestLDAPPoolConfig_MapsEveryExposedSetting is issue #677: the LDAP_POOL_*
// values were parsed and read by nobody. Each exposed setting must reach the
// field of ldap.PoolConfig that the library actually reads.
func TestLDAPPoolConfig_MapsEveryExposedSetting(t *testing.T) {
	pool := poolOpts().LDAPPoolConfig()
	require.NotNil(t, pool, "a service account must get a pool")

	assert.Equal(t, 17, pool.MaxConnections)
	assert.Equal(t, 3, pool.MinConnections)
	assert.Equal(t, 7*time.Minute, pool.MaxIdleTime)
	assert.Equal(t, 41*time.Second, pool.HealthCheckInterval)
	assert.Equal(t, 13*time.Second, pool.ConnectionTimeout)
	assert.Equal(t, 9*time.Second, pool.GetTimeout, "LDAP_POOL_ACQUIRE_TIMEOUT is the library's GetTimeout")
}

// TestLDAPPoolConfig_KeepsTheLibrarysLeakDetection pins why the mapping starts
// from DefaultPoolConfig. The library defaults zero durations but not
// EnableSelfHealing or the leak thresholds, so a struct literal would switch
// leak detection off with no error and no log line.
func TestLDAPPoolConfig_KeepsTheLibrarysLeakDetection(t *testing.T) {
	pool := poolOpts().LDAPPoolConfig()
	require.NotNil(t, pool)

	want := ldap.DefaultPoolConfig()
	assert.True(t, pool.EnableSelfHealing, "self-healing must stay on")
	assert.Equal(t, want.LeakDetectionThreshold, pool.LeakDetectionThreshold)
	assert.Equal(t, want.LeakEvictionThreshold, pool.LeakEvictionThreshold)
	assert.NotZero(t, pool.LeakDetectionThreshold)
}

// TestLDAPPoolConfig_NilWithoutServiceAccount covers the per-user mode: with
// no service account there is no long-lived client, so there is nothing to
// pool. Either half of the credential missing counts as absent, matching the
// check NewApp uses to decide whether to build the client at all.
func TestLDAPPoolConfig_NilWithoutServiceAccount(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
	}{
		{"neither", "", ""},
		{"user only", "cn=readonly,dc=test,dc=local", ""},
		{"password only", "", "readonly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := poolOpts()
			o.ReadonlyUser = tt.user
			o.ReadonlyPassword = tt.password // pragma: allowlist secret
			assert.Nil(t, o.LDAPPoolConfig())
		})
	}
}

// TestLDAPPoolConfig_ReturnsAFreshValue guards against handing out a shared
// pointer: simple-ldap-go copies the config it is given, but a caller of this
// method mutating the result must not change what the next caller receives.
func TestLDAPPoolConfig_ReturnsAFreshValue(t *testing.T) {
	o := poolOpts()
	first := o.LDAPPoolConfig()
	first.MaxConnections = 99

	assert.Equal(t, 17, o.LDAPPoolConfig().MaxConnections)
}
