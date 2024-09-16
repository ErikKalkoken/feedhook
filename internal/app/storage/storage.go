package storage

import (
	"errors"
	"log/slog"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
)

const (
	bucketFeeds    = "feeds"
	bucketStats    = "stats"
	bucketWebhooks = "webhooks"
)

var ErrNotFound = errors.New("not found")

type Storage struct {
	db  *bolt.DB
	cfg app.MyConfig
}

func New(db *bolt.DB, cfg app.MyConfig) *Storage {
	st := &Storage{
		db:  db,
		cfg: cfg,
	}
	return st
}

// Init creates all required buckets and deletes obsolete buckets.
func (st *Storage) Init() error {
	feeds := make(map[string]bool)
	for _, f := range st.cfg.Feeds {
		feeds[f.Name] = true
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
		bf.ForEach(func(k, v []byte) error {
			if name := string(k); !feeds[name] {
				if err := bf.DeleteBucket(k); err != nil {
					return err
				}
				slog.Info("Deleted obsolete bucket for feed", "name", name)
			}
			return nil
		})
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
