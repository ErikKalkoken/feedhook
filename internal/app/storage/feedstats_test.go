package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

func TestStorage_GetFeedStats(t *testing.T) {
	t.Run("can update and read feed stats", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		err := st.UpdateOrCreateFeedStats(feedName, func(fs *app.FeedStats) error {
			fs.ReceivedCount++
			return nil
		})
		require.NoError(t, err)
		// when
		got, err := st.GetFeedStats(feedName)
		// then
		require.NoError(t, err)
		assert.Equal(t, feedName, got.Name)
		assert.Equal(t, 1, got.ReceivedCount)
	})

	t.Run("reports not found when object does not exist", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		// when
		_, err := st.GetFeedStats(feedName)
		// then
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestStorage_GetWebhookStats(t *testing.T) {
	t.Run("can update and read webhook stats", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		err := st.UpdateOrCreateWebhookStats(hookName, func(fs *app.WebhookStats) error {
			fs.SentCount++
			return nil
		})
		require.NoError(t, err)
		// when
		got, err := st.GetWebhookStats(hookName)
		// then
		require.NoError(t, err)
		assert.Equal(t, hookName, got.Name)
		assert.Equal(t, 1, got.SentCount)
	})

	t.Run("reports not found when object does not exist", func(t *testing.T) {
		// given
		st := newTestStorage(t)
		// when
		_, err := st.GetWebhookStats(hookName)
		// then
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

}
