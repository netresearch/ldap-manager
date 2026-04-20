// internal/web/pinned.go
package web

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// pinnedBucketName is the top-level bucket. Each user gets a nested
// sub-bucket keyed by their DN, inside which target-DN bytes map to an
// ISO-8601 creation timestamp.
var pinnedBucketName = []byte("pinned")

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
func (s *PinnedStore) List(userDN string) ([]string, error) {
	if userDN == "" {
		return nil, errors.New("pinned: empty user DN")
	}
	var out []string
	err := s.db.View(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket([]byte(userDN))
		if sub == nil {
			return nil
		}
		return sub.ForEach(func(k, _ []byte) error {
			out = append(out, string(bytes.Clone(k)))
			return nil
		})
	})
	return out, err
}

// Add records a pin. Idempotent: re-adding updates the timestamp.
func (s *PinnedStore) Add(userDN, targetDN string) error {
	if userDN == "" || targetDN == "" {
		return errors.New("pinned: empty user or target DN")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return s.db.Update(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return errors.New("pinned: top bucket missing")
		}
		sub, err := top.CreateBucketIfNotExists([]byte(userDN))
		if err != nil {
			return fmt.Errorf("pinned: create user bucket: %w", err)
		}
		return sub.Put([]byte(targetDN), []byte(now))
	})
}

// Remove deletes a pin. No error if the pin doesn't exist.
func (s *PinnedStore) Remove(userDN, targetDN string) error {
	if userDN == "" || targetDN == "" {
		return errors.New("pinned: empty user or target DN")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket([]byte(userDN))
		if sub == nil {
			return nil
		}
		return sub.Delete([]byte(targetDN))
	})
}

// IsPinned returns true iff (userDN, targetDN) exists in the store.
func (s *PinnedStore) IsPinned(userDN, targetDN string) (bool, error) {
	if userDN == "" || targetDN == "" {
		return false, errors.New("pinned: empty user or target DN")
	}
	var pinned bool
	err := s.db.View(func(tx *bolt.Tx) error {
		top := tx.Bucket(pinnedBucketName)
		if top == nil {
			return nil
		}
		sub := top.Bucket([]byte(userDN))
		if sub == nil {
			return nil
		}
		pinned = sub.Get([]byte(targetDN)) != nil
		return nil
	})
	return pinned, err
}
