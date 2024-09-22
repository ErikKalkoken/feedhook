package dhooks

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
		rl, err := newRateLimitInfo(header)
		if assert.NoError(t, err) {
			assert.Equal(t, 5, rl.limit)
			assert.Equal(t, 1, rl.remaining)
			assert.Equal(t, time.Date(2016, 8, 2, 21, 23, 43, 0, time.UTC), rl.resetAt)
			assert.Equal(t, 1.2, rl.resetAfter)
			assert.Equal(t, "abcd1234", rl.bucket)
		}
	})
	t.Run("should return empty rate limit if header is incomplete", func(t *testing.T) {
		header := http.Header{}
		rl, err := newRateLimitInfo(header)
		if assert.NoError(t, err) {
			assert.True(t, rl.resetAt.IsZero())
		}
	})
}
func TestRateLimitLimitExceeded(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		rl   rateLimitInfo
		want bool
	}{
		{rateLimitInfo{}, false},
		{rateLimitInfo{timestamp: now, remaining: 1}, false},
		{rateLimitInfo{timestamp: now, remaining: 0, resetAt: now.Add(-5 * time.Second)}, false},
		{rateLimitInfo{timestamp: now, remaining: 0, resetAt: now.Add(5 * time.Second)}, true},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case %d", i+1), func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rl.limitExceeded(now))
		})
	}
}

func TestUpdateFromHeader(t *testing.T) {
	t.Run("should decrease remaining if header is about same period and bucket", func(t *testing.T) {
		l := limiterAPI{rl: rateLimitInfo{remaining: 2, resetAt: time.Unix(1470173023, 0).UTC(), bucket: "abcd1234"}}
		header := http.Header{}
		header.Set("X-RateLimit-Limit", "5")
		header.Set("X-RateLimit-Remaining", "3")
		header.Set("X-RateLimit-Reset", "1470173023")
		header.Set("X-RateLimit-Reset-After", "1.2")
		header.Set("X-RateLimit-Bucket", "abcd1234")
		l.UpdateFromHeader(header)
		assert.Equal(t, 1, l.rl.remaining)
	})
	t.Run("should update when header is about new period and same bucket", func(t *testing.T) {
		l := limiterAPI{rl: rateLimitInfo{remaining: 2, resetAt: time.Unix(1470173022, 0).UTC(), bucket: "abcd1234"}}
		header := http.Header{}
		header.Set("X-RateLimit-Limit", "5")
		header.Set("X-RateLimit-Remaining", "4")
		header.Set("X-RateLimit-Reset", "1470173023")
		header.Set("X-RateLimit-Reset-After", "1.2")
		header.Set("X-RateLimit-Bucket", "abcd1234")
		l.UpdateFromHeader(header)
		assert.Equal(t, 4, l.rl.remaining)
	})
	t.Run("should update when header is about same period and different bucket", func(t *testing.T) {
		l := limiterAPI{rl: rateLimitInfo{remaining: 2, resetAt: time.Unix(1470173022, 0).UTC(), bucket: "abcd1234"}}
		header := http.Header{}
		header.Set("X-RateLimit-Limit", "5")
		header.Set("X-RateLimit-Remaining", "4")
		header.Set("X-RateLimit-Reset", "1470173022")
		header.Set("X-RateLimit-Reset-After", "1.2")
		header.Set("X-RateLimit-Bucket", "abcd9234")
		l.UpdateFromHeader(header)
		assert.Equal(t, 4, l.rl.remaining)
	})
}
