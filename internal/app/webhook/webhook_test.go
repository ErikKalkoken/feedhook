package webhook_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/ErikKalkoken/feedforward/internal/app/webhook"
	"github.com/jarcoal/httpmock"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestWebhook(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com",
		httpmock.NewStringResponder(204, ""),
	)
	wh := webhook.New(http.DefaultClient, "https://www.example.com")
	wh.Start()
	feed := &gofeed.Feed{Title: "title"}
	now := time.Now()
	item := &gofeed.Item{Content: "content", PublishedParsed: &now}
	err := wh.Send(feed, item)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	}
}
