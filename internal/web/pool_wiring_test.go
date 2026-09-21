package web

import (
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netresearch/ldap-manager/internal/options"
)

// TestServiceAccountLDAPConfig_CarriesThePool is the wiring half of issue
// #677: the service account client is the one built from this config, so the
// pool has to be on it.
func TestServiceAccountLDAPConfig_CarriesThePool(t *testing.T) {
	opts := &options.Opts{
		LDAP:               ldap.Config{Server: unreachableLDAPServer, BaseDN: "dc=test,dc=local"},
		ReadonlyUser:       "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:   "readonly", // pragma: allowlist secret
		PoolMaxConnections: 5,
		PoolMinConnections: 1,
		PoolMaxIdleTime:    time.Minute,
	}

	cfg := serviceAccountLDAPConfig(opts)

	require.NotNil(t, cfg.Pool, "the service account client must be pooled")
	assert.Equal(t, 5, cfg.Pool.MaxConnections)
	assert.Equal(t, 1, cfg.Pool.MinConnections)
	assert.Equal(t, opts.LDAP.Server, cfg.Server, "the shared settings must carry over")
	assert.Equal(t, opts.LDAP.BaseDN, cfg.BaseDN)
}

// TestServiceAccountLDAPConfig_LeavesTheSharedConfigUnpooled is the other
// direction, and the one the issue's own suggested fix would have broken:
// opts.LDAP is also the template for the per-request clients built from a
// logged-in user's credentials. A pool there would warm MinConnections
// connections on every login and every request.
func TestServiceAccountLDAPConfig_LeavesTheSharedConfigUnpooled(t *testing.T) {
	opts := &options.Opts{
		LDAP:               ldap.Config{Server: unreachableLDAPServer, BaseDN: "dc=test,dc=local"},
		ReadonlyUser:       "cn=readonly,dc=test,dc=local",
		ReadonlyPassword:   "readonly", // pragma: allowlist secret
		PoolMaxConnections: 5,
	}

	_ = serviceAccountLDAPConfig(opts)

	assert.Nil(t, opts.LDAP.Pool, "the per-request template must stay unpooled")
}

// TestPoolIsHealthy covers the predicate both health endpoints use. The
// MinConnections-0 row is the case the library's changelog warns about: an
// idle pool drains to zero and a healthy client would read as down.
func TestPoolIsHealthy(t *testing.T) {
	tests := []struct {
		name  string
		stats ldap.PerformanceStats
		want  bool
	}{
		{
			name:  "no pool",
			stats: ldap.PerformanceStats{},
			want:  false,
		},
		{
			name: "pool with connections",
			stats: ldap.PerformanceStats{
				TotalConnections: 2,
				PoolStats:        &ldap.ConnectionPoolStats{MinConnections: 2},
			},
			want: true,
		},
		{
			name: "pool drained below its minimum",
			stats: ldap.PerformanceStats{
				TotalConnections: 0,
				PoolStats:        &ldap.ConnectionPoolStats{MinConnections: 2},
			},
			want: false,
		},
		{
			name: "idle pool with no minimum",
			stats: ldap.PerformanceStats{
				TotalConnections: 0,
				PoolStats:        &ldap.ConnectionPoolStats{MinConnections: 0},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, poolIsHealthy(tt.stats))
		})
	}
}
