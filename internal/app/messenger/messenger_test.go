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
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/ErikKalkoken/go-dhook"
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
	c := dhook.NewClient(http.DefaultClient)
	t.Run("can return name", func(t *testing.T) {
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		assert.Equal(t, "dummy", mg.Name())

	})
	t.Run("can submit messages", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com",
			httpmock.NewStringResponder(204, ""),
		)
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
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
		mg.Shutdown()
	})
	t.Run("can submit messages 2", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
		mg.Shutdown()
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})
	t.Run("should return error when try to start twice", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		err := mg.Start()
		assert.Error(t, err)
		mg.Shutdown()
	})
	t.Run("should report if shutdown was done", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		ok := mg.Shutdown()
		assert.True(t, ok)
		ok = mg.Shutdown()
		assert.False(t, ok)
	})
}
