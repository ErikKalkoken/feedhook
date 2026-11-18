// Package pqueue implements persistent queues.
package pqueue

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"sync"

	bolt "go.etcd.io/bbolt"
)

var errEmpty = errors.New("empty queue")

// PQueue represents a persistent FIFO queue.
type PQueue struct {
	db   *bolt.DB
	name string
	proc chan struct{} // allows one Process call at a time

	mu   sync.Mutex
	cond *sync.Cond
}

// New returns a new PQueue instance with the given name.
// When a queue with that name already exists in the DB, it will be re-used.
func New(db *bolt.DB, name string) (*PQueue, error) {
	q := &PQueue{
		db:   db,
		name: name,
		proc: make(chan struct{}, 1),
	}
	q.cond = sync.NewCond(&q.mu)
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		return err
	})
	if err != nil {
		return nil, err
	}
	return q, nil
}

// Clear deletes all items from the queue.
func (q *PQueue) Clear() error {
	err := q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.name))
		b.ForEach(func(k, v []byte) error {
			return b.Delete(k)
		})
		return nil
	})
	return err
}

// IsEmpty reports wether the queue is empty.
func (q *PQueue) IsEmpty() bool {
	return q.Size() == 0
}

// Process passes the next item to fn, blocking until one is available or ctx is done.
// The item is removed only when fn returns nil. Concurrent calls are serialized.
func (q *PQueue) Process(ctx context.Context, fn func(v []byte) error) error {
	select {
	case q.proc <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-q.proc }()
	k, v, err := q.peekWithContext(ctx)
	if err != nil {
		return err
	}
	if err := fn(v); err != nil {
		return err
	}
	return q.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(q.name)).Delete(k)
	})
}

// Get removes and returns the next item, blocking until one is available or ctx is done.
func (q *PQueue) Get(ctx context.Context) ([]byte, error) {
	var v []byte
	err := q.Process(ctx, func(b []byte) error {
		v = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

// peekWithContext returns the first item without removing it,
// blocking until one is available or ctx is done.
func (q *PQueue) peekWithContext(ctx context.Context) ([]byte, []byte, error) {
	stopf := context.AfterFunc(ctx, func() {
		q.cond.L.Lock()
		defer q.cond.L.Unlock()
		q.cond.Broadcast()
	})
	defer stopf()

	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for {
		k, v, err := q.peekNoWait()
		if err == nil {
			return k, v, nil
		}
		if err != errEmpty {
			return nil, nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		q.cond.Wait()
	}
}

// peekNoWait returns the key and value of the first item without removing it.
// When the queue is empty it returns errEmpty.
func (q *PQueue) peekNoWait() ([]byte, []byte, error) {
	var k, v []byte
	err := q.db.View(func(tx *bolt.Tx) error {
		k2, v2 := tx.Bucket([]byte(q.name)).Cursor().First()
		if k2 == nil {
			return errEmpty
		}
		k, v = bytes.Clone(k2), bytes.Clone(v2) // only valid for the life of the transaction
		return nil
	})
	return k, v, err
}

// Put adds an item to the queue.
func (q *PQueue) Put(v []byte) error {
	err := q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.name))
		id, err := b.NextSequence()
		if err != nil {
			return err
		}
		return b.Put(itob(id), v)
	})
	if err != nil {
		return err
	}
	// lock so the signal can't fall between a reader's empty check and its Wait
	q.cond.L.Lock()
	q.cond.Signal()
	q.cond.L.Unlock()
	return nil
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func (q *PQueue) Size() int {
	var c int
	q.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.name))
		c = b.Stats().KeyN
		return nil
	})
	return c
}
