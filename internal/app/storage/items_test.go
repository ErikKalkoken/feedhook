package storage_test

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

func TestStorage_GetItemState(t *testing.T) {
	st := newTestStorage(t)

	t.Run("should report not found for missing item without GUID", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		i := &gofeed.Item{Title: "title", Description: "description"}
		// when
		_, err := st.GetItemState(feedName, i)
		// then
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("should report not found for missing item with GUID", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		i := &gofeed.Item{GUID: "abc1"}
		// when
		_, err := st.GetItemState(feedName, i)
		// then
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("should report known item with GUID and same publish date as processed", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		t1 := time.Now()
		i1 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t1}
		err := st.UpdateOrCreateItem(feedName, i1)
		require.NoError(t, err)
		i2 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t1}
		// when
		s, err := st.GetItemState(feedName, i2)
		// then
		require.NoError(t, err)
		assert.Equal(t, app.StateProcessed, s)
	})

	t.Run("should report known item with GUID and different publish date as updated", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		t1 := time.Now().Add(-5 * time.Second)
		i1 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t1}
		err := st.UpdateOrCreateItem(feedName, i1)
		require.NoError(t, err)
		t2 := time.Now()
		i2 := &gofeed.Item{GUID: "abc2", PublishedParsed: &t2}
		// when
		s, err := st.GetItemState(feedName, i2)
		// then
		require.NoError(t, err)
		assert.Equal(t, app.StateUpdated, s)
	})

	t.Run("should report known item without GUID as processed", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		i := &gofeed.Item{Title: "title", Description: "description"}
		err := st.UpdateOrCreateItem(feedName, i)
		require.NoError(t, err)
		// when
		s, err := st.GetItemState(feedName, i)
		// then
		require.NoError(t, err)
		assert.Equal(t, app.StateProcessed, s)
	})

	t.Run("should report not found for invalid item record", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		putInvalidItem(t, st, "abc1")
		t1 := time.Now()
		i := &gofeed.Item{GUID: "abc1", PublishedParsed: &t1}
		// when
		_, err := st.GetItemState(feedName, i)
		// then
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("should return error when feed name is empty", func(t *testing.T) {
		_, err := st.GetItemState("", &gofeed.Item{GUID: "abc1"})
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when item is nil", func(t *testing.T) {
		_, err := st.GetItemState(feedName, nil)
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when feed is unknown", func(t *testing.T) {
		_, err := st.GetItemState("unknown", &gofeed.Item{GUID: "abc1"})
		require.ErrorIs(t, err, storage.ErrInvalid)
	})
}

func TestStorage_UpdateOrCreateItem(t *testing.T) {
	st := newTestStorage(t)

	t.Run("should return error when feed name is empty", func(t *testing.T) {
		err := st.UpdateOrCreateItem("", &gofeed.Item{GUID: "abc1"})
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when item is nil", func(t *testing.T) {
		err := st.UpdateOrCreateItem(feedName, nil)
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when feed is unknown", func(t *testing.T) {
		err := st.UpdateOrCreateItem("unknown", &gofeed.Item{GUID: "abc1"})
		require.ErrorIs(t, err, storage.ErrInvalid)
	})
}

func TestStorage_CountItems(t *testing.T) {
	st := newTestStorage(t)

	t.Run("should return count of items", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		err := st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "1"})
		require.NoError(t, err)
		err = st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "2"})
		require.NoError(t, err)
		// when
		c, err := st.CountItems(feedName)
		// then
		require.NoError(t, err)
		assert.Equal(t, 2, c)
	})

	t.Run("should return zero for empty feed", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		// when
		c, err := st.CountItems(feedName)
		// then
		require.NoError(t, err)
		assert.Equal(t, 0, c)
	})

	t.Run("should return error when feed name is empty", func(t *testing.T) {
		_, err := st.CountItems("")
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when feed is unknown", func(t *testing.T) {
		_, err := st.CountItems("unknown")
		require.ErrorIs(t, err, storage.ErrInvalid)
	})
}

func TestStorage_ListItems(t *testing.T) {
	st := newTestStorage(t)

	t.Run("should skip invalid item records", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		err := st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "1"})
		require.NoError(t, err)
		putInvalidItem(t, st, "2")
		// when
		ii, err := st.ListItems(feedName)
		// then
		require.NoError(t, err)
		require.Len(t, ii, 1)
		assert.Equal(t, "1", ii[0].ID)
	})

	t.Run("should return error when feed name is empty", func(t *testing.T) {
		_, err := st.ListItems("")
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when feed is unknown", func(t *testing.T) {
		_, err := st.ListItems("unknown")
		require.ErrorIs(t, err, storage.ErrInvalid)
	})
}

func TestStorage_PruneOldestItems(t *testing.T) {
	st := newTestStorage(t)

	t.Run("should delete oldest items", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		now := time.Now().Add(-10 * time.Hour)
		t1 := now.Add(5 * time.Hour)
		err := st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "1", PublishedParsed: &t1})
		require.NoError(t, err)
		t2 := now.Add(1 * time.Hour)
		err = st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "2", PublishedParsed: &t2})
		require.NoError(t, err)
		t3 := now.Add(4 * time.Hour)
		err = st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "3", PublishedParsed: &t3})
		require.NoError(t, err)
		// when
		err = st.PruneOldestItems(feedName, 2)
		// then
		require.NoError(t, err)
		c, err := st.CountItems(feedName)
		require.NoError(t, err)
		assert.Equal(t, 2, c)
		ii, err := st.ListItems(feedName)
		require.NoError(t, err)
		var ids []string
		for _, i := range ii {
			ids = append(ids, string(i.ID))
		}
		assert.Contains(t, ids, "1")
		assert.Contains(t, ids, "3")
	})

	t.Run("should delete invalid items and prune valid ones", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		now := time.Now().Add(-10 * time.Hour)
		t1 := now.Add(5 * time.Hour)
		err := st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "1", PublishedParsed: &t1})
		require.NoError(t, err)
		t2 := now.Add(1 * time.Hour)
		err = st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "2", PublishedParsed: &t2})
		require.NoError(t, err)
		t3 := now.Add(4 * time.Hour)
		err = st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "3", PublishedParsed: &t3})
		require.NoError(t, err)
		putInvalidItem(t, st, "4")
		// when
		err = st.PruneOldestItems(feedName, 2)
		// then
		require.NoError(t, err)
		c, err := st.CountItems(feedName)
		require.NoError(t, err)
		assert.Equal(t, 2, c)
		ii, err := st.ListItems(feedName)
		require.NoError(t, err)
		var ids []string
		for _, i := range ii {
			ids = append(ids, i.ID)
		}
		assert.ElementsMatch(t, []string{"1", "3"}, ids)
	})

	t.Run("should delete invalid items when below limit", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		err := st.UpdateOrCreateItem(feedName, &gofeed.Item{GUID: "1"})
		require.NoError(t, err)
		putInvalidItem(t, st, "2")
		// when
		err = st.PruneOldestItems(feedName, 10)
		// then
		require.NoError(t, err)
		c, err := st.CountItems(feedName)
		require.NoError(t, err)
		assert.Equal(t, 1, c)
	})

	t.Run("should return error when feed name is empty", func(t *testing.T) {
		err := st.PruneOldestItems("", 2)
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when limit is zero", func(t *testing.T) {
		err := st.PruneOldestItems(feedName, 0)
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when limit is negative", func(t *testing.T) {
		err := st.PruneOldestItems(feedName, -1)
		require.ErrorIs(t, err, storage.ErrInvalid)
	})

	t.Run("should return error when feed is unknown", func(t *testing.T) {
		err := st.PruneOldestItems("unknown", 2)
		require.ErrorIs(t, err, storage.ErrInvalid)
	})
}

// putInvalidItem stores a record that can not be decoded for an item.
func putInvalidItem(t *testing.T, st *storage.Storage, key string) {
	t.Helper()
	err := st.DB().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("feeds")).Bucket([]byte(feedName))
		return b.Put([]byte(key), []byte("invalid"))
	})
	require.NoError(t, err)
}
