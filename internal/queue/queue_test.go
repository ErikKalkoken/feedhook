package queue_test

import (
	"path/filepath"
	"testing"

	"github.com/ErikKalkoken/feedforward/internal/queue"
	"github.com/stretchr/testify/assert"

	bolt "go.etcd.io/bbolt"
)

func TestQueue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	q, err := queue.New(db, "")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	t.Run("can put and get with one item", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		err := q.Put([]byte("alpha"))
		if assert.NoError(t, err) {
			v, err := q.Get()
			if assert.NoError(t, err) {
				assert.Equal(t, []byte("alpha"), v)
			}
		}
	})
	t.Run("should return first added item", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		err := q.Put([]byte("alpha"))
		if assert.NoError(t, err) {
			err := q.Put([]byte("bravo"))
			if assert.NoError(t, err) {
				v, err := q.Get()
				if assert.NoError(t, err) {
					assert.Equal(t, []byte("alpha"), v)
					v, err := q.Get()
					if assert.NoError(t, err) {
						assert.Equal(t, []byte("bravo"), v)
					}
				}
			}
		}
	})
	t.Run("should report queue size", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		err := q.Put([]byte("alpha"))
		if assert.NoError(t, err) {
			assert.Equal(t, 1, q.Size())
		}
	})
	t.Run("should return empty queue error", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		_, err := q.Get()
		assert.ErrorIs(t, err, queue.ErrEmpty)
	})
}
