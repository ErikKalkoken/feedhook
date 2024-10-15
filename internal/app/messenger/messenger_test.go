package messenger_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/messenger"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/dhooks"
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/jarcoal/httpmock"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestMessenger(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	q, err := pqueue.New(db, "test")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	st := storage.New(db, config.Config{})
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	t.Run("can submit messages", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com",
			httpmock.NewStringResponder(204, ""),
		)
		c := dhooks.NewClient(http.DefaultClient)
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		mg.Start()
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		err = mg.AddMessage("dummy", feed, item, false)
		time.Sleep(2 * time.Second)
		if assert.NoError(t, err) {
			assert.Equal(t, 1, httpmock.GetTotalCallCount())
		}
		ws, err := st.GetWebhookStats("dummy")
		if assert.NoError(t, err) {
			assert.Equal(t, 1, ws.SentCount)
		}
	})
	t.Run("can submit messages 2", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com",
			httpmock.NewStringResponder(204, ""),
		)
		c := dhooks.NewClient(http.DefaultClient)
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		mg.Start()
		time.Sleep(100 * time.Millisecond)
		mg.Close()
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})
}
