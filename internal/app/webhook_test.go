package app

import (
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestSendToWebhook(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com",
		httpmock.NewStringResponder(204, ""))
	p := webhookPayload{
		Content: "contents",
	}
	err := sendToWebhook(&p, "https://www.example.com", 30*time.Second)
	if assert.NoError(t, err) {
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	}
}
