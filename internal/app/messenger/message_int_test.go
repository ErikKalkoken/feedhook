package messenger

import (
	"reflect"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestMessage(t *testing.T) {
	t.Run("can create new message from feed item", func(t *testing.T) {
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		x, err := newMessage("dummy", feed, item, false)
		if assert.NoError(t, err) {
			assert.Equal(t, "content", x.Item.Description)
		}
	})
}

func TestSerialization(t *testing.T) {
	t.Run("can serialize and de-serialize payload", func(t *testing.T) {
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
		if assert.NoError(t, err) {
			m2, err := newMessageFromBytes(b)
			if assert.NoError(t, err) {
				assert.True(t, reflect.DeepEqual(m, m2))
			}
		}
	})
}
