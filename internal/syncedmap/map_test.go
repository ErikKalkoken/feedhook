package syncedmap_test

import (
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/syncedmap"
	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	t.Run("can store and load kv-pair", func(t *testing.T) {
		m := syncedmap.New[string, int]()
		m.Store("alpha", 1)
		v, ok := m.Load("alpha")
		assert.True(t, ok)
		assert.Equal(t, 1, v)
	})
	t.Run("should return false when key does not exist", func(t *testing.T) {
		m := syncedmap.New[string, int]()
		m.Store("alpha", 1)
		_, ok := m.Load("bravo")
		assert.False(t, ok)
	})
	t.Run("should work concurrently", func(t *testing.T) {
		values := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"}
		m := syncedmap.New[string, int]()
		var wg sync.WaitGroup
		for i, k := range values {
			wg.Add(1)
			go func(k string, v int) {
				defer wg.Done()
				time.Sleep(time.Duration(rand.IntN(250)) * time.Millisecond)
				m.Store(k, v)
			}(k, i)
		}
		wg.Wait()
		for i, k := range values {
			v, ok := m.Load(k)
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}
	})

}
