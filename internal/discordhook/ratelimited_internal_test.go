package discordhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimited(t *testing.T) {
	t.Run("should set", func(t *testing.T) {
		var rl rateLimited
		rl.set(5 * time.Minute)
		ok, d := rl.getOrReset()
		assert.True(t, ok)
		now := time.Now()
		assert.WithinDuration(t, now.Add(5*time.Minute), now.Add(d), 1*time.Second)
	})
	t.Run("should reset when expired", func(t *testing.T) {
		var rl rateLimited
		rl.resetAt = time.Now().Add(-1 * time.Second)
		ok, _ := rl.getOrReset()
		assert.False(t, ok)
		assert.Equal(t, time.Time{}, rl.resetAt)
	})
	t.Run("should report empty as not active", func(t *testing.T) {
		var rl rateLimited
		ok, _ := rl.getOrReset()
		assert.False(t, ok)
	})
}
