package storage

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/ErikKalkoken/feedforward/internal/app"
	bolt "go.etcd.io/bbolt"
)

func (st *Storage) UpdateWebhookStats(name string) error {
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
		fs.SentCount++
		fs.SentLast = time.Now().UTC()
		v, err = dbFromWebhookStats(fs)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), v)
	})
	return err
}

func (st *Storage) GetWebhookStats(name string) (*app.WebhookStats, error) {
	fs := &app.WebhookStats{Name: name}
	err := st.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketStats))
		b := root.Bucket([]byte(bucketWebhooks))
		v := b.Get([]byte(name))
		if v == nil {
			return nil
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

func dbFromWebhookStats(fs *app.WebhookStats) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(*fs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
