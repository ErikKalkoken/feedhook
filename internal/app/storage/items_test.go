package storage_test

import (
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

func TestItems(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cf := config.ConfigFeed{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}
	cfg := config.Config{
		Feeds: []config.ConfigFeed{cf},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		log.Fatalf("Failed to init: %s", err)
	}
	t.Run("should report unknown item with GUID as new", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{GUID: "abc1"}
		s, err := st.GetItemState(cf, i)
		if assert.NoError(t, err) {
			assert.Equal(t, app.StateNew, s)
		}
	})
	t.Run("should report known item item with GUID and same publish date as processed", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		t1 := time.Now()
		i1 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t1}
		if err := st.RecordItem(cf, i1); err != nil {
			t.Fatal(err)
		}
		i2 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t1}
		s, err := st.GetItemState(cf, i2)
		if assert.NoError(t, err) {
			assert.Equal(t, app.StateProcessed, s)
		}
	})
	t.Run("should report known item with GUID and different publish data as updated", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		t1 := time.Now().Add(-5 * time.Second)
		i1 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t1}
		if err := st.RecordItem(cf, i1); err != nil {
			t.Fatal(err)
		}
		t2 := time.Now()
		i2 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t2}
		s, err := st.GetItemState(cf, i2)
		if assert.NoError(t, err) {
			assert.Equal(t, app.StateUpdated, s)
		}
	})
	t.Run("should report unknown item without GUID as new", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{Title: "title", Description: "description"}
		s, err := st.GetItemState(cf, i)
		if assert.NoError(t, err) {
			assert.Equal(t, app.StateNew, s)
		}
	})
	t.Run("should report known item without GUID as processed", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{Title: "title", Description: "description"}
		if err := st.RecordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		s, err := st.GetItemState(cf, i)
		if assert.NoError(t, err) {
			assert.Equal(t, app.StateProcessed, s)
		}
	})
	t.Run("should delete oldest items", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Add(-10 * time.Hour)
		t1 := now.Add(5 * time.Hour)
		if err := st.RecordItem(cf, &gofeed.Item{GUID: "1", PublishedParsed: &t1}); err != nil {
			t.Fatal(err)
		}
		t2 := now.Add(1 * time.Hour)
		if err := st.RecordItem(cf, &gofeed.Item{GUID: "2", PublishedParsed: &t2}); err != nil {
			t.Fatal(err)
		}
		t3 := now.Add(4 * time.Hour)
		if err := st.RecordItem(cf, &gofeed.Item{GUID: "3", PublishedParsed: &t3}); err != nil {
			t.Fatal(err)
		}
		err := st.CullItems(cf, 2)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, st.ItemCount(cf))
			ii, err := st.ListItems(cf.Name)
			if assert.NoError(t, err) {
				var ids []string
				for _, i := range ii {
					ids = append(ids, string(i.ID))
				}
				assert.Contains(t, ids, "1")
				assert.Contains(t, ids, "3")
			}
		}
	})
}
