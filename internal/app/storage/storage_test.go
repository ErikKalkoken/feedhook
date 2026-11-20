package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

const (
	feedName = "feed1"
	hookName = "hook1"
)

// newTestStorage returns an initialized storage with the default test feed.
func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	return newTestStorageWithFeeds(t, feedName)
}

// newTestStorageWithFeeds returns an initialized storage with the given feeds.
func newTestStorageWithFeeds(t *testing.T, names ...string) *storage.Storage {
	t.Helper()
	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0600, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	st := storage.New(db, newConfigWithFeeds(names...))
	err = st.Init()
	require.NoError(t, err)
	return st
}

// newConfigWithFeeds returns a config with the given feeds.
func newConfigWithFeeds(names ...string) config.Config {
	var feeds []config.ConfigFeed
	for _, n := range names {
		feeds = append(feeds, config.ConfigFeed{
			Name:    n,
			URL:     "https://www.example.com/" + n,
			Webhook: hookName,
		})
	}
	return config.Config{
		Feeds: feeds,
	}
}

func TestStorage_Init(t *testing.T) {
	t.Run("should create buckets for feeds and stats", func(t *testing.T) {
		// when
		st := newTestStorageWithFeeds(t, "alpha", "bravo")
		// then
		err := st.DB().View(func(tx *bolt.Tx) error {
			bf := tx.Bucket([]byte("feeds"))
			require.NotNil(t, bf)
			assert.NotNil(t, bf.Bucket([]byte("alpha")))
			assert.NotNil(t, bf.Bucket([]byte("bravo")))
			bs := tx.Bucket([]byte("stats"))
			require.NotNil(t, bs)
			assert.NotNil(t, bs.Bucket([]byte("feeds")))
			assert.NotNil(t, bs.Bucket([]byte("webhooks")))
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("should keep existing data when run again", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		err := st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "abc1"})
		require.NoError(t, err)
		err = st.UpdateOrCreateFeedStats(feedName, func(fs *app.FeedStats) error {
			fs.ReceivedCount = 3
			return nil
		})
		require.NoError(t, err)
		// when
		err = st.Init()
		// then
		require.NoError(t, err)
		n, err := st.CountItems(feedName)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		fs, err := st.GetFeedStats(feedName)
		require.NoError(t, err)
		assert.Equal(t, 3, fs.ReceivedCount)
	})

	t.Run("should add buckets for new feeds", func(t *testing.T) {
		// given
		st := newTestStorageWithFeeds(t, "alpha")
		st2 := storage.New(st.DB(), newConfigWithFeeds("alpha", "bravo"))
		// when
		err := st2.Init()
		// then
		require.NoError(t, err)
		got, err := st2.ListFeeds()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"alpha", "bravo"}, got)
	})

	t.Run("should delete buckets of obsolete feeds", func(t *testing.T) {
		// given
		names := []string{"alpha", "bravo", "charlie", "delta", "echo"}
		st := newTestStorageWithFeeds(t, names...)
		for _, n := range names {
			err := st.UpdateOrCreateItem(n, &gofeed.Item{GUID: "abc1"})
			require.NoError(t, err)
		}
		st2 := storage.New(st.DB(), newConfigWithFeeds("bravo", "delta"))
		// when
		err := st2.Init()
		// then
		require.NoError(t, err)
		got, err := st2.ListFeeds()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"bravo", "delta"}, got)
		for _, n := range []string{"bravo", "delta"} {
			c, err := st2.CountItems(n)
			require.NoError(t, err)
			assert.Equal(t, 1, c, "items of feed %s", n)
		}
		_, err = st2.CountItems("alpha")
		assert.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should delete all feed buckets when no feeds are configured", func(t *testing.T) {
		// given
		st := newTestStorageWithFeeds(t, "alpha", "bravo")
		st2 := storage.New(st.DB(), config.Config{})
		// when
		err := st2.Init()
		// then
		require.NoError(t, err)
		got, err := st2.ListFeeds()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestStorage_DB(t *testing.T) {
	t.Run("should return database passed to New", func(t *testing.T) {
		// given
		db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0600, nil)
		require.NoError(t, err)
		t.Cleanup(func() {
			db.Close()
		})
		st := storage.New(db, config.Config{})
		// when
		got := st.DB()
		// then
		assert.Same(t, db, got)
	})
}
