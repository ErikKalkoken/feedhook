package storage

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/ErikKalkoken/feedhook/internal/app"
	bolt "go.etcd.io/bbolt"
)

// UpdateOrCreateFeedStats executes a function that updates or creates the stats for feed by name.
func (st *Storage) UpdateOrCreateFeedStats(name string, f func(fs *app.FeedStats) error) error {
	if name == "" {
		return ErrInvalid
	}
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketFeeds))
		var fs *app.FeedStats
		v := b.Get([]byte(name))
		var err error
		if v != nil {
			fs, err = feedStatsFromDB(v)
			if err != nil {
				slog.Warn("UpdateOrCreateFeedStats: invalid stats record, starting fresh", "feed", name, "error", err)
				fs = nil
			}
		}
		if fs == nil {
			fs = &app.FeedStats{Name: name}
		}
		if err := f(fs); err != nil {
			return err
		}
		v, err = feedStatsToDB(fs)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), v)
	})
	return err
}

// GetFeedStats returns the stats for a feed.
// It returns [ErrNotFound] when the feed does not exist.
func (st *Storage) GetFeedStats(name string) (*app.FeedStats, error) {
	if name == "" {
		return nil, ErrInvalid
	}
	fs := &app.FeedStats{Name: name}
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketFeeds))
		v := b.Get([]byte(name))
		if v == nil {
			return ErrNotFound
		}
		var err error
		fs, err = feedStatsFromDB(v)
		if err != nil {
			slog.Warn("GetFeedStats: invalid stats record, treating as not found", "feed", name, "error", err)
			return fmt.Errorf("decoding failed: %w: %w", ErrNotFound, err)
		}
		return nil
	})
	return fs, err
}

func feedStatsFromDB(v []byte) (*app.FeedStats, error) {
	buf := bytes.NewBuffer(v)
	dec := gob.NewDecoder(buf)
	var o app.FeedStats
	if err := dec.Decode(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

func feedStatsToDB(fs *app.FeedStats) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(*fs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
