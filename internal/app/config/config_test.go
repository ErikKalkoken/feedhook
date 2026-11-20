package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var toml = `
[app]
timeout = 2
oldest = 3
ticker = 4
loglevel = "DEBUG"

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
	cf, err := config.FromFile(p)
	require.NoError(t, err)
	assert.Equal(t, 2, cf.App.Timeout)
	assert.Equal(t, 3, cf.App.Oldest)
	assert.Equal(t, 4, cf.App.Ticker)
	assert.Equal(t, "DEBUG", cf.App.LogLevel)
	assert.Equal(t, "hook-1", cf.Webhooks[0].Name)
	assert.Equal(t, "https://www.example.com/webhook", cf.Webhooks[0].URL)
	assert.Equal(t, "Feed 1", cf.Feeds[0].Name)
	assert.Equal(t, "https://www.example.com/feed.rss", cf.Feeds[0].URL)
	assert.Equal(t, "hook-1", cf.Feeds[0].Webhook)
}

func TestOldest(t *testing.T) {
	cases := []struct {
		oldest string
		want   int
	}{
		{"oldest = 3600", 3600},
		{"", 7200},
		{"oldest = 0", 7200},
		{"oldest = -1", config.OldestUnlimited},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("should parse %q as %d", tc.oldest, tc.want), func(t *testing.T) {
			cf, err := fromString(t, configWithOldest(tc.oldest))
			require.NoError(t, err)
			assert.Equal(t, tc.want, cf.App.Oldest)
		})
	}
	t.Run("should return error for values below -1", func(t *testing.T) {
		_, err := fromString(t, configWithOldest("oldest = -60"))
		assert.Error(t, err)
	})
}

func configWithOldest(line string) string {
	return fmt.Sprintf(`
[app]
%s

[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`, line)
}

func fromString(t *testing.T, s string) (config.Config, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(s), 0644))
	return config.FromFile(p)
}
