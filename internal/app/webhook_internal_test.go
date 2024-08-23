package app

import (
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestSendToWebhook(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com",
		httpmock.NewStringResponder(204, ""),
	)
	p := webhookPayload{
		Content: "contents",
	}
	err := sendToWebhook(&p, "https://www.example.com", 30*time.Second)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	}
}

func TestMakePayload(t *testing.T) {
	feed := gofeed.Feed{Title: "title"}
	now := time.Now()
	item := gofeed.Item{Content: "content", PublishedParsed: &now}
	x, err := makePayload(&feed, &item)
	if assert.NoError(t, err) {
		em := x.Embeds[0]
		assert.Equal(t, "content", em.Description)
	}
}
