package messenger_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/messenger"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/dhooks"
	"github.com/ErikKalkoken/feedhook/internal/queue"
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
	q, err := queue.New(db, "test")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	st := storage.New(db, app.MyConfig{})
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com",
		httpmock.NewStringResponder(204, ""),
	)
	c := dhooks.NewClient(http.DefaultClient)
	wh := messenger.New(c, q, "dummy", "https://www.example.com", st, app.MyConfig{})
	wh.Start()
	feed := &gofeed.Feed{Title: "title"}
	now := time.Now()
	item := &gofeed.Item{Content: "content", PublishedParsed: &now}
	err = wh.AddMessage("dummy", feed, item, false)
	time.Sleep(2 * time.Second)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	}
	ws, err := st.GetWebhookStats("dummy")
	if assert.NoError(t, err) {
		assert.Equal(t, 1, ws.SentCount)
	}
}
