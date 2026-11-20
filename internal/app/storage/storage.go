// Package storage provides access to persistent storage.
package storage

import (
	"errors"
	"log/slog"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketFeeds    = "feeds"
	bucketStats    = "stats"
	bucketWebhooks = "webhooks"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
)

type Storage struct {
	db *bolt.DB
}

func New(db *bolt.DB) *Storage {
	st := &Storage{
		db: db,
	}
	return st
}

// Init creates all required buckets and deletes obsolete buckets.
func (st *Storage) Init(feedNames []string) error {
	feeds := make(map[string]bool)
	for _, n := range feedNames {
		feeds[n] = true
	}
	err := st.db.Update(func(tx *bolt.Tx) error {
		// feeds bucket
		bf, err := tx.CreateBucketIfNotExists([]byte(bucketFeeds))
		if err != nil {
			return err
		}
		// Create new buckets as needed
		for f := range feeds {
			if _, err := bf.CreateBucketIfNotExists([]byte(f)); err != nil {
				return err
			}
		}
		// Delete obsolete buckets
		var obsolete [][]byte
		bf.ForEach(func(k, v []byte) error {
			if !feeds[string(k)] {
				obsolete = append(obsolete, k)
			}
			return nil
		})
		for _, k := range obsolete {
			if err := bf.DeleteBucket(k); err != nil {
				return err
			}
			slog.Info("Deleted obsolete bucket for feed", "name", string(k))
		}
		// stats bucket
		bs, err := tx.CreateBucketIfNotExists([]byte(bucketStats))
		if err != nil {
			return err
		}
		if _, err := bs.CreateBucketIfNotExists([]byte(bucketFeeds)); err != nil {
			return err
		}
		if _, err := bs.CreateBucketIfNotExists([]byte(bucketWebhooks)); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (st *Storage) DB() *bolt.DB {
	return st.db
}
