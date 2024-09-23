package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

func (st *Storage) RecordItem(cf config.ConfigFeed, item *gofeed.Item) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		i := processedItemFromFeed(item)
		v, err := i.ToBytes()
		if err != nil {
			return err
		}
		return b.Put(i.Key(), v)
	})
	return err
}

// GetItemState return the state of an item.
func (st *Storage) GetItemState(cf config.ConfigFeed, item *gofeed.Item) (app.ItemState, error) {
	var s app.ItemState
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		k := processedItemFromFeed(item).Key()
		v := b.Get(k)
		if v == nil {
			s = app.StateNew
			return nil
		}
		if item.PublishedParsed == nil {
			s = app.StateProcessed
			return nil
		}
		pi, err := app.NewProcessedItemFromBytes(v)
		if err != nil {
			return err
		}
		if pp := *item.PublishedParsed; pp.UTC() == pi.Published.UTC() {
			s = app.StateProcessed
			return nil
		}
		s = app.StateUpdated
		return nil
	})
	return s, err
}

// CullItems deletes the oldest items when there are more items then a limit
func (st *Storage) CullItems(cf config.ConfigFeed, limit int) error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		items := make([]*app.ProcessedItem, 0)
		b.ForEach(func(k, v []byte) error {
			i, err := app.NewProcessedItemFromBytes(v)
			if err != nil {
				return err
			}
			items = append(items, i)
			return nil
		})
		slices.SortFunc(items, func(a *app.ProcessedItem, b *app.ProcessedItem) int {
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

func (st *Storage) ListItems(feed string) ([]*app.ProcessedItem, error) {
	var items []*app.ProcessedItem
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feed))
		b.ForEach(func(k, v []byte) error {
			i, err := app.NewProcessedItemFromBytes(v)
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

func (st *Storage) ItemCount(cf config.ConfigFeed) int {
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

func processedItemFromFeed(item *gofeed.Item) *app.ProcessedItem {
	var t time.Time
	if item.PublishedParsed != nil {
		t = *item.PublishedParsed
	} else {
		t = time.Now().UTC()
	}
	return &app.ProcessedItem{ID: itemUniqueID(item), Published: t}
}

// itemUniqueID returns the unique ID for a feed item.
// This is the GUID when provided or otherwise a hash of the item's content.
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
