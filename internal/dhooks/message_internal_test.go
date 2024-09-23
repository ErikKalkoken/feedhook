package dhooks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLength(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"alpha 😀 boy", 11},
		{"alpha boy", 9},
		{"", 0},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			got := length(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidation(t *testing.T) {
	validURL := "https://www.googl.com"
	invalidURL := "//invalid/server/abc"
	t.Run("author URL", func(t *testing.T) {
		x1 := Embed{Author: EmbedAuthor{URL: validURL}}
		assert.NoError(t, x1.validate())
		x2 := Embed{Author: EmbedAuthor{URL: invalidURL}}
		assert.ErrorIs(t, x2.validate(), ErrInvalidMessage)
	})
	t.Run("author icon URL", func(t *testing.T) {
		x1 := Embed{Author: EmbedAuthor{IconURL: validURL}}
		assert.NoError(t, x1.validate())
		x2 := Embed{Author: EmbedAuthor{IconURL: invalidURL}}
		assert.ErrorIs(t, x2.validate(), ErrInvalidMessage)
	})
	t.Run("image URL", func(t *testing.T) {
		x1 := Embed{Image: EmbedImage{URL: validURL}}
		assert.NoError(t, x1.validate())
		x2 := Embed{Image: EmbedImage{URL: invalidURL}}
		assert.ErrorIs(t, x2.validate(), ErrInvalidMessage)
	})
	t.Run("footer icon URL", func(t *testing.T) {
		x1 := Embed{Footer: EmbedFooter{IconURL: validURL}}
		assert.NoError(t, x1.validate())
		x2 := Embed{Footer: EmbedFooter{IconURL: invalidURL}}
		assert.ErrorIs(t, x2.validate(), ErrInvalidMessage)
	})
	t.Run("thumbnail URL", func(t *testing.T) {
		x1 := Embed{Thumbnail: EmbedThumbnail{URL: validURL}}
		assert.NoError(t, x1.validate())
		x2 := Embed{Thumbnail: EmbedThumbnail{URL: invalidURL}}
		assert.ErrorIs(t, x2.validate(), ErrInvalidMessage)
	})
}
