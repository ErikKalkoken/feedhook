package config

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	t.Run("should return no error when config is valid", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook1",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				Name:    "feed1",
				URL:     "https://www.example.com/url2",
				Webhook: "hook1",
			}},
		}
		assert.NoError(t, parseConfig(&cf))
	})
	t.Run("should return error when no feeds defined", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "name",
				URL:  "https://www.example.com/url",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when hook has no URL", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "name",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when hook has no name", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				URL: "https://www.example.com/url",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no name", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				URL:     "https://www.example.com/url2",
				Webhook: "hook",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no webhook", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				Name: "feed",
				URL:  "https://www.example.com/url2",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no URL", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				Name:    "name",
				Webhook: "hook",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed defines unknown hook", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "name",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				Name:    "dummy",
				URL:     "https://www.example.com/url2",
				Webhook: "unknown",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when a webhook URL is invalid", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook1",
				URL:  "invalid",
			}},
			Feeds: []ConfigFeed{{
				Name:    "feed1",
				URL:     "https://www.example.com/url2",
				Webhook: "hook1",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when a feed URL is invalid", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook1",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				Name:    "feed1",
				URL:     "invalid",
				Webhook: "hook1",
			}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should set app defaults when missing", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook1",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{{
				Name:    "feed1",
				URL:     "https://www.example.com/url2",
				Webhook: "hook1",
			}},
		}
		err := parseConfig(&cf)
		require.NoError(t, err)
		assert.Equal(t, timeoutDefault, cf.App.Timeout)
		assert.Equal(t, oldestDefault, cf.App.Oldest)
		assert.Equal(t, tickerDefault, cf.App.Ticker)
	})
	t.Run("should return error when webhook names are not unique", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{
				{
					Name: "hook1",
					URL:  "https://www.example.com/url1",
				},
				{
					Name: "hook1",
					URL:  "https://www.example.com/url2",
				},
			},
			Feeds: []ConfigFeed{{
				Name:    "feed1",
				URL:     "https://www.example.com/url2",
				Webhook: "hook1",
			}},
		}
		assert.ErrorContains(t, parseConfig(&cf), "webhook name hook1 not unique")
	})
	t.Run("should return error when webhook URLs are not unique", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{
				{
					Name: "hook1",
					URL:  "https://www.example.com/url1",
				},
				{
					Name: "hook2",
					URL:  "https://www.example.com/url1",
				},
			},
			Feeds: []ConfigFeed{{
				Name:    "feed1",
				URL:     "https://www.example.com/url2",
				Webhook: "hook1",
			}},
		}
		assert.ErrorContains(t, parseConfig(&cf), "webhook hook2: url not unique")
	})
	t.Run("should return error when feed names are not unique", func(t *testing.T) {
		cf := Config{
			Webhooks: []ConfigWebhook{{
				Name: "hook1",
				URL:  "https://www.example.com/url1",
			}},
			Feeds: []ConfigFeed{
				{
					Name:    "feed1",
					URL:     "https://www.example.com/url2",
					Webhook: "hook1",
				},
				{
					Name:    "feed1",
					URL:     "https://www.example.com/url3",
					Webhook: "hook1",
				},
			},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return log level from config", func(t *testing.T) {
		cases := []struct {
			in   string
			want slog.Level
		}{
			{"ERROR", slog.LevelError},
			{"WARN", slog.LevelWarn},
			{"INFO", slog.LevelInfo},
			{"DEBUG", slog.LevelDebug},
			{"", logLevelDefault},
			{"XXX", logLevelDefault},
		}
		for _, tc := range cases {
			cf := Config{App: ConfigApp{LogLevel: tc.in}}
			assert.Equal(t, tc.want, cf.App.LoggerLevel())
		}
	})
}

func TestEnabledFeeds(t *testing.T) {
	t.Run("should return enabled feeds only 1", func(t *testing.T) {
		cf := Config{
			Feeds: []ConfigFeed{
				{
					Name:    "feed1",
					URL:     "https://www.example.com/url1",
					Webhook: "hook1",
				},
				{
					Name:    "feed2",
					URL:     "https://www.example.com/url2",
					Webhook: "hook1",
				},
			},
		}
		f := cf.EnabledFeeds()
		assert.Len(t, f, 2)
	})
	t.Run("should return enabled feeds only 2", func(t *testing.T) {
		cf := Config{
			Feeds: []ConfigFeed{
				{
					Name:     "feed1",
					URL:      "https://www.example.com/url1",
					Webhook:  "hook1",
					Disabled: true,
				},
				{
					Name:    "feed2",
					URL:     "https://www.example.com/url2",
					Webhook: "hook1",
				},
			},
		}
		f := cf.EnabledFeeds()
		assert.Len(t, f, 1)
	})
}

func TestFeedNames(t *testing.T) {
	t.Run("should return names of all feeds including disabled ones", func(t *testing.T) {
		cf := Config{
			Feeds: []ConfigFeed{
				{
					Name:    "feed1",
					URL:     "https://www.example.com/url1",
					Webhook: "hook1",
				},
				{
					Name:     "feed2",
					URL:      "https://www.example.com/url2",
					Webhook:  "hook1",
					Disabled: true,
				},
			},
		}
		got := cf.FeedNames()
		assert.Equal(t, []string{"feed1", "feed2"}, got)
	})
	t.Run("should return empty slice when no feeds are configured", func(t *testing.T) {
		cf := Config{}
		got := cf.FeedNames()
		assert.Empty(t, got)
	})
}

func TestFeedByName(t *testing.T) {
	cf := Config{
		Feeds: []ConfigFeed{
			{
				Name:    "feed1",
				URL:     "https://www.example.com/url1",
				Webhook: "hook1",
			},
			{
				Name:    "feed2",
				URL:     "https://www.example.com/url2",
				Webhook: "hook1",
			},
		},
	}
	t.Run("should return existing feed", func(t *testing.T) {
		f, ok := cf.FeedByName("feed2")
		require.True(t, ok)
		assert.Equal(t, "https://www.example.com/url2", f.URL)
	})
	t.Run("should report false for unknown feed", func(t *testing.T) {
		_, ok := cf.FeedByName("unknown")
		assert.False(t, ok)
	})
}

func TestWebhookByName(t *testing.T) {
	cf := Config{
		Webhooks: []ConfigWebhook{
			{
				Name: "hook1",
				URL:  "https://www.example.com/hook1",
			},
			{
				Name: "hook2",
				URL:  "https://www.example.com/hook2",
			},
		},
	}
	t.Run("should return existing webhook", func(t *testing.T) {
		w, ok := cf.WebhookByName("hook2")
		require.True(t, ok)
		assert.Equal(t, "https://www.example.com/hook2", w.URL)
	})
	t.Run("should report false for unknown webhook", func(t *testing.T) {
		_, ok := cf.WebhookByName("unknown")
		assert.False(t, ok)
	})
}
