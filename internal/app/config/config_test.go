package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/stretchr/testify/assert"
)

var toml = `
[app]
timeout = 2
oldest = 3
ticker = 4
loglevel = "DEBUG"
db_path = "/path1/alpha"

[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhooks = ["hook-1"]
`

func TestConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	cf, err := config.ReadConfig(p)
	if assert.NoError(t, err) {
		assert.Equal(t, cf.App.Timeout, 2)
		assert.Equal(t, cf.App.Oldest, 3)
		assert.Equal(t, cf.App.Ticker, 4)
		assert.Equal(t, cf.App.LogLevel, "DEBUG")
		assert.Equal(t, cf.App.DBPath, "/path1/alpha")
		assert.Equal(t, cf.Webhooks[0].Name, "hook-1")
		assert.Equal(t, cf.Webhooks[0].URL, "https://www.example.com/webhook")
		assert.Equal(t, cf.Feeds[0].Name, "Feed 1")
		assert.Equal(t, cf.Feeds[0].URL, "https://www.example.com/feed.rss")
		assert.Equal(t, cf.Feeds[0].Webhooks, []string{"hook-1"})
	}
}
