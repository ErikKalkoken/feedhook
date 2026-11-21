// Package config provides the app's configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	timeoutDefault  = 30
	oldestDefault   = 7200
	tickerDefault   = 30
	logLevelDefault = slog.LevelInfo
)

// OldestUnlimited is the value for Oldest to forward items of any age.
const OldestUnlimited = -1

// Config is the app's configuration.
type Config struct {
	App      ConfigApp
	Feeds    []ConfigFeed
	Webhooks []ConfigWebhook
}

// FeedNames returns the names of all configured feeds.
func (c *Config) FeedNames() []string {
	names := make([]string, len(c.Feeds))
	for i, f := range c.Feeds {
		names[i] = f.Name
	}
	return names
}

// EnabledFeeds returns all feeds that are not disabled.
func (c *Config) EnabledFeeds() []ConfigFeed {
	feeds := make([]ConfigFeed, 0)
	for _, f := range c.Feeds {
		if f.Disabled {
			continue
		}
		feeds = append(feeds, f)
	}
	return feeds
}

// FeedByName returns the feed with the given name and reports whether it was found.
func (c *Config) FeedByName(name string) (ConfigFeed, bool) {
	i := slices.IndexFunc(c.Feeds, func(f ConfigFeed) bool { return f.Name == name })
	if i == -1 {
		return ConfigFeed{}, false
	}
	return c.Feeds[i], true
}

// WebhookByName returns the webhook with the given name and reports whether it was found.
func (c *Config) WebhookByName(name string) (ConfigWebhook, bool) {
	i := slices.IndexFunc(c.Webhooks, func(w ConfigWebhook) bool { return w.Name == name })
	if i == -1 {
		return ConfigWebhook{}, false
	}
	return c.Webhooks[i], true
}

// ConfigApp is the app-wide section of the configuration.
type ConfigApp struct {
	BrandingDisabled bool   `toml:"branding_disabled"`
	LogLevel         string `toml:"loglevel"`
	Oldest           int    `toml:"oldest"`
	Ticker           int    `toml:"ticker"`
	Timeout          int    `toml:"timeout"`
}

// LoggerLevel returns the configured log level or the default if it is invalid.
func (ca ConfigApp) LoggerLevel() slog.Level {
	m := map[string]slog.Level{"DEBUG": slog.LevelDebug, "INFO": slog.LevelInfo, "WARN": slog.LevelWarn, "ERROR": slog.LevelError}
	v, ok := m[strings.ToUpper(ca.LogLevel)]
	if !ok {
		return logLevelDefault
	}
	return v
}

// ConfigFeed is the configuration of a feed.
type ConfigFeed struct {
	Name     string `toml:"name"`
	URL      string `toml:"url"`
	Webhook  string `toml:"webhook"`
	Disabled bool   `toml:"disabled"`
}

// ConfigWebhook is the configuration of a webhook.
type ConfigWebhook struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

// FromFile reads, validates and returns the configuration from a TOML file.
// Missing app settings are set to their defaults.
func FromFile(path string) (Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return config, err
	}
	if err := parseConfig(&config); err != nil {
		return config, err
	}
	return config, nil
}

func parseConfig(config *Config) error {
	var errs []error
	webhookNames := make(map[string]bool)
	webhookURLs := make(map[string]bool)
	for i, x := range config.Webhooks {
		var name string
		if x.Name == "" {
			name = fmt.Sprintf("#%d", i+1)
		} else {
			name = strconv.Quote(x.Name)
		}
		if x.Name == "" {
			errs = append(errs, fmt.Errorf("webhook %s has no name", name))
		} else if webhookNames[x.Name] {
			errs = append(errs, fmt.Errorf("webhook name %s not unique", name))
		}
		webhookNames[x.Name] = true

		if x.URL == "" {
			errs = append(errs, fmt.Errorf("webhook %s has no url", name))
		} else if _, err := url.ParseRequestURI(x.URL); err != nil {
			errs = append(errs, fmt.Errorf("webhook %s has invalid url: %w", name, err))
		} else if webhookURLs[x.URL] {
			errs = append(errs, fmt.Errorf("webhook %s: url not unique", name))
		}
		webhookURLs[x.URL] = true
	}
	if len(config.Feeds) == 0 {
		errs = append(errs, fmt.Errorf("no feeds defined"))
	}

	feedNames := make(map[string]bool)
	webhooksUsed := make(map[string]bool)
	for i, x := range config.Feeds {
		var name string
		if x.Name == "" {
			name = fmt.Sprintf("#%d", i+1)
		} else {
			name = strconv.Quote(x.Name)
		}
		if x.Name == "" {
			errs = append(errs, fmt.Errorf("feed %s has no name", name))
		} else if feedNames[x.Name] {
			errs = append(errs, fmt.Errorf("feed name %s not unique", name))
		}
		feedNames[x.Name] = true

		if x.Webhook == "" {
			errs = append(errs, fmt.Errorf("feed %s has no webhook", name))
		} else if !webhookNames[x.Webhook] {
			errs = append(errs, fmt.Errorf("feed %s: invalid webhook: %q", name, x.Webhook))
		}
		webhooksUsed[x.Webhook] = true

		if x.URL == "" {
			errs = append(errs, fmt.Errorf("feed %s has no url", name))
		} else if _, err := url.ParseRequestURI(x.URL); err != nil {
			errs = append(errs, fmt.Errorf("feed %s has invalid url: %w", name, err))
		}
	}

	for _, wh := range config.Webhooks {
		if !webhooksUsed[wh.Name] {
			slog.Warn("Webhook defined, but not used", "name", wh.Name)
		}
	}
	if config.App.Timeout <= 0 {
		config.App.Timeout = timeoutDefault
	}
	if config.App.Oldest < OldestUnlimited {
		errs = append(
			errs,
			fmt.Errorf("oldest must be %d or greater, got %d", OldestUnlimited, config.App.Oldest))
	}
	if config.App.Oldest == 0 {
		config.App.Oldest = oldestDefault
	}
	if config.App.Ticker <= 0 {
		config.App.Ticker = tickerDefault
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid config:\n%w", errors.Join(errs...))
	}
	return nil
}
