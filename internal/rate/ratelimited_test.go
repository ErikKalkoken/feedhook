package rate_test

import (
	"testing"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/rate"
	"github.com/stretchr/testify/assert"
)

func TestRateLimited(t *testing.T) {
	t.Run("should set", func(t *testing.T) {
		var rl rate.RateLimited
		rl.Set(5 * time.Minute)
		ok, d := rl.GetOrReset()
		assert.True(t, ok)
		now := time.Now()
		assert.WithinDuration(t, now.Add(5*time.Minute), now.Add(d), 1*time.Second)
	})
	t.Run("should return false when expired", func(t *testing.T) {
		var rl rate.RateLimited
		rl.Set(-1 * time.Second)
		ok, _ := rl.GetOrReset()
		assert.False(t, ok)
	})
	t.Run("should report zero-value as not active", func(t *testing.T) {
		var rl rate.RateLimited
		ok, _ := rl.GetOrReset()
		assert.False(t, ok)
	})
}
