package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestStorage_ListFeeds(t *testing.T) {
	t.Run("should return all configured feeds", func(t *testing.T) {
		// given
		st := newTestStorageWithFeeds(t, "alpha", "bravo")
		// when
		got, err := st.ListFeeds()
		// then
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"alpha", "bravo"}, got)
	})

	t.Run("should return empty list when no feeds are configured", func(t *testing.T) {
		// given
		st := newTestStorageWithFeeds(t)
		// when
		got, err := st.ListFeeds()
		// then
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("should ignore keys which are not feed buckets", func(t *testing.T) {
		// given
		st := newTestStorageWithFeeds(t, "alpha")
		err := st.DB().Update(func(tx *bolt.Tx) error {
			return tx.Bucket([]byte("feeds")).Put([]byte("bravo"), []byte("value"))
		})
		require.NoError(t, err)
		// when
		got, err := st.ListFeeds()
		// then
		require.NoError(t, err)
		assert.Equal(t, []string{"alpha"}, got)
	})
}
