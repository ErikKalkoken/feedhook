package storage_test

import (
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedforward/internal/app"
	"github.com/ErikKalkoken/feedforward/internal/app/storage"
)

func TestStorage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cf := app.ConfigFeed{Name: "feed1", URL: "https://www.example.com/feed", Webhook: "hook1"}
	cfg := app.MyConfig{
		Feeds: []app.ConfigFeed{cf},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		log.Fatalf("Failed to init: %s", err)
	}
	t.Run("should report true when item has GUI and does not exit", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{GUID: "abc1"}
		assert.True(t, st.IsItemNew(cf, i))
	})
	t.Run("should report false when item has GUI and exists", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{GUID: "abc2"}
		if err := st.RecordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		assert.False(t, st.IsItemNew(cf, i))
	})
	t.Run("should report true when item has no GUI and does not exit", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{Title: "title", Description: "description"}
		assert.True(t, st.IsItemNew(cf, i))
	})
	t.Run("should report false when item has no GUI and exists", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{Title: "title", Description: "description"}
		if err := st.RecordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		assert.False(t, st.IsItemNew(cf, i))
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

func TestStats(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cf := app.ConfigFeed{Name: "feed1", URL: "https://www.example.com/feed", Webhook: "hook1"}
	cfg := app.MyConfig{
		Feeds: []app.ConfigFeed{cf},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
	t.Run("can update and read feed stats", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		err := st.UpdateFeedStats("feed1", func(fs *app.FeedStats) error {
			fs.ReceivedCount++
			return nil
		})
		if assert.NoError(t, err) {
			got, err := st.GetFeedStats("feed1")
			if assert.NoError(t, err) {
				assert.Equal(t, "feed1", got.Name)
				assert.Equal(t, 1, got.ReceivedCount)
			}
		}
	})
	t.Run("can update and read webhook stats", func(t *testing.T) {
		if err := st.ClearFeeds(); err != nil {
			t.Fatal(err)
		}
		err := st.UpdateWebhookStats("hook1", func(fs *app.WebhookStats) error {
			fs.SentCount++
			return nil
		})
		if assert.NoError(t, err) {
			got, err := st.GetWebhookStats("hook1")
			if assert.NoError(t, err) {
				assert.Equal(t, "hook1", got.Name)
				assert.Equal(t, 1, got.SentCount)
			}
		}
	})
}
