package app

import (
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"
)

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
	return rt.now
}

func TestIsItemNew(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cf := ConfigFeed{Name: "feed1", URL: "https://www.example.com/feed", Webhook: "hook1"}
	cfg := MyConfig{
		App:   ConfigApp{Oldest: 3600 * 24, Ticker: 1},
		Feeds: []ConfigFeed{cf},
	}
	a := New(db, cfg, faketime{})
	if err := a.Init(); err != nil {
		log.Fatalf("Failed to init: %s", err)
	}
	t.Run("should report true when item has GUI and does not exit", func(t *testing.T) {
		if err := clearFeeds(db); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{GUID: "abc1"}
		assert.True(t, a.isItemNew(cf, i))
	})
	t.Run("should report false when item has GUI and exists", func(t *testing.T) {
		if err := clearFeeds(db); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{GUID: "abc2"}
		if err := a.recordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		assert.False(t, a.isItemNew(cf, i))
	})
	t.Run("should report true when item has no GUI and does not exit", func(t *testing.T) {
		if err := clearFeeds(db); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{Title: "title", Description: "description"}
		assert.True(t, a.isItemNew(cf, i))
	})
	t.Run("should report false when item has no GUI and exists", func(t *testing.T) {
		if err := clearFeeds(db); err != nil {
			t.Fatal(err)
		}
		i := &gofeed.Item{Title: "title", Description: "description"}
		if err := a.recordItem(cf, i); err != nil {
			t.Fatal(err)
		}
		assert.False(t, a.isItemNew(cf, i))
	})
	t.Run("should delete oldest items", func(t *testing.T) {
		if err := clearFeeds(db); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Add(-10 * time.Hour)
		t1 := now.Add(5 * time.Hour)
		if err := a.recordItem(cf, &gofeed.Item{GUID: "1", PublishedParsed: &t1}); err != nil {
			t.Fatal(err)
		}
		t2 := now.Add(1 * time.Hour)
		if err := a.recordItem(cf, &gofeed.Item{GUID: "2", PublishedParsed: &t2}); err != nil {
			t.Fatal(err)
		}
		t3 := now.Add(4 * time.Hour)
		if err := a.recordItem(cf, &gofeed.Item{GUID: "3", PublishedParsed: &t3}); err != nil {
			t.Fatal(err)
		}
		err := a.cullFeed(cf, 2)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, feedSize(db, cf))
			ii, err := items(db, cf)
			if assert.NoError(t, err) {
				var ids []string
				for _, i := range ii {
					ids = append(ids, string(i.key))
				}
				assert.Contains(t, ids, "1")
				assert.Contains(t, ids, "3")
			}
		}
	})
}

func clearFeeds(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		root.ForEachBucket(func(k []byte) error {
			b := root.Bucket(k)
			b.ForEach(func(k, v []byte) error {
				if err := b.Delete(k); err != nil {
					return err
				}
				return nil
			})
			return nil
		})
		return nil
	})
	return err
}

func feedSize(db *bolt.DB, cf ConfigFeed) int {
	var c int
	db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		b.ForEach(func(k, v []byte) error {
			c++
			return nil
		})
		return nil
	})
	return c
}

func items(db *bolt.DB, cf ConfigFeed) ([]item, error) {
	var ii []item
	err := db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		b.ForEach(func(k, v []byte) error {
			x, err := time.Parse(time.RFC3339, string(v))
			if err != nil {
				return err
			}
			ii = append(ii, item{key: k, time: x})
			return nil
		})
		return nil
	})
	return ii, err
}
