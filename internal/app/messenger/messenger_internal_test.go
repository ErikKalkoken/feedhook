package messenger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSleepContext(t *testing.T) {
	t.Run("should report true when full duration elapsed", func(t *testing.T) {
		ok := sleepContext(context.Background(), 10*time.Millisecond)
		assert.True(t, ok)
	})
	t.Run("should return false promptly when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		ok := sleepContext(ctx, time.Hour)
		assert.False(t, ok)
		assert.Less(t, time.Since(start), 100*time.Millisecond)
	})
}
