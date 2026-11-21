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

func TestFromFile(t *testing.T) {
	t.Run("should return parsed config", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		err := os.WriteFile(p, []byte(toml), 0644)
		require.NoError(t, err)
		// when
		cf, err := config.FromFile(p)
		// then
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
	})
	t.Run("should return err when path is a directory", func(t *testing.T) {
		// when
		_, err := config.FromFile(t.TempDir())
		// then
		require.Error(t, err)
	})
	t.Run("should return err when file does not exist", func(t *testing.T) {
		// when
		_, err := config.FromFile(filepath.Join(t.TempDir(), "config.toml"))
		// then
		require.ErrorIs(t, err, os.ErrNotExist)
	})
	t.Run("should return err when config has unknown keys", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		err := os.WriteFile(p, []byte(toml+"\n[unknown]\nloglevl = \"DEBUG\"\n"), 0644)
		require.NoError(t, err)
		// when
		_, err = config.FromFile(p)
		// then
		require.ErrorContains(t, err, "unknown.loglevl")
	})
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

func TestValidation(t *testing.T) {
	const webhook = `
[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"
`
	const feed = `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`
	cases := []struct {
		name     string
		toml     string
		wantErrs []string
	}{
		{
			name: "webhook has no name",
			toml: `
[[webhooks]]
url = "https://www.example.com/webhook"
` + feed,
			wantErrs: []string{
				"webhook #1 has no name",
				`feed "Feed 1": invalid webhook: "hook-1"`,
			},
		},
		{
			name: "multiple webhooks have no name",
			toml: webhook + `
[[webhooks]]
url = "https://www.example.com/webhook-2"

[[webhooks]]
url = "https://www.example.com/webhook-3"
` + feed,
			wantErrs: []string{
				"webhook #2 has no name",
				"webhook #3 has no name",
			},
		},
		{
			name: "multiple webhooks have no url",
			toml: `
[[webhooks]]
name = "hook-1"

[[webhooks]]
name = "hook-2"
` + feed,
			wantErrs: []string{
				`webhook "hook-1" has no url`,
				`webhook "hook-2" has no url`,
			},
		},
		{
			name: "webhook has no url",
			toml: `
[[webhooks]]
name = "hook-1"
` + feed,
			wantErrs: []string{`webhook "hook-1" has no url`},
		},
		{
			name: "webhook has invalid url",
			toml: `
[[webhooks]]
name = "hook-1"
url = "invalid"
` + feed,
			wantErrs: []string{`webhook "hook-1" has invalid url`},
		},
		{
			name: "webhook name not unique",
			toml: webhook + `
[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook-2"
` + feed,
			wantErrs: []string{`webhook name "hook-1" not unique`},
		},
		{
			name: "webhook url not unique",
			toml: webhook + `
[[webhooks]]
name = "hook-2"
url = "https://www.example.com/webhook"
` + feed,
			wantErrs: []string{`webhook "hook-2": url not unique`},
		},
		{
			name:     "no feeds defined",
			toml:     webhook,
			wantErrs: []string{"no feeds defined"},
		},
		{
			name: "feed has no name",
			toml: webhook + `
[[feeds]]
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`,
			wantErrs: []string{"feed #1 has no name"},
		},
		{
			name: "feed name not unique",
			toml: webhook + feed + `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed-2.rss"
webhook = "hook-1"
`,
			wantErrs: []string{`feed name "Feed 1" not unique`},
		},
		{
			name: "feed has no webhook",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
`,
			wantErrs: []string{`feed "Feed 1" has no webhook`},
		},
		{
			name: "feed has invalid webhook",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "unknown"
`,
			wantErrs: []string{`feed "Feed 1": invalid webhook: "unknown"`},
		},
		{
			name: "feed has no url",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
webhook = "hook-1"
`,
			wantErrs: []string{`feed "Feed 1" has no url`},
		},
		{
			name: "feed has invalid url",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
url = "invalid"
webhook = "hook-1"
`,
			wantErrs: []string{`feed "Feed 1" has invalid url`},
		},
		{
			name: "multiple errors",
			toml: `
[app]
oldest = -60

[[webhooks]]
name = "hook-1"

[[feeds]]
name = "Feed 1"
url = "invalid"
webhook = "unknown"
`,
			wantErrs: []string{
				`webhook "hook-1" has no url`,
				`feed "Feed 1": invalid webhook: "unknown"`,
				`feed "Feed 1" has invalid url`,
				"oldest must be -1 or greater, got -60",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fromString(t, tc.toml)
			require.Error(t, err)
			for _, want := range tc.wantErrs {
				assert.ErrorContains(t, err, want)
			}
			var joined interface{ Unwrap() []error }
			require.ErrorAs(t, err, &joined)
			assert.Len(t, joined.Unwrap(), len(tc.wantErrs))
		})
	}
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
