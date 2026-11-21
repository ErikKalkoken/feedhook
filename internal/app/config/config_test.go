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
		cf, warnings, failures, err := config.FromFile(p)
		// then
		require.NoError(t, err)
		assert.Empty(t, failures)
		assert.Empty(t, warnings)
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
		_, _, _, err := config.FromFile(t.TempDir())
		// then
		require.Error(t, err)
	})
	t.Run("should return err when file does not exist", func(t *testing.T) {
		// when
		_, _, _, err := config.FromFile(filepath.Join(t.TempDir(), "config.toml"))
		// then
		require.ErrorIs(t, err, os.ErrNotExist)
	})
	t.Run("should return only err when TOML is invalid", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		err := os.WriteFile(p, []byte("[[webhooks]\n"), 0644)
		require.NoError(t, err)
		// when
		_, warnings, failures, err := config.FromFile(p)
		// then
		require.Error(t, err)
		assert.Empty(t, failures)
		assert.Empty(t, warnings)
	})
	t.Run("should return failures and err when config is invalid", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		err := os.WriteFile(p, []byte("[[webhooks]]\nname = \"hook-1\"\nurl = \"https://www.example.com/webhook\"\n"), 0644)
		require.NoError(t, err)
		// when
		_, _, failures, err := config.FromFile(p)
		// then
		require.EqualError(t, err, "invalid config")
		assert.Equal(t, []string{"no feeds defined"}, failures)
	})
	t.Run("should warn about unknown keys", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		err := os.WriteFile(p, []byte(toml+"\n[unknown]\nloglevl = \"DEBUG\"\n"), 0644)
		require.NoError(t, err)
		// when
		_, warnings, _, err := config.FromFile(p)
		// then
		require.NoError(t, err)
		assert.Contains(t, warnings, "unknown key: unknown.loglevl")
	})
	t.Run("should warn about unknown keys when config is invalid", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		data := `
[[webhooks]]
name = "hook-1"
ulr = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`
		err := os.WriteFile(p, []byte(data), 0644)
		require.NoError(t, err)
		// when
		_, warnings, failures, err := config.FromFile(p)
		// then
		require.Error(t, err)
		assert.Equal(t, []string{`webhook "hook-1" has no url`}, failures)
		assert.Equal(t, []string{"unknown key: webhooks.ulr"}, warnings)
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
			cf, _, err := fromString(t, configWithApp(tc.oldest))
			require.NoError(t, err)
			assert.Equal(t, tc.want, cf.App.Oldest)
		})
	}
	t.Run("should return error for values below -1", func(t *testing.T) {
		_, failures, err := fromString(t, configWithApp("oldest = -60"))
		require.Error(t, err)
		assert.Equal(t, []string{"oldest must be -1 or greater, got -60"}, failures)
	})
}

func TestAppSettings(t *testing.T) {
	t.Run("should warn about negative timeout and use default", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(p, []byte(configWithApp("timeout = -5")), 0644))
		cfg, warnings, failures, err := config.FromFile(p)
		require.NoError(t, err)
		assert.Empty(t, failures)
		assert.Equal(t, []string{"timeout must be 0 or greater, got -5, using 30"}, warnings)
		assert.Equal(t, 30, cfg.App.Timeout)
	})
	t.Run("should warn about negative ticker and use default", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(p, []byte(configWithApp("ticker = -5")), 0644))
		cfg, warnings, failures, err := config.FromFile(p)
		require.NoError(t, err)
		assert.Empty(t, failures)
		assert.Equal(t, []string{"ticker must be 0 or greater, got -5, using 30"}, warnings)
		assert.Equal(t, 30, cfg.App.Ticker)
	})
	t.Run("should warn about unknown loglevel", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(p, []byte(configWithApp(`loglevel = "VERBOSE"`)), 0644))
		_, warnings, failures, err := config.FromFile(p)
		require.NoError(t, err)
		assert.Empty(t, failures)
		assert.Equal(t, []string{`unknown loglevel "VERBOSE", using INFO`}, warnings)
	})
	cases := []string{
		"",
		"timeout = 0\nticker = 0",
		`loglevel = ""`,
		`loglevel = "warn"`,
	}
	for _, line := range cases {
		t.Run(fmt.Sprintf("should accept %q without warnings", line), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.toml")
			require.NoError(t, os.WriteFile(p, []byte(configWithApp(line)), 0644))
			_, warnings, failures, err := config.FromFile(p)
			require.NoError(t, err)
			assert.Empty(t, failures)
			assert.Empty(t, warnings)
		})
	}
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
		name         string
		toml         string
		wantFailures []string
	}{
		{
			name: "webhook has no name",
			toml: `
[[webhooks]]
url = "https://www.example.com/webhook"
` + feed,
			wantFailures: []string{
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
			wantFailures: []string{
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
			wantFailures: []string{
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
			wantFailures: []string{`webhook "hook-1" has no url`},
		},
		{
			name: "webhook has invalid url",
			toml: `
[[webhooks]]
name = "hook-1"
url = "invalid"
` + feed,
			wantFailures: []string{`webhook "hook-1" has invalid url`},
		},
		{
			name: "webhook name not unique",
			toml: webhook + `
[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook-2"
` + feed,
			wantFailures: []string{`webhook name "hook-1" not unique`},
		},
		{
			name: "webhook url not unique",
			toml: webhook + `
[[webhooks]]
name = "hook-2"
url = "https://www.example.com/webhook"
` + feed,
			wantFailures: []string{`webhook "hook-2": url not unique`},
		},
		{
			name:         "no feeds defined",
			toml:         webhook,
			wantFailures: []string{"no feeds defined"},
		},
		{
			name: "feed has no name",
			toml: webhook + `
[[feeds]]
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`,
			wantFailures: []string{"feed #1 has no name"},
		},
		{
			name: "feed name not unique",
			toml: webhook + feed + `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed-2.rss"
webhook = "hook-1"
`,
			wantFailures: []string{`feed name "Feed 1" not unique`},
		},
		{
			name: "feed has no webhook",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
`,
			wantFailures: []string{`feed "Feed 1" has no webhook`},
		},
		{
			name: "feed has invalid webhook",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "unknown"
`,
			wantFailures: []string{`feed "Feed 1": invalid webhook: "unknown"`},
		},
		{
			name: "feed has no url",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
webhook = "hook-1"
`,
			wantFailures: []string{`feed "Feed 1" has no url`},
		},
		{
			name: "feed has invalid url",
			toml: webhook + `
[[feeds]]
name = "Feed 1"
url = "invalid"
webhook = "hook-1"
`,
			wantFailures: []string{`feed "Feed 1" has invalid url`},
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
			wantFailures: []string{
				`webhook "hook-1" has no url`,
				`feed "Feed 1": invalid webhook: "unknown"`,
				`feed "Feed 1" has invalid url`,
				"oldest must be -1 or greater, got -60",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, failures, err := fromString(t, tc.toml)
			require.Error(t, err)
			require.Len(t, failures, len(tc.wantFailures))
			for i, want := range tc.wantFailures {
				assert.Contains(t, failures[i], want)
			}
		})
	}
}

func configWithApp(line string) string {
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

func fromString(t *testing.T, s string) (config.Config, []string, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(s), 0644))
	cf, _, failures, err := config.FromFile(p)
	return cf, failures, err
}
