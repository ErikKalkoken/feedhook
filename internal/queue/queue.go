// Package queue contains a persistent queue.
package queue

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	bolt "go.etcd.io/bbolt"
)

var ErrEmpty = errors.New("empty queue")

// Queue represents a persistent FIFO queue and support multiple concurrent consumers and produces.
// It uses Bolt as database.
type Queue struct {
	db         *bolt.DB
	bucketName string

	mu   sync.Mutex
	cond *sync.Cond
}

// New returns a new Queue object with a given name.
// When a queue with that name already exists in the DB, it will be re-used.
func New(db *bolt.DB, name string) (*Queue, error) {
	bn := fmt.Sprintf("queue-%s", name)
	q := &Queue{
		db:         db,
		bucketName: bn,
	}
	q.cond = sync.NewCond(&q.mu)
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bn))
		return err
	})
	if err != nil {
		return nil, err
	}
	return q, nil
}

// Clear deletes all items from the queue.
func (q *Queue) Clear() error {
	err := q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.bucketName))
		b.ForEach(func(k, v []byte) error {
			return b.Delete(k)
		})
		return nil
	})
	return err
}

// IsEmpty reports wether the queue is empty.
func (q *Queue) IsEmpty() bool {
	return q.Size() == 0
}

// GetNoWait return an item from the queue.
// When the queue is empty it returns the ErrEmpty error.
func (q *Queue) GetNoWait() ([]byte, error) {
	var v2 []byte
	err := q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.bucketName))
		c := b.Cursor()
		k, v := c.First()
		if k == nil {
			return ErrEmpty
		}
		if err := b.Delete(k); err != nil {
			return err
		}
		v2 = v
		return nil
	})
	return v2, err
}

// Get returns an item from the queue. If the queue is empty it will block until there is a new item.
func (q *Queue) Get() ([]byte, error) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for {
		v, err := q.GetNoWait()
		if err == nil {
			return v, nil
		} else if err != ErrEmpty {
			return nil, err
		}
		q.cond.Wait()
	}
}

// Puts adds an item to the queue.
func (q *Queue) Put(v []byte) error {
	err := q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.bucketName))
		id, err := b.NextSequence()
		if err != nil {
			return err
		}
		return b.Put(itob(id), v)
	})
	if err != nil {
		return err
	}
	q.cond.Signal()
	return nil
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func (q *Queue) Size() int {
	var c int
	q.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.bucketName))
		b.ForEach(func(k, v []byte) error {
			c++
			return nil
		})
		return nil
	})
	return c
}
