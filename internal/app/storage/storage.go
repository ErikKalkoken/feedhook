package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedforward/internal/app"
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

func (st *Storage) RecordItem(cf app.ConfigFeed, item *gofeed.Item) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		i := sentItemFromFeed(item)
		return b.Put(i.Key(), i.Value())
	})
	return err
}

// IsItemNew reports wether an item in a feed is new
func (st *Storage) IsItemNew(cf app.ConfigFeed, item *gofeed.Item) bool {
	var isNew bool
	st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		v := b.Get(sentItemFromFeed(item).Key())
		if v == nil {
			isNew = true
		}
		return nil
	})
	return isNew
}

// CullFeed deletes the oldest items when there are more items then a limit
func (st *Storage) CullFeed(cf app.ConfigFeed, limit int) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		items := make([]*app.SentItem, 0)
		b.ForEach(func(k, v []byte) error {
			i, err := sentItemFromDB(k, v)
			if err != nil {
				return err
			}
			items = append(items, i)
			return nil
		})
		slices.SortFunc(items, func(a *app.SentItem, b *app.SentItem) int {
			return a.Published.Compare(b.Published) * -1
		})
		if len(items) <= limit {
			return nil
		}
		for _, i := range items[limit:] {
			if err := b.Delete(i.Key()); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (st *Storage) ListItems(feed string) ([]*app.SentItem, error) {
	var items []*app.SentItem
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feed))
		b.ForEach(func(k, v []byte) error {
			i, err := sentItemFromDB(k, v)
			if err != nil {
				return err
			}
			items = append(items, i)
			return nil
		})
		return nil
	})
	return items, err
}

func (st *Storage) ListFeeds() ([]string, error) {
	var nn []string
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		root.ForEachBucket(func(k []byte) error {
			nn = append(nn, string(k))
			return nil
		})
		return nil
	})
	return nn, err
}

func (st *Storage) ClearFeeds() error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		root.ForEachBucket(func(k []byte) error {
			b := root.Bucket(k)
			b.ForEach(func(k, v []byte) error {
				if err := b.Delete(k); err != nil {
					return err
				}
				return nil
			})
			return nil
		})
		return nil
	})
	return err
}

func (st *Storage) ItemCount(cf app.ConfigFeed) int {
	var c int
	st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		b.ForEach(func(k, v []byte) error {
			c++
			return nil
		})
		return nil
	})
	return c
}

func (st *Storage) UpdateFeedStats(name string) error {
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
		fs.SentCount++
		fs.SentLast = time.Now().UTC()
		v, err = dbFromFeedStats(fs)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), v)
	})
	return err
}

func (st *Storage) ReadFeedStats(name string) (*app.FeedStats, error) {
	var fs *app.FeedStats
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

func sentItemFromDB(k, v []byte) (*app.SentItem, error) {
	t, err := time.Parse(time.RFC3339, string(v))
	if err != nil {
		return nil, err
	}
	return &app.SentItem{ID: string(k), Published: t}, nil
}
func sentItemFromFeed(item *gofeed.Item) *app.SentItem {
	var t time.Time
	if item.PublishedParsed != nil {
		t = *item.PublishedParsed
	} else {
		t = time.Now().UTC()
	}
	return &app.SentItem{ID: itemUniqueID(item), Published: t}
}

// itemUniqueID returns a unique ID of an item.
func itemUniqueID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	s := item.Title + item.Description + item.Content
	return makeHash(s)
}

func makeHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
