package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConfig(t *testing.T) {
	t.Run("should return no error when config is valid", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds:    []configFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.NoError(t, parseConfig(&cf))
	})
	t.Run("should return error when no feeds defined", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "name", URL: "https://www.example.com/url"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when no hook has no https://www.example.com/url", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "name"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when no hook has no name", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{URL: "https://www.example.com/url"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no name", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook", URL: "https://www.example.com/url1"}},
			Feeds:    []configFeed{{URL: "https://www.example.com/url2", Webhook: "hook"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed has no https://www.example.com/url", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook", URL: "https://www.example.com/url1"}},
			Feeds:    []configFeed{{Name: "name", Webhook: "hook"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed defines unknown hook", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "name", URL: "https://www.example.com/url1"}},
			Feeds:    []configFeed{{Name: "dummy", URL: "https://www.example.com/url2", Webhook: "unknown"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when a webhook url is invalid", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook1", URL: "invalid"}},
			Feeds:    []configFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when a feed url is invalid", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds:    []configFeed{{Name: "feed1", URL: "invalid", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return default for timeouts when missing", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds:    []configFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		err := parseConfig(&cf)
		if assert.NoError(t, err) {
			assert.Equal(t, cf.App.DiscordTimeout, discordTimeoutDefault)
			assert.Equal(t, cf.App.FeedTimeout, feedTimeoutDefault)
		}
	})
	t.Run("should return error when webhook names not unique", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{
				{Name: "hook1", URL: "https://www.example.com/url1"},
				{Name: "hook1", URL: "https://www.example.com/url2"},
			},
			Feeds: []configFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when webhook URLs not unique", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{
				{Name: "hook1", URL: "https://www.example.com/url1"},
				{Name: "hook2", URL: "https://www.example.com/url1"},
			},
			Feeds: []configFeed{{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"}},
		}
		assert.Error(t, parseConfig(&cf))
	})
	t.Run("should return error when feed names are not unique", func(t *testing.T) {
		cf := MyConfig{
			Webhooks: []configWebhook{{Name: "hook1", URL: "https://www.example.com/url1"}},
			Feeds: []configFeed{
				{Name: "feed1", URL: "https://www.example.com/url2", Webhook: "hook1"},
				{Name: "feed1", URL: "https://www.example.com/url3", Webhook: "hook1"},
			},
		}
		assert.Error(t, parseConfig(&cf))
	})

}
