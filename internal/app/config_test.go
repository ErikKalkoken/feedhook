package app_test

import (
	"example/feedforward/internal/app"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var toml = `
[app]
discordTimeout = 2
feedTimeout = 3

[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`

func TestConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	cf, err := app.ReadConfig(p)
	if assert.NoError(t, err) {
		assert.Equal(t, cf.App.DiscordTimeout, 2)
		assert.Equal(t, cf.App.FeedTimeout, 3)
		assert.Equal(t, cf.Webhooks[0].Name, "hook-1")
		assert.Equal(t, cf.Webhooks[0].URL, "https://www.example.com/webhook")
		assert.Equal(t, cf.Feeds[0].Name, "Feed 1")
		assert.Equal(t, cf.Feeds[0].URL, "https://www.example.com/feed.rss")
		assert.Equal(t, cf.Feeds[0].Webhook, "hook-1")
	}
}
