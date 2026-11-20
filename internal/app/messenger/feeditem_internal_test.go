package messenger

import (
	"fmt"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFeedItem(t *testing.T) {
	t.Run("should create feed item from feed and item", func(t *testing.T) {
		// given
		published := time.Date(2026, 11, 20, 12, 30, 0, 0, time.UTC)
		feed := &gofeed.Feed{
			Title: "feedTitle",
			Link:  "http://www.example.com/feed",
			Image: &gofeed.Image{
				URL: "http://www.example.com/icon",
			},
		}
		item := &gofeed.Item{
			Title:           "title",
			Description:     "description",
			Link:            "http://www.example.com/item",
			PublishedParsed: &published,
			Image: &gofeed.Image{
				URL: "http://www.example.com/image",
			},
		}
		// when
		fi, err := NewFeedItem("feedName", feed, item, true)
		// then
		require.NoError(t, err)
		assert.Equal(t, "description", fi.Description)
		assert.Equal(t, "feedName", fi.FeedName)
		assert.Equal(t, "feedTitle", fi.FeedTitle)
		assert.Equal(t, "http://www.example.com/feed", fi.FeedURL)
		assert.Equal(t, "http://www.example.com/icon", fi.IconURL)
		assert.Equal(t, "http://www.example.com/image", fi.ImageURL)
		assert.True(t, fi.IsUpdated)
		assert.Equal(t, "http://www.example.com/item", fi.ItemURL)
		assert.True(t, published.Equal(fi.Published))
		assert.Equal(t, "title", fi.Title)
	})

	t.Run("should prefer content over description", func(t *testing.T) {
		// given
		item := &gofeed.Item{
			Content:     "content",
			Description: "description",
		}
		// when
		fi, err := NewFeedItem("feedName", &gofeed.Feed{}, item, false)
		// then
		require.NoError(t, err)
		assert.Equal(t, "content", fi.Description)
	})

	t.Run("should leave optional fields empty when not provided", func(t *testing.T) {
		// given
		item := &gofeed.Item{
			Title: "title",
		}
		// when
		fi, err := NewFeedItem("feedName", &gofeed.Feed{}, item, false)
		// then
		require.NoError(t, err)
		assert.Equal(t, "", fi.IconURL)
		assert.Equal(t, "", fi.ImageURL)
		assert.True(t, fi.Published.IsZero())
		assert.False(t, fi.IsUpdated)
	})

	t.Run("should return error when feed is nil", func(t *testing.T) {
		_, err := NewFeedItem("feedName", nil, &gofeed.Item{}, false)
		assert.Error(t, err)
	})

	t.Run("should return error when item is nil", func(t *testing.T) {
		_, err := NewFeedItem("feedName", &gofeed.Feed{}, nil, false)
		assert.Error(t, err)
	})
}

func TestFeedItem(t *testing.T) {
	t.Run("can generate webhook payload from item", func(t *testing.T) {
		published := time.Now().UTC()
		fi := FeedItem{
			Description: "description",
			FeedName:    "feedName",
			FeedTitle:   "feedTitle",
			FeedURL:     "http://www.example.com/feed",
			ImageURL:    "http://www.example.com/image",
			IconURL:     "http://www.example.com/icon",
			ItemURL:     "http://www.example.com/item",
			Published:   published,
			Title:       "title",
		}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		assert.NotEqual(t, "", x.Username)
		assert.NotEqual(t, "", x.AvatarURL)
		em := x.Embeds[0]
		assert.Equal(t, "description", em.Description)
		assert.Equal(t, "feedTitle", em.Author.Name)
		assert.Equal(t, "http://www.example.com/feed", em.Author.URL)
		assert.Equal(t, "http://www.example.com/icon", em.Author.IconURL)
		assert.Equal(t, "http://www.example.com/image", em.Image.URL)
		assert.Equal(t, "http://www.example.com/item", em.URL)
		assert.True(t, published.Equal(em.Timestamp))
		assert.Equal(t, "title", em.Title)
		assert.Equal(t, "feedName", em.Footer.Text)
	})
	t.Run("can add UPDATE tag to title", func(t *testing.T) {
		published := time.Now().UTC()
		fi := FeedItem{
			IsUpdated: true,
			Published: published,
			Title:     "title",
		}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		em := x.Embeds[0]
		assert.Equal(t, "UPDATED: title", em.Title)
	})
	t.Run("can remove img tags from description", func(t *testing.T) {
		fi := FeedItem{Description: `alpha <img src="abc">bravo</img> charlie`}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		assert.Equal(t, "alpha bravo charlie", x.Embeds[0].Description)
	})
	t.Run("can remove figure tags from description", func(t *testing.T) {
		fi := FeedItem{Description: `alpha <figure><a href="https:///www.google.com">bravo</a></figure> charlie`}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		assert.Equal(t, "alpha  charlie", x.Embeds[0].Description)
	})
	t.Run("can sanitize mailto links from description", func(t *testing.T) {
		fi := FeedItem{Description: `<a href="mailto:info@example.com">info</a>`}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		assert.Equal(t, "info", x.Embeds[0].Description)
	})
	t.Run("can sanitize invalid URLs in description", func(t *testing.T) {
		fi := FeedItem{Description: `<a href="https://www.xgoogle.com">https://www.google.com</a>`}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		assert.Equal(t, "[Link](https://www.xgoogle.com)", x.Embeds[0].Description)
	})
	t.Run("should not impact valid URLs", func(t *testing.T) {
		fi := FeedItem{Description: `<a href="https://www.google.com">Google</a>`}
		x, err := fi.ToDiscordMessage(false)
		require.NoError(t, err)
		assert.Equal(t, "[Google](https://www.google.com)", x.Embeds[0].Description)
	})
	t.Run("can disable branding", func(t *testing.T) {
		fi := FeedItem{Description: "description"}
		x, err := fi.ToDiscordMessage(true)
		require.NoError(t, err)
		assert.Equal(t, "", x.Username)
		assert.Equal(t, "", x.AvatarURL)
	})
}

func TestTruncateString(t *testing.T) {
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
			assert.GreaterOrEqual(t, tc.max, len([]rune(got)))
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
