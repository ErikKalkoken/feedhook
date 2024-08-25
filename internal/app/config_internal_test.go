package app

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	t.Run("should return no error when config is valid", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds:    []ConfigFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.NoError(t, parseConfig(&cf))
	})
	t.Run("should return error when no feeds defined", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "name", URL: "https://www.example.com/url"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when no hook has no https://www.example.com/url", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "name"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when no hook has no name", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{URL: "https://www.example.com/url"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no name", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook", URL: "https://www.example.com/url1"}},
			Feeds:    []ConfigFeed{{URL: "https://www.example.com/url2", Webhook: "hook"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no https://www.example.com/url", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook", URL: "https://www.example.com/url1"}},
			Feeds:    []ConfigFeed{{Name: "name", Webhook: "hook"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed defines unknown hook", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "name", URL: "https://www.example.com/url1"}},
			Feeds:    []ConfigFeed{{Name: "dummy", URL: "https://www.example.com/url2", Webhook: "unknown"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when a webhook url is invalid", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook1", URL: "invalid"}},
			Feeds:    []ConfigFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when a feed url is invalid", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds:    []ConfigFeed{{Name: "feed1", URL: "invalid", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should set app defaults when missing", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds:    []ConfigFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		err := parseConfig(&cf)
		if assert.NoError(t, err) {
			assert.Equal(t, cf.App.Timeout, timeoutDefault)
			assert.Equal(t, cf.App.Oldest, oldestDefault)
			assert.Equal(t, cf.App.Ticker, tickerDefault)
		}
	})
	t.Run("should return error when webhook names not unique", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{
				{Name: "hook1", URL: "https://www.example.com/url1"},
				{Name: "hook1", URL: "https://www.example.com/url2"},
			},
			Feeds: []ConfigFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when webhook URLs not unique", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{
				{Name: "hook1", URL: "https://www.example.com/url1"},
				{Name: "hook2", URL: "https://www.example.com/url1"},
			},
			Feeds: []ConfigFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed names are not unique", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds: []ConfigFeed{
				{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"},
				{Name: "feed1", URL: "https://www.example.com/url3", Webhook: "hook1"},
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
			cf := MyConfig{App: ConfigApp{LogLevel: tc.in}}
			assert.Equal(t, tc.want, cf.App.LoggerLevel())
		}
	})
}

func TestEnabledFeeds(t *testing.T) {
	t.Run("should return enabled feeds only 1", func(t *testing.T) {
		cf := MyConfig{
			Feeds: []ConfigFeed{
				{Name: "feed1", URL: "https://www.example.com/url1", Webhook: "hook1"},
				{Name: "feed2", URL: "https://www.example.com/url2", Webhook: "hook1"},
			},
		}
		f := cf.EnabledFeeds()
		assert.Len(t, f, 2)
	})
	t.Run("should return enabled feeds only 2", func(t *testing.T) {
		cf := MyConfig{
			Feeds: []ConfigFeed{
				{Name: "feed1", URL: "https://www.example.com/url1", Webhook: "hook1", Disabled: true},
				{Name: "feed2", URL: "https://www.example.com/url2", Webhook: "hook1"},
			},
		}
		f := cf.EnabledFeeds()
		assert.Len(t, f, 1)
	})
}
