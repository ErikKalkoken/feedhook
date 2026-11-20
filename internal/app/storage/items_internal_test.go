package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ErikKalkoken/feedhook/internal/app"
)

func TestProcessedItemSerialization(t *testing.T) {
	t.Run("should restore item from its bytes", func(t *testing.T) {
		// given
		pi := &app.ProcessedItem{
			ID:        "abc1",
			Published: time.Date(2026, 11, 20, 12, 30, 0, 0, time.UTC),
		}
		b, err := processedItemToDB(pi)
		require.NoError(t, err)
		// when
		got, err := processedItemFromDB(b)
		// then
		require.NoError(t, err)
		assert.Equal(t, pi.ID, got.ID)
		assert.True(t, pi.Published.Equal(got.Published))
	})

	t.Run("should restore item with zero values", func(t *testing.T) {
		// given
		pi := &app.ProcessedItem{}
		b, err := processedItemToDB(pi)
		require.NoError(t, err)
		// when
		got, err := processedItemFromDB(b)
		// then
		require.NoError(t, err)
		assert.Equal(t, "", got.ID)
		assert.True(t, got.Published.IsZero())
	})

	t.Run("should return error when bytes are invalid", func(t *testing.T) {
		_, err := processedItemFromDB([]byte("invalid"))
		assert.Error(t, err)
	})

	t.Run("should return error when bytes are empty", func(t *testing.T) {
		_, err := processedItemFromDB([]byte{})
		assert.Error(t, err)
	})
}
