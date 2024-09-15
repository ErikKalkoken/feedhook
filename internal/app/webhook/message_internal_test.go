package webhook

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ErikKalkoken/feedforward/internal/discordhook"
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

func TestSerialization(t *testing.T) {
	t.Run("can serialize and de-serialize payload", func(t *testing.T) {
		pl := discordhook.WebhookPayload{
			Content: "content",
			Embeds: []discordhook.Embed{{
				Author: discordhook.EmbedAuthor{Name: "name", IconURL: "iconURL", URL: "url"},
				Title:  "In Fight for Congress, Democrats Run as ‘Team Normal,’ Casting G.O.P. as ‘Weird’",
			}},
		}
		m := Message{
			Title:     "In Fight for Congress, Democrats Run as ‘Team Normal,’ Casting G.O.P. as ‘Weird’",
			Feed:      "feed",
			Timestamp: time.Now().UTC(),
			Payload:   pl,
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

func TestEllipsis(t *testing.T) {
	cases := []struct {
		in        string
		max       int
		want      string
		truncated bool
	}{
		{"alpha 😀 boy", 11, "alpha 😀 boy", false},
		{"alpha 😀 boy", 100, "alpha 😀 boy", false},
		{"alpha 😀 boy", 10, "alpha 😀...", true},
		{"alpha boy", 3, "...", true},
		{"", 3, "", false},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			got, truncated := truncateString(tc.in, tc.max)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.truncated, truncated)
		})
	}

	t.Run("should panic when maxLen is below 3", func(t *testing.T) {
		assert.Panics(t, func() {
			truncateString("xyz", 2)
		})
		assert.Panics(t, func() {
			truncateString("xyz", -1)
		})
	})
}
