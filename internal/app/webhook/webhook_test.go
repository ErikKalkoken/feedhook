package webhook_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedforward/internal/app/webhook"
	"github.com/ErikKalkoken/feedforward/internal/queue"
	"github.com/jarcoal/httpmock"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestWebhook(t *testing.T) {
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
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com",
		httpmock.NewStringResponder(204, ""),
	)
	wh := webhook.New(http.DefaultClient, q, "dummy", "https://www.example.com")
	wh.Start()
	feed := &gofeed.Feed{Title: "title"}
	now := time.Now()
	item := &gofeed.Item{Content: "content", PublishedParsed: &now}
	err = wh.Send(feed, item)
	time.Sleep(2 * time.Second)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	}
}
