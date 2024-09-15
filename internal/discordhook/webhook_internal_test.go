package discordhook

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookUpdateRateLimit(t *testing.T) {
	t.Run("should always decrease remaining", func(t *testing.T) {
		wh := Webhook{}
		wh.arl.remaining = 1
		wh.updateAPIRateLimit(http.Header{})
		assert.Equal(t, 0, wh.arl.remaining)
	})
	t.Run("should not update when header is about same period", func(t *testing.T) {
		wh := Webhook{}
		header := http.Header{}
		header.Set("X-RateLimit-Limit", "5")
		header.Set("X-RateLimit-Remaining", "3")
		header.Set("X-RateLimit-Reset", "1470173023")
		header.Set("X-RateLimit-Reset-After", "1.2")
		header.Set("X-RateLimit-Bucket", "abcd1234")
		wh.arl, _ = rateLimitFromHeader(header)
		wh.updateAPIRateLimit(header)
		assert.Equal(t, 2, wh.arl.remaining)
	})
	t.Run("should update when header is about new period", func(t *testing.T) {
		wh := Webhook{}
		header := http.Header{}
		header.Set("X-RateLimit-Limit", "5")
		header.Set("X-RateLimit-Remaining", "3")
		header.Set("X-RateLimit-Reset", "1470173023")
		header.Set("X-RateLimit-Reset-After", "1.2")
		header.Set("X-RateLimit-Bucket", "abcd1234")
		wh.arl, _ = rateLimitFromHeader(header)
		header.Set("X-RateLimit-Remaining", "4")
		header.Set("X-RateLimit-Reset", "1470173024")
		wh.updateAPIRateLimit(header)
		assert.Equal(t, 4, wh.arl.remaining)
	})
}
