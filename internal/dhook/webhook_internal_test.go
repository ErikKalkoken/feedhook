package dhook

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWebhook(t *testing.T) {
	t.Run("should abort when rateLimitExceeded and not yet reset", func(t *testing.T) {
		c := NewClient(http.DefaultClient)
		wh := NewWebhook(c, "url")
		wh.rl.Set(60 * time.Second)
		err := wh.Execute(Message{Content: "content"})
		err2, _ := err.(TooManyRequestsError)
		assert.False(t, err2.Global)
	})
	t.Run("should abort when rateLimitExceeded and not yet reset", func(t *testing.T) {
		c := NewClient(http.DefaultClient)
		c.rl.Set(60 * time.Second)
		wh := NewWebhook(c, "url")
		err := wh.Execute(Message{Content: "content"})
		err2, _ := err.(TooManyRequestsError)
		assert.True(t, err2.Global)
	})
}
