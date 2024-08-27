package app

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"slices"
	"time"

	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketFeeds = "feeds"
)

type Item struct {
	time time.Time
	key  []byte
}

type Storage struct {
	db  *bolt.DB
	cfg MyConfig
}

func NewStorage(db *bolt.DB, cfg MyConfig) *Storage {
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
		root, err := tx.CreateBucketIfNotExists([]byte(bucketFeeds))
		if err != nil {
			return err
		}
		// Create new buckets
		for f := range feeds {
			if _, err := root.CreateBucketIfNotExists([]byte(f)); err != nil {
				return err
			}
		}
		// Delete obsolete buckets
		root.ForEach(func(k, v []byte) error {
			if name := string(k); !feeds[name] {
				if err := root.DeleteBucket(k); err != nil {
					return err
				}
				slog.Info("Deleted obsolete bucket for feed", "name", name)
			}
			return nil
		})
		return nil
	})
	return err
}

func (st *Storage) RecordItem(cf ConfigFeed, item *gofeed.Item) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		var t time.Time
		if item.PublishedParsed != nil {
			t = *item.PublishedParsed
		} else {
			t = time.Now().UTC()
		}
		return b.Put([]byte(itemUniqueID(item)), []byte(t.Format(time.RFC3339)))
	})
	return err
}

// IsItemNew reports wether an item in a feed is new
func (st *Storage) IsItemNew(cf ConfigFeed, item *gofeed.Item) bool {
	var isNew bool
	st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		v := b.Get([]byte(itemUniqueID(item)))
		if v == nil {
			isNew = true
		}
		return nil
	})
	return isNew
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

// CullFeed deletes the oldest items when there are more items then a limit
func (st *Storage) CullFeed(cf ConfigFeed, limit int) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		items := make([]Item, 0)
		b.ForEach(func(k, v []byte) error {
			t, err := time.Parse(time.RFC3339, string(v))
			if err != nil {
				return err
			}
			items = append(items, Item{time: t, key: k})
			return nil
		})
		slices.SortFunc(items, func(a Item, b Item) int {
			return a.time.Compare(b.time) * -1
		})
		if len(items) <= limit {
			return nil
		}
		for _, i := range items[limit:] {
			if err := b.Delete(i.key); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (st *Storage) ListItems(cf ConfigFeed) ([]Item, error) {
	var ii []Item
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		b.ForEach(func(k, v []byte) error {
			x, err := time.Parse(time.RFC3339, string(v))
			if err != nil {
				return err
			}
			ii = append(ii, Item{key: k, time: x})
			return nil
		})
		return nil
	})
	return ii, err
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

func (st *Storage) ItemCount(cf ConfigFeed) int {
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
