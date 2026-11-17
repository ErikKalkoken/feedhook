package pqueue_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	bolt "go.etcd.io/bbolt"
)

func createDB(t *testing.T) *bolt.DB {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	return db
}

func createQueue(t *testing.T, db *bolt.DB) *pqueue.PQueue {
	t.Helper()
	q, err := pqueue.New(db, "test")
	require.NoError(t, err)
	require.NoError(t, q.Clear())
	return q
}

func TestNew(t *testing.T) {
	db := createDB(t)
	t.Run("should create empty queue", func(t *testing.T) {
		q, err := pqueue.New(db, "alpha")
		require.NoError(t, err)
		assert.True(t, q.IsEmpty())
	})
	t.Run("should return error for invalid name", func(t *testing.T) {
		_, err := pqueue.New(db, "")
		assert.Error(t, err)
	})
	t.Run("should keep queues with different names separate", func(t *testing.T) {
		// given
		q1, err := pqueue.New(db, "q1")
		require.NoError(t, err)
		q2, err := pqueue.New(db, "q2")
		require.NoError(t, err)
		require.NoError(t, q1.Put([]byte("alpha")))
		require.NoError(t, q2.Put([]byte("bravo")))
		require.NoError(t, q2.Put([]byte("charlie")))
		// when
		v, err := q1.Get(t.Context())
		require.NoError(t, err)
		require.NoError(t, q2.Clear())
		require.NoError(t, q1.Put([]byte("delta")))
		// then
		assert.Equal(t, []byte("alpha"), v)
		assert.Equal(t, 1, q1.Size())
		assert.True(t, q2.IsEmpty())
	})
}

func TestClear(t *testing.T) {
	db := createDB(t)
	t.Run("should remove all items", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		for _, x := range []string{"alpha", "bravo"} {
			require.NoError(t, q.Put([]byte(x)))
		}
		// when
		err := q.Clear()
		// then
		require.NoError(t, err)
		assert.True(t, q.IsEmpty())
	})
}

func TestIsEmpty(t *testing.T) {
	db := createDB(t)
	t.Run("should report true for empty queue", func(t *testing.T) {
		q := createQueue(t, db)
		assert.True(t, q.IsEmpty())
	})
	t.Run("should report false for queue with items", func(t *testing.T) {
		q := createQueue(t, db)
		require.NoError(t, q.Put([]byte("alpha")))
		assert.False(t, q.IsEmpty())
	})
}

func TestSize(t *testing.T) {
	db := createDB(t)
	t.Run("should report size of queue with items", func(t *testing.T) {
		q := createQueue(t, db)
		require.NoError(t, q.Put([]byte("alpha")))
		assert.Equal(t, 1, q.Size())
	})
	t.Run("should report zero for empty queue", func(t *testing.T) {
		q := createQueue(t, db)
		assert.Equal(t, 0, q.Size())
	})
}

func TestPut(t *testing.T) {
	db := createDB(t)
	t.Run("can put and get with one item", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		// when
		err := q.Put([]byte("alpha"))
		// then
		require.NoError(t, err)
		v, err := q.Get(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []byte("alpha"), v)
	})
	t.Run("should return error when DB fails", func(t *testing.T) {
		db := createDB(t)
		q := createQueue(t, db)
		require.NoError(t, db.Close())
		err := q.Put([]byte("alpha"))
		assert.Error(t, err)
	})
}

func TestGet(t *testing.T) {
	db := createDB(t)
	t.Run("should return items in the order they were added", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		require.NoError(t, q.Put([]byte("alpha")))
		require.NoError(t, q.Put([]byte("bravo")))
		// when
		v1, err1 := q.Get(t.Context())
		v2, err2 := q.Get(t.Context())
		// then
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, []byte("alpha"), v1)
		assert.Equal(t, []byte("bravo"), v2)
	})
	t.Run("should keep order beyond 255 items", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		const n = 300 // keys need more than one byte
		for i := range n {
			require.NoError(t, q.Put([]byte(strconv.Itoa(i))))
		}
		// when
		got := make([]string, 0, n)
		for range n {
			v, err := q.Get(t.Context())
			require.NoError(t, err)
			got = append(got, string(v))
		}
		// then
		want := make([]string, 0, n)
		for i := range n {
			want = append(want, strconv.Itoa(i))
		}
		assert.Equal(t, want, got)
	})
	t.Run("should return error when DB fails", func(t *testing.T) {
		db := createDB(t)
		q := createQueue(t, db)
		require.NoError(t, db.Close())
		_, err := q.Get(t.Context())
		assert.Error(t, err)
	})
	t.Run("should time out on empty queue", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		// when
		_, err := q.Get(ctx)
		// then
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("should let get wait for an ongoing process", func(t *testing.T) {
		q := createQueue(t, db)
		for _, x := range []string{"alpha", "bravo"} {
			require.NoError(t, q.Put([]byte(x)))
		}
		started := make(chan struct{})
		release := make(chan struct{})
		g := new(errgroup.Group)
		g.Go(func() error {
			return q.Process(t.Context(), func(v []byte) error {
				close(started)
				<-release
				return nil
			})
		})
		<-started
		got := make(chan []byte, 1)
		g.Go(func() error {
			v, err := q.Get(t.Context())
			got <- v
			return err
		})
		select {
		case v := <-got:
			t.Fatalf("Get returned %q during an ongoing Process", v)
		case <-time.After(100 * time.Millisecond):
		}
		close(release)
		require.NoError(t, g.Wait())
		assert.Equal(t, []byte("bravo"), <-got)
	})
	t.Run("should wait until there is an item in the queue", func(t *testing.T) {
		q := createQueue(t, db)
		g := new(errgroup.Group)
		g.Go(func() error {
			v, err := q.Get(t.Context())
			if err != nil {
				return err
			}
			assert.Equal(t, []byte("alpha"), v)
			return nil
		})
		time.Sleep(250 * time.Millisecond)
		require.NoError(t, q.Put([]byte("alpha")))
		require.NoError(t, g.Wait())
		assert.True(t, q.IsEmpty())
	})
	t.Run("should abort wait when context is canceled", func(t *testing.T) {
		q := createQueue(t, db)
		g := new(errgroup.Group)
		ctx, cancel := context.WithCancel(t.Context())
		g.Go(func() error {
			_, err := q.Get(ctx)
			return err
		})
		time.Sleep(10 * time.Millisecond)
		cancel()
		err := g.Wait()
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestProcess(t *testing.T) {
	db := createDB(t)
	t.Run("should remove item when fn succeeds", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		require.NoError(t, q.Put([]byte("alpha")))
		// when
		var got []byte
		err := q.Process(t.Context(), func(v []byte) error {
			got = v
			return nil
		})
		// then
		require.NoError(t, err)
		assert.Equal(t, []byte("alpha"), got)
		assert.True(t, q.IsEmpty())
	})
	t.Run("should keep item when fn fails", func(t *testing.T) {
		q := createQueue(t, db)
		for _, x := range []string{"alpha", "bravo"} {
			require.NoError(t, q.Put([]byte(x)))
		}
		errFail := errors.New("fail")
		err := q.Process(t.Context(), func(v []byte) error {
			return errFail
		})
		assert.ErrorIs(t, err, errFail)
		assert.Equal(t, 2, q.Size())
		for _, want := range []string{"alpha", "bravo"} {
			v, err := q.Get(t.Context())
			require.NoError(t, err)
			assert.Equal(t, want, string(v))
		}
	})
	t.Run("should abort process when context is canceled", func(t *testing.T) {
		q := createQueue(t, db)
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(10*time.Millisecond, cancel)
		err := q.Process(ctx, func(v []byte) error {
			t.Error("fn called on empty queue")
			return nil
		})
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("should abort waiting for an ongoing process when context is canceled", func(t *testing.T) {
		// given
		q := createQueue(t, db)
		require.NoError(t, q.Put([]byte("alpha")))
		started := make(chan struct{})
		release := make(chan struct{})
		g := new(errgroup.Group)
		g.Go(func() error {
			return q.Process(t.Context(), func(v []byte) error {
				close(started)
				<-release
				return nil
			})
		})
		<-started
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(10*time.Millisecond, cancel)
		// when
		err := q.Process(ctx, func(v []byte) error {
			t.Error("fn called while another process is ongoing")
			return nil
		})
		// then
		assert.ErrorIs(t, err, context.Canceled)
		close(release)
		require.NoError(t, g.Wait())
		assert.True(t, q.IsEmpty())
	})
	t.Run("should serialize concurrent process calls", func(t *testing.T) {
		q := createQueue(t, db)
		const n = 50
		for i := range n {
			require.NoError(t, q.Put([]byte(strconv.Itoa(i))))
		}
		var active atomic.Int32
		var overlap atomic.Bool
		var mu sync.Mutex
		seen := make(map[string]int)
		g := new(errgroup.Group)
		for range 5 {
			g.Go(func() error {
				for range n / 5 {
					err := q.Process(t.Context(), func(v []byte) error {
						if active.Add(1) > 1 {
							overlap.Store(true)
						}
						defer active.Add(-1)
						time.Sleep(time.Millisecond)
						mu.Lock()
						seen[string(v)]++
						mu.Unlock()
						return nil
					})
					if err != nil {
						return err
					}
				}
				return nil
			})
		}
		require.NoError(t, g.Wait())
		assert.False(t, overlap.Load(), "fn ran concurrently")
		assert.Len(t, seen, n)
		for k, c := range seen {
			assert.Equal(t, 1, c, "item %s", k)
		}
		assert.True(t, q.IsEmpty())
	})
}

func TestConcurrent(t *testing.T) {
	db := createDB(t)
	t.Run("should allow multiple consumers and producers", func(t *testing.T) {
		q := createQueue(t, db)
		results := make([]string, 6)
		g := new(errgroup.Group)
		g.Go(func() error {
			for i := range 3 {
				v, err := q.Get(t.Context())
				if err != nil {
					return err
				}
				results[i] = string(v)
			}
			return nil
		})
		g.Go(func() error {
			for i := range 3 {
				v, err := q.Get(t.Context())
				if err != nil {
					return err
				}
				results[i+3] = string(v)
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
		require.NoError(t, g.Wait())
		assert.True(t, q.IsEmpty())
		assert.ElementsMatch(t, results, []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"})
	})
	t.Run("should process larger workload with parallel consumers without error", func(t *testing.T) {
		q := createQueue(t, db)
		items := make([]string, 1000)
		for i := range len(items) {
			items[i] = randSeq(rand.IntN(20) + 5)
		}
		for _, v := range items {
			require.NoError(t, q.Put([]byte(v)))
		}
		resultsC := make(chan string, len(items))
		g := new(errgroup.Group)
		for range 3 {
			g.Go(func() error {
				for {
					ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
					v, err := q.Get(ctx)
					cancel()
					if errors.Is(err, context.DeadlineExceeded) {
						break
					} else if err != nil {
						return err
					}
					resultsC <- string(v)
				}
				return nil
			})
		}
		require.NoError(t, g.Wait())
		results := make([]string, 0)
		for len(resultsC) > 0 {
			results = append(results, <-resultsC)
		}
		assert.ElementsMatch(t, results, items)
	})
}

func TestResurrectQueue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	require.NoError(t, err)
	q, err := pqueue.New(db, "johnny")
	require.NoError(t, err)
	require.NoError(t, q.Put([]byte("alpha")))
	db.Close()
	db, err = bolt.Open(p, 0600, nil)
	require.NoError(t, err)
	defer db.Close()
	q, err = pqueue.New(db, "johnny")
	require.NoError(t, err)
	v, err := q.Get(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []byte("alpha"), v)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}
