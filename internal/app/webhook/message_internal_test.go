package webhook

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestMakeMessage(t *testing.T) {
	t.Run("can create new message", func(t *testing.T) {
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		x, err := newMessage("dummy", feed, item)
		if assert.NoError(t, err) {
			em := x.Payload.Embeds[0]
			assert.Equal(t, "content", em.Description)
		}
	})
	t.Run("can remove img tags", func(t *testing.T) {
		feed := gofeed.Feed{Title: "title"}
		now := time.Now()
		item := gofeed.Item{Content: `alpha <img src="abc">bravo</img> charlie`, PublishedParsed: &now}
		x, err := newMessage("dummy", &feed, &item)
		if assert.NoError(t, err) {
			em := x.Payload.Embeds[0]
			assert.Equal(t, "alpha bravo charlie", em.Description)
		}
	})
}

func TestSerialize(t *testing.T) {
	t.Run("can serialize and de-serialize payload", func(t *testing.T) {
		pl := webhookPayload{
			Content: "content",
			Embeds:  []embed{{Title: "title"}},
		}
		m := message{
			Title:   "title",
			Feed:    "feed",
			Payload: pl,
		}
		b, err := m.toBytes()
		if assert.NoError(t, err) {
			m2, err := newMessageFromBytes(b)
			if assert.NoError(t, err) {
				assert.Equal(t, m, m2)
			}
		}
	})
}
