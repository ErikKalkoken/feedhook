package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"time"

	"github.com/ErikKalkoken/feedforward/internal/app"
	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

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

// CullItems deletes the oldest items when there are more items then a limit
func (st *Storage) CullItems(cf app.ConfigFeed, limit int) error {
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
