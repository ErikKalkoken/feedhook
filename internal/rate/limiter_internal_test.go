package rate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRoundUpDuration(t *testing.T) {
	t.Run("should round up small fraction", func(t *testing.T) {
		x := roundUpDuration(1*time.Second+100*time.Millisecond, time.Second)
		assert.Equal(t, 2*time.Second, x)
	})
	t.Run("should round up large fraction", func(t *testing.T) {
		x := roundUpDuration(1*time.Second+900*time.Millisecond, time.Second)
		assert.Equal(t, 2*time.Second, x)
	})
	t.Run("should not round when no fraction", func(t *testing.T) {
		x := roundUpDuration(1*time.Second, time.Second)
		assert.Equal(t, 1*time.Second, x)
	})
}
