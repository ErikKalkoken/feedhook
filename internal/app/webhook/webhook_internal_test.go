package webhook

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestMakePayload(t *testing.T) {
	t.Run("can create payload", func(t *testing.T) {
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		x, err := makePayload(feed, item)
		if assert.NoError(t, err) {
			em := x.Embeds[0]
			assert.Equal(t, "content", em.Description)
		}
	})

	t.Run("can remove img tags", func(t *testing.T) {
		feed := gofeed.Feed{Title: "title"}
		now := time.Now()
		item := gofeed.Item{Content: `alpha <img src="abc">bravo</img> charlie`, PublishedParsed: &now}
		x, err := makePayload(&feed, &item)
		if assert.NoError(t, err) {
			em := x.Embeds[0]
			assert.Equal(t, "alpha bravo charlie", em.Description)
		}
	})
}
