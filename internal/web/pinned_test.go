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
