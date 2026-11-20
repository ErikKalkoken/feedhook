package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

// GetItemState returns the state of an item.
// Returns [ErrNotFound] when the item does not exist.
func (st *Storage) GetItemState(feedName string, item *gofeed.Item) (app.ItemState, error) {
	var s app.ItemState
	if feedName == "" || item == nil {
		return s, ErrInvalid
	}
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feedName))
		if b == nil {
			return ErrInvalid
		}
		k := processedItemFromFeed(item).Key()
		v := b.Get(k)
		if v == nil {
			return ErrNotFound
		}
		if item.PublishedParsed == nil {
			s = app.StateProcessed
			return nil
		}
		pi, err := processedItemFromDB(v)
		if err != nil {
			slog.Warn("GetItemState: invalid item record, treating as new", "feed", feedName, "item", string(k), "error", err)
			return fmt.Errorf("decoding failed: %w: %w", ErrNotFound, err)
		}
		if pp := *item.PublishedParsed; pp.Equal(pi.Published) {
			s = app.StateProcessed
			return nil
		}
		s = app.StateUpdated
		return nil
	})
	return s, err
}

// PruneOldestItems deletes the oldest items when there are more items than a limit.
func (st *Storage) PruneOldestItems(feedName string, limit int) error {
	if feedName == "" || limit <= 0 {
		return ErrInvalid
	}
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feedName))
		if b == nil {
			return ErrInvalid
		}
		items := make([]*app.ProcessedItem, 0)
		var invalidItems [][]byte
		b.ForEach(func(k, v []byte) error {
			i, err := processedItemFromDB(v)
			if err != nil {
				invalidItems = append(invalidItems, k)
				return nil
			}
			items = append(items, i)
			return nil
		})
		if len(invalidItems) > 0 {
			for _, k := range invalidItems {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
			slog.Warn("PruneOldestItems: removed invalid items", "feed", feedName, "count", len(invalidItems))
		}
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

// ListItems returns a list of all items for a feed.
func (st *Storage) ListItems(feedName string) ([]*app.ProcessedItem, error) {
	if feedName == "" {
		return nil, ErrInvalid
	}
	var items []*app.ProcessedItem
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feedName))
		if b == nil {
			return ErrInvalid
		}
		err := b.ForEach(func(k, v []byte) error {
			i, err := processedItemFromDB(v)
			if err != nil {
				slog.Warn("ListItems: ignoring invalid item", "feed", feedName, "item", string(k), "error", err)
				return nil
			}
			items = append(items, i)
			return nil
		})
		return err
	})
	return items, err
}

// CountItems returns the count of items in a feed.
func (st *Storage) CountItems(feedName string) (int, error) {
	if feedName == "" {
		return 0, ErrInvalid
	}
	var c int
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feedName))
		if b == nil {
			return ErrInvalid
		}
		err := b.ForEach(func(k, v []byte) error {
			c++
			return nil
		})
		return err
	})
	if err != nil {
		return 0, err
	}
	return c, nil
}

// UpdateOrCreateItem updates or creates an item.
func (st *Storage) UpdateOrCreateItem(feedName string, item *gofeed.Item) error {
	if feedName == "" || item == nil {
		return ErrInvalid
	}
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(feedName))
		if b == nil {
			return ErrInvalid
		}
		i := processedItemFromFeed(item)
		v, err := processedItemToDB(i)
		if err != nil {
			return err
		}
		return b.Put(i.Key(), v)
	})
	return err
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

func processedItemToDB(pi *app.ProcessedItem) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(*pi); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func processedItemFromDB(byt []byte) (*app.ProcessedItem, error) {
	b := bytes.NewBuffer(byt)
	d := gob.NewDecoder(b)
	var pi app.ProcessedItem
	if err := d.Decode(&pi); err != nil {
		return &pi, err
	}
	return &pi, nil
}
