package storage_test

import (
	"log"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

func TestFeedStats(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cf := app.ConfigFeed{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}
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
