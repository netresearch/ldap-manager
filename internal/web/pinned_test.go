// internal/web/pinned_test.go
package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func newTestPinStore(t *testing.T) *PinnedStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pinned.bbolt")
	db, err := bolt.Open(path, 0o600, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(); _ = os.Remove(path) })
	store, err := NewPinnedStore(db)
	require.NoError(t, err)
	return store
}

func TestPinnedStore_AddListRemove(t *testing.T) {
	s := newTestPinStore(t)

	user := "uid=alice,ou=Users,dc=test"
	g1 := "cn=admins,ou=Groups,dc=test"
	g2 := "cn=devs,ou=Groups,dc=test"

	got, err := s.List(user)
	require.NoError(t, err)
	assert.Empty(t, got)

	require.NoError(t, s.Add(user, g1))
	require.NoError(t, s.Add(user, g2))

	got, err = s.List(user)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{g1, g2}, got)

	pinned, err := s.IsPinned(user, g1)
	require.NoError(t, err)
	assert.True(t, pinned)

	pinned, err = s.IsPinned(user, "cn=never,dc=test")
	require.NoError(t, err)
	assert.False(t, pinned)

	require.NoError(t, s.Remove(user, g1))
	got, err = s.List(user)
	require.NoError(t, err)
	assert.Equal(t, []string{g2}, got)

	require.NoError(t, s.Remove(user, g1)) // double-remove is idempotent
}

func TestPinnedStore_PerUser(t *testing.T) {
	s := newTestPinStore(t)
	_ = s.Add("uid=alice,dc=test", "cn=x,dc=test")
	_ = s.Add("uid=bob,dc=test", "cn=y,dc=test")

	alice, _ := s.List("uid=alice,dc=test")
	bob, _ := s.List("uid=bob,dc=test")

	assert.Equal(t, []string{"cn=x,dc=test"}, alice)
	assert.Equal(t, []string{"cn=y,dc=test"}, bob)
}

func TestPinnedStore_RejectsEmpty(t *testing.T) {
	s := newTestPinStore(t)
	assert.Error(t, s.Add("", "cn=x"))
	assert.Error(t, s.Add("uid=alice", ""))
	assert.Error(t, s.Remove("", "cn=x"))
	assert.Error(t, s.Remove("uid=alice", ""))
}

// TestPinnedStorePath covers the precedence rules documented on the
// function: explicit override > session-path suffix > hardcoded
// default > disabled sentinel.
func TestPinnedStorePath(t *testing.T) {
	cases := []struct {
		name        string
		sessionPath string
		pinnedPath  string
		want        string
	}{
		{"default with no session path", "", "", "pinned.bbolt"},
		{"auto-placed next to session file", "/var/lib/app/db.bbolt", "", "/var/lib/app/db.bbolt.pinned"},
		{"explicit path overrides session auto-placement", "/var/lib/app/db.bbolt", "/data/pinned.db", "/data/pinned.db"},
		{"explicit path overrides empty session", "", "/tmp/pinned.db", "/tmp/pinned.db"},
		{"'none' disables the store", "/var/lib/app/db.bbolt", "none", ""},
		{"'off' disables the store", "", "off", ""},
		{"'disabled' disables the store", "", "disabled", ""},
		{"uppercase sentinel matches case-insensitively", "", "NONE", ""},
		{"whitespace around sentinel is trimmed", "", "  none  ", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pinnedStorePath(tc.sessionPath, tc.pinnedPath)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestPinnedStore_NilReceiver verifies the nil-safe API: handlers
// don't need to guard every call when pinning is disabled
// (--pinned-path=none or a read-only fallback at boot).
func TestPinnedStore_NilReceiver(t *testing.T) {
	var s *PinnedStore // intentionally nil

	// List returns (nil, nil) so callers can range over the result.
	got, err := s.List("uid=alice,dc=test")
	require.NoError(t, err)
	assert.Nil(t, got)

	// Add/Remove are silent no-ops.
	assert.NoError(t, s.Add("uid=alice,dc=test", "cn=x,dc=test"))
	assert.NoError(t, s.Remove("uid=alice,dc=test", "cn=x,dc=test"))

	// IsPinned returns (false, nil) so drawer guards don't branch.
	pinned, err := s.IsPinned("uid=alice,dc=test", "cn=x,dc=test")
	require.NoError(t, err)
	assert.False(t, pinned)
}

// TestPinnedStore_LongUserDN guards the fix for the 255-byte bbolt
// bucket-name ceiling. A DN assembled from many nested OUs — common in
// large AD directories — easily exceeds the limit, and a raw-DN bucket
// name would fail with bbolt.ErrBucketNameTooLong. The store hashes
// the user DN (SHA-256 hex = 64 bytes) before using it as a bucket key
// so Add/List/Remove work for any DN length.
func TestPinnedStore_LongUserDN(t *testing.T) {
	s := newTestPinStore(t)

	// Build a DN well past 255 bytes. 20 × "ou=department-N,…" clears
	// it comfortably on any reasonable encoding.
	userDN := "cn=alice"
	for i := 0; i < 20; i++ {
		userDN += ",ou=department-with-a-pretty-long-name-for-padding-" + string(rune('a'+i))
	}
	userDN += ",dc=example,dc=com"
	if len(userDN) <= 255 {
		t.Fatalf("precondition: test DN must exceed bbolt's 255-byte bucket-name limit, got %d bytes", len(userDN))
	}

	target := "cn=admins,ou=Groups,dc=example,dc=com"
	require.NoError(t, s.Add(userDN, target), "Add must succeed for a long user DN")

	got, err := s.List(userDN)
	require.NoError(t, err)
	assert.Equal(t, []string{target}, got)

	pinned, err := s.IsPinned(userDN, target)
	require.NoError(t, err)
	assert.True(t, pinned)

	require.NoError(t, s.Remove(userDN, target))

	got, err = s.List(userDN)
	require.NoError(t, err)
	assert.Empty(t, got)
}
