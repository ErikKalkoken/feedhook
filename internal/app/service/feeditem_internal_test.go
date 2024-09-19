package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFeedItem(t *testing.T) {
	t.Run("can generate webhook payload from item", func(t *testing.T) {
		published := time.Now().UTC()
		fi := FeedItem{
			Description: "description",
			FeedName:    "feedName",
			FeedTitle:   "feedTitle",
			FeedURL:     "feedURL",
			ImageURL:    "imageURL",
			IconURL:     "iconURL",
			ItemURL:     "itemURL",
			Published:   published,
			Title:       "title",
		}
		x, err := fi.ToDiscordMessage(false)
		if assert.NoError(t, err) {
			assert.NotEqual(t, "", x.Username)
			assert.NotEqual(t, "", x.AvatarURL)
			em := x.Embeds[0]
			assert.Equal(t, "description", em.Description)
			assert.Equal(t, "feedTitle", em.Author.Name)
			assert.Equal(t, "feedURL", em.Author.URL)
			assert.Equal(t, "iconURL", em.Author.IconURL)
			assert.Equal(t, "imageURL", em.Image.URL)
			assert.Equal(t, "itemURL", em.URL)
			assert.Equal(t, published.Format(time.RFC3339), em.Timestamp)
			assert.Equal(t, "title", em.Title)
			assert.Equal(t, "feedName", em.Footer.Text)
		}
	})
	t.Run("can add UPDATE tag to title", func(t *testing.T) {
		published := time.Now().UTC()
		fi := FeedItem{
			IsUpdated: true,
			Published: published,
			Title:     "title",
		}
		x, err := fi.ToDiscordMessage(false)
		if assert.NoError(t, err) {
			em := x.Embeds[0]
			assert.Equal(t, "UPDATED: title", em.Title)
		}
	})
	t.Run("can remove img tags from description", func(t *testing.T) {
		fi := FeedItem{Description: `alpha <img src="abc">bravo</img> charlie`}
		x, err := fi.ToDiscordMessage(false)
		if assert.NoError(t, err) {
			assert.Equal(t, "alpha bravo charlie", x.Embeds[0].Description)
		}
	})
	t.Run("can sanitize invalid URLs in description", func(t *testing.T) {
		fi := FeedItem{Description: `<a href="https://www.xgoogle.com">https://www.google.com</a>`}
		x, err := fi.ToDiscordMessage(false)
		if assert.NoError(t, err) {
			assert.Equal(t, "[Link](https://www.xgoogle.com)", x.Embeds[0].Description)
		}
	})
	t.Run("should not impact valid URLs", func(t *testing.T) {
		fi := FeedItem{Description: `<a href="https://www.google.com">Google</a>`}
		x, err := fi.ToDiscordMessage(false)
		if assert.NoError(t, err) {
			assert.Equal(t, "[Google](https://www.google.com)", x.Embeds[0].Description)
		}
	})
	t.Run("can disable branding", func(t *testing.T) {
		fi := FeedItem{Description: "description"}
		x, err := fi.ToDiscordMessage(true)
		if assert.NoError(t, err) {
			assert.Equal(t, "", x.Username)
			assert.Equal(t, "", x.AvatarURL)
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
