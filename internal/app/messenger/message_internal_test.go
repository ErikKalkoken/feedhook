package messenger

import (
	"reflect"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage(t *testing.T) {
	t.Run("can create new message from feed item", func(t *testing.T) {
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		x, err := newMessage("dummy", feed, item, false)
		require.NoError(t, err)
		assert.Equal(t, "content", x.Item.Description)
	})
	t.Run("can serialize and de-serialize a message", func(t *testing.T) {
		fi := FeedItem{
			Description: "description",
			FeedTitle:   "feedTitle",
			FeedURL:     "feedURL",
			ImageURL:    "imageURL",
			IconURL:     "iconURL",
			ItemURL:     "itemURL",
			Published:   time.Now().UTC(),
			Title:       "title",
		}
		m := Message{
			Timestamp: time.Now().UTC(),
			Item:      fi,
		}
		b, err := m.toBytes()
		require.NoError(t, err)
		m2, err := newMessageFromBytes(b)
		require.NoError(t, err)
		assert.True(t, reflect.DeepEqual(m, m2))
	})
	t.Run("can serialize message for item with zone offset of minus one minute", func(t *testing.T) {
		// given
		feed := &gofeed.Feed{Title: "title"}
		published := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("", -60))
		item := &gofeed.Item{
			Content:         "content",
			PublishedParsed: &published,
		}
		m, err := newMessage("dummy", feed, item, false)
		require.NoError(t, err)
		// when
		b, err := m.toBytes()
		// then
		require.NoError(t, err)
		m2, err := newMessageFromBytes(b)
		require.NoError(t, err)
		assert.True(t, published.Equal(m2.Item.Published))
	})
}
