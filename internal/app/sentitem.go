package app

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/mmcdole/gofeed"
)

// SentItem represents a sent item
type SentItem struct {
	ID        string
	Published time.Time
}

func (si *SentItem) Key() []byte {
	return []byte(si.ID)
}

func (si *SentItem) Value() []byte {
	return []byte(si.Published.Format(time.RFC3339))
}

func sentItemFromDB(k, v []byte) (*SentItem, error) {
	t, err := time.Parse(time.RFC3339, string(v))
	if err != nil {
		return nil, err
	}
	return &SentItem{ID: string(k), Published: t}, nil
}
func sentItemFromFeed(item *gofeed.Item) *SentItem {
	var t time.Time
	if item.PublishedParsed != nil {
		t = *item.PublishedParsed
	} else {
		t = time.Now().UTC()
	}
	return &SentItem{ID: itemUniqueID(item), Published: t}
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
