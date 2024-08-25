package app

import (
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
	return rt.now
}

func TestIsItemNew(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cf := ConfigFeed{Name: "feed1", URL: "https://www.example.com/feed", Webhook: "hook1"}
	cfg := MyConfig{
		App:   ConfigApp{Oldest: 3600 * 24, Ticker: 1},
		Feeds: []ConfigFeed{cf},
	}
	a := New(db, cfg, faketime{})
	if err := a.Init(); err != nil {
		log.Fatalf("Failed to init: %s", err)
	}
	t.Run("should report true when item has GUI and does not exit", func(t *testing.T) {
		i := &gofeed.Item{GUID: "abc1"}
		assert.True(t, a.isItemNew(cf, i))
	})
	t.Run("should report false when item has GUI and exists", func(t *testing.T) {
		i := &gofeed.Item{GUID: "abc2"}
		if err := a.recordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		assert.False(t, a.isItemNew(cf, i))
	})
	t.Run("should report true when item has no GUI and does not exit", func(t *testing.T) {
		i := &gofeed.Item{Title: "title", Description: "description"}
		assert.True(t, a.isItemNew(cf, i))
	})
	t.Run("should report false when item has no GUI and exists", func(t *testing.T) {
		i := &gofeed.Item{Title: "title", Description: "description"}
		if err := a.recordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		assert.False(t, a.isItemNew(cf, i))
	})
}
