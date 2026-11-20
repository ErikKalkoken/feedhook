package storage

import (
	"bytes"
	"encoding/gob"

	"github.com/ErikKalkoken/feedhook/internal/app"
	bolt "go.etcd.io/bbolt"
)

// UpdateOrCreateWebhookStats executes a function that updates or creates the stats for webhook by name.
func (st *Storage) UpdateOrCreateWebhookStats(name string, f func(ws *app.WebhookStats) error) error {
	if name == "" {
		return ErrInvalid
	}
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketWebhooks))
		var fs *app.WebhookStats
		v := b.Get([]byte(name))
		var err error
		if v != nil {
			fs, err = webhookStatsFromDB(v)
			if err != nil {
				return err
			}
		} else {
			fs = &app.WebhookStats{Name: name}
		}
		if err := f(fs); err != nil {
			return err
		}
		v, err = webhookStatsToDB(fs)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), v)
	})
	return err
}

func (st *Storage) ClearWebhookStats() error {
	err := st.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketWebhooks))
		return b.ForEach(func(k, v []byte) error {
			if err := b.Delete(k); err != nil {
				return err
			}
			return nil
		})
	})
	return err
}

// GetWebhookStats returns the stats for webhook by name.
// It returns [ErrNotFound] when the webhook does not exist.
func (st *Storage) GetWebhookStats(name string) (*app.WebhookStats, error) {
	if name == "" {
		return nil, ErrInvalid
	}
	fs := &app.WebhookStats{Name: name}
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketWebhooks))
		v := b.Get([]byte(name))
		if v == nil {
			return ErrNotFound
		}
		var err error
		fs, err = webhookStatsFromDB(v)
		if err != nil {
			return err
		}
		return nil
	})
	return fs, err
}

func webhookStatsFromDB(v []byte) (*app.WebhookStats, error) {
	buf := bytes.NewBuffer(v)
	dec := gob.NewDecoder(buf)
	var o app.WebhookStats
	if err := dec.Decode(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

func webhookStatsToDB(fs *app.WebhookStats) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(*fs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
