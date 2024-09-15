package storage

import (
	"bytes"
	"encoding/gob"

	"github.com/ErikKalkoken/feedforward/internal/app"
	bolt "go.etcd.io/bbolt"
)

// UpdateFeedStats executes a function that updates the stats for feed by name.
func (st *Storage) UpdateFeedStats(name string, f func(fs *app.FeedStats) error) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketFeeds))
		var fs *app.FeedStats
		v := b.Get([]byte(name))
		var err error
		if v != nil {
			fs, err = feedStatsFromDB(v)
			if err != nil {
				return err
			}
		} else {
			fs = &app.FeedStats{Name: name}
		}
		if err := f(fs); err != nil {
			return err
		}
		v, err = dbFromFeedStats(fs)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), v)
	})
	return err
}

// GetFeedStats returns the stats for a feed.
func (st *Storage) GetFeedStats(name string) (*app.FeedStats, error) {
	fs := &app.FeedStats{Name: name}
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketFeeds))
		v := b.Get([]byte(name))
		if v == nil {
			return nil
		}
		var err error
		fs, err = feedStatsFromDB(v)
		if err != nil {
			return err
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

func dbFromFeedStats(fs *app.FeedStats) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(*fs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
