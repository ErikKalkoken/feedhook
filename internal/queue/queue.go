package queue

import (
	"encoding/binary"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var ErrEmpty = errors.New("empty queue")

type Queue struct {
	db         *bolt.DB
	bucketName string
}

func New(db *bolt.DB, name string) (*Queue, error) {
	bn := fmt.Sprintf("queue-%s", name)
	q := &Queue{db: db, bucketName: bn}
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bn))
		return err
	})
	if err != nil {
		return nil, err
	}
	return q, nil
}

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

func (q *Queue) Empty() bool {
	return q.Size() == 0
}
func (q *Queue) Get() ([]byte, error) {
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

func (q *Queue) Put(v []byte) error {
	err := q.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(q.bucketName))
		id, err := b.NextSequence()
		if err != nil {
			return err
		}
		return b.Put(itob(id), v)
	})
	return err
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
