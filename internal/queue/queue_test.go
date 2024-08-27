package queue_test

import (
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/ErikKalkoken/feedforward/internal/queue"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	bolt "go.etcd.io/bbolt"
)

func TestQueue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	q, err := queue.New(db, "test")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	t.Run("can put and get with one item", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		err := q.Put([]byte("alpha"))
		if assert.NoError(t, err) {
			v, err := q.GetNoWait()
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
				v, err := q.GetNoWait()
				if assert.NoError(t, err) {
					assert.Equal(t, []byte("alpha"), v)
					v, err := q.GetNoWait()
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
		_, err := q.GetNoWait()
		assert.ErrorIs(t, err, queue.ErrEmpty)
	})
	t.Run("should wait until there is an item in the queue", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		g := new(errgroup.Group)
		g.Go(func() error {
			v, err := q.Get()
			if err != nil {
				return err
			}
			assert.Equal(t, []byte("alpha"), v)
			return nil
		})
		time.Sleep(250 * time.Millisecond)
		if err := q.Put([]byte("alpha")); err != nil {
			t.Fatal(err)
		}
		if err := g.Wait(); err != nil {
			t.Fatal(err)
		}
		assert.True(t, q.IsEmpty())
	})
	t.Run("should allow multiple consumers and producers", func(t *testing.T) {
		if err := q.Clear(); err != nil {
			t.Fatal(err)
		}
		results := make([]string, 0)
		g := new(errgroup.Group)
		g.Go(func() error {
			for range 3 {
				v, err := q.Get()
				if err != nil {
					return err
				}
				results = append(results, string(v))
			}
			return nil
		})
		g.Go(func() error {
			for range 3 {
				v, err := q.Get()
				if err != nil {
					return err
				}
				results = append(results, string(v))
			}
			return nil
		})
		time.Sleep(250 * time.Millisecond)
		g.Go(func() error {
			for _, x := range []string{"alpha", "bravo", "charlie"} {
				if err := q.Put([]byte(x)); err != nil {
					return err
				}
				time.Sleep(time.Duration(rand.IntN(250)) * time.Millisecond)
			}
			return nil
		})
		g.Go(func() error {
			for _, x := range []string{"delta", "echo", "foxtrot"} {
				if err := q.Put([]byte(x)); err != nil {
					return err
				}
				time.Sleep(time.Duration(rand.IntN(250)) * time.Millisecond)
			}
			return nil
		})
		if err := g.Wait(); err != nil {
			t.Fatal(err)
		}
		assert.True(t, q.IsEmpty())
		assert.ElementsMatch(t, results, []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"})
	})
}

func TestResurrectQueue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	q, err := queue.New(db, "johnny")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	if err := q.Put([]byte("alpha")); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	q, err = queue.New(db, "johnny")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	v, err := q.Get()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []byte("alpha"), v)
	db.Close()
}
