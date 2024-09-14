package webhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
	return rt.now
}

func TestWebhookRateLimit(t *testing.T) {
	now := time.Now()
	t.Run("should report false when no requests recorded", func(t *testing.T) {
		rl := newWebhookRateLimit(faketime{})
		remaining, _ := rl.calc()
		assert.Equal(t, 30, remaining)
	})
	t.Run("should report false when below limit", func(t *testing.T) {
		rl := newWebhookRateLimit(faketime{now})
		rl.s = []time.Time{now, now, now}
		remaining, _ := rl.calc()
		assert.Equal(t, 27, remaining)
	})
	t.Run("should report true when above limit", func(t *testing.T) {
		rl := newWebhookRateLimit(faketime{now})
		oldest := now.Add(-30 * time.Second)
		rl.s = append(rl.s, oldest)
		rl.s = append(rl.s, now.Add(-61*time.Second))
		for range 29 {
			rl.s = append(rl.s, now)
		}
		remaining, reset := rl.calc()
		assert.Equal(t, 0, remaining)
		assert.WithinDuration(t, oldest.Add(60*time.Second), now.Add(reset), 1*time.Second)
	})
}
