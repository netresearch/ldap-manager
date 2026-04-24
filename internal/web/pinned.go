// internal/web/pinned.go
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// pinnedBucketName is the top-level bucket. Each user gets a nested
// sub-bucket keyed by a SHA-256 hash of their DN, inside which
// target-DN bytes map to an ISO-8601 creation timestamp.
//
// Why hash the user DN rather than use it raw: bbolt bucket names are
// capped at 255 bytes, and real-world AD DNs in deeply nested OUs can
// comfortably exceed that. A raw-DN bucket call returns
// bbolt.ErrBucketNameTooLong for those users, silently breaking pinning
// for exactly the accounts most likely to exist in a large directory.
// The 64-char hex SHA-256 is fixed-length, collision-resistant for the
// population we care about, and keeps the bucket keyspace compact.
//
// A secondary reverse-lookup (hash → DN) is not required: callers
// already supply userDN on every call, so the hash is computed on the
// fly and we never need to materialise a DN from a bucket name.
var pinnedBucketName = []byte("pinned")

// userBucketKey derives a fixed-length bbolt bucket key from a raw
// user DN. See the doc comment on pinnedBucketName for the rationale.
func userBucketKey(userDN string) []byte {
	sum := sha256.Sum256([]byte(userDN))
	// 64-byte hex is well under bbolt's 255-byte bucket-name limit.
	dst := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(dst, sum[:])

	return dst
}

// PinnedStore is a per-user pinned-items store backed by bbolt.
type PinnedStore struct {
	db *bolt.DB
}

// NewPinnedStore ensures the top bucket exists and returns a ready store.
func NewPinnedStore(db *bolt.DB) (*PinnedStore, error) {
	if db == nil {
		return nil, errors.New("pinned store: db is nil")
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(pinnedBucketName)

		return err
	}); err != nil {
		return nil, fmt.Errorf("pinned store: init bucket: %w", err)
	}

	return &PinnedStore{db: db}, nil
}

// List returns the target DNs pinned by the given user.
//
// Nil-safe: when PinnedStore is nil (pinning disabled — see
// --pinned-path=none or read-only filesystem fallback at boot) List
// returns an empty slice and no error so the caller can treat pinning
// as a universally-available feature without conditional branches.
func (s *PinnedStore) List(userDN string) ([]string, error) {
	if s == nil {
		return nil, nil
	}
	if userDN == "" {
		return nil, errors.New("pinned: empty user DN")
	}
	var out []string
	err := s.db.View(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket(userBucketKey(userDN))
		if sub == nil {
			return nil
		}

		return sub.ForEach(func(k, _ []byte) error {
			// string(k) already performs a copy of the underlying
			// byte slice (which bbolt invalidates after the
			// transaction ends); bytes.Clone would be a redundant
			// second copy.
			out = append(out, string(k))

			return nil
		})
	})

	return out, err
}

// Add records a pin. Idempotent: re-adding updates the timestamp.
// Nil-safe: a nil receiver is a silent no-op (see List).
func (s *PinnedStore) Add(userDN, targetDN string) error {
	if s == nil {
		return nil
	}
	if userDN == "" || targetDN == "" {
		return errors.New("pinned: empty user or target DN")
	}
	now := time.Now().UTC().Format(time.RFC3339)

	return s.db.Update(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return errors.New("pinned: top bucket missing")
		}
		sub, err := top.CreateBucketIfNotExists(userBucketKey(userDN))
		if err != nil {
			return fmt.Errorf("pinned: create user bucket: %w", err)
		}

		return sub.Put([]byte(targetDN), []byte(now))
	})
}

// Remove deletes a pin. No error if the pin doesn't exist.
// Nil-safe: a nil receiver is a silent no-op (see List).
func (s *PinnedStore) Remove(userDN, targetDN string) error {
	if s == nil {
		return nil
	}
	if userDN == "" || targetDN == "" {
		return errors.New("pinned: empty user or target DN")
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket(userBucketKey(userDN))
		if sub == nil {
			return nil
		}

		return sub.Delete([]byte(targetDN))
	})
}

// IsPinned returns true iff (userDN, targetDN) exists in the store.
// Nil-safe: a nil receiver returns (false, nil).
func (s *PinnedStore) IsPinned(userDN, targetDN string) (bool, error) {
	if s == nil {
		return false, nil
	}
	if userDN == "" || targetDN == "" {
		return false, errors.New("pinned: empty user or target DN")
	}
	var pinned bool
	err := s.db.View(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket(userBucketKey(userDN))
		if sub == nil {
			return nil
		}
		pinned = sub.Get([]byte(targetDN)) != nil

		return nil
	})

	return pinned, err
}
