package webhook

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	t.Run("should extract rate limit from header", func(t *testing.T) {
		header := http.Header{}
		header.Set("X-RateLimit-Limit", "5")
		header.Set("X-RateLimit-Remaining", "1")
		header.Set("X-RateLimit-Reset", "1470173023")
		header.Set("X-RateLimit-Reset-After", "1.2")
		header.Set("X-RateLimit-Bucket", "abcd1234")
		rl, err := rateLimitFromHeader(header)
		if assert.NoError(t, err) {
			assert.Equal(t, 5, rl.limit)
			assert.Equal(t, 1, rl.remaining)
			assert.Equal(t, time.Date(2016, 8, 2, 21, 23, 43, 0, time.UTC), rl.reset)
			assert.Equal(t, 1.2, rl.resetAfter)
			assert.Equal(t, "abcd1234", rl.bucket)
		}
	})
	t.Run("should return empty rate limit if header is incomplete", func(t *testing.T) {
		header := http.Header{}
		rl, err := rateLimitFromHeader(header)
		if assert.NoError(t, err) {
			assert.True(t, rl.reset.IsZero())
		}
	})
}
func TestRateLimitWait(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		rl   apiRateLimit
		want bool
	}{
		{apiRateLimit{}, false},
		{apiRateLimit{timestamp: now, remaining: 1}, false},
		{apiRateLimit{timestamp: now, remaining: 0, reset: now.Add(-5 * time.Second)}, false},
		{apiRateLimit{timestamp: now, remaining: 0, reset: now.Add(5 * time.Second)}, true},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case %d", i+1), func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rl.limitExceeded(now))
		})
	}
}

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
