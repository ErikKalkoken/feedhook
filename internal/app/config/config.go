// Package config provides the app's configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
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

var logLevels = map[string]slog.Level{"DEBUG": slog.LevelDebug, "INFO": slog.LevelInfo, "WARN": slog.LevelWarn, "ERROR": slog.LevelError}

// LoggerLevel returns the configured log level or the default if it is invalid.
func (ca ConfigApp) LoggerLevel() slog.Level {
	v, ok := logLevels[strings.ToUpper(ca.LogLevel)]
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
// Failures make the config invalid and are reported with a general error. Warnings never do.
// Missing app settings are set to their defaults.
func FromFile(path string) (cfg Config, warnings []string, failures []string, err error) {
	s, err := os.Stat(path)
	if err != nil {
		return Config{}, nil, nil, err
	}
	if s.IsDir() {
		return Config{}, nil, nil, fmt.Errorf("%s is a directory, expected path to config file", path)
	}
	var config Config
	md, err := toml.DecodeFile(path, &config)
	if err != nil {
		return Config{}, nil, nil, err
	}
	for _, k := range md.Undecoded() {
		warnings = append(warnings, fmt.Sprintf("unknown key: %s", k))
	}
	w, failures := parseConfig(&config)
	warnings = append(warnings, w...)
	if len(failures) > 0 {
		return Config{}, warnings, failures, errors.New("invalid config")
	}
	return config, warnings, nil, nil
}

// parseConfig validates config and sets defaults.
// It returns detected configuration issues as warnings and failures.
func parseConfig(config *Config) (warnings []string, failures []string) {
	makeName := func(i int, name string) string {
		if name == "" {
			return fmt.Sprintf("#%d", i+1)
		}
		return strconv.Quote(name)
	}
	webhookNames := make(map[string]bool)
	webhookURLs := make(map[string]bool)
	for i, h := range config.Webhooks {
		name := makeName(i, h.Name)
		if h.Name == "" {
			failures = append(failures, fmt.Sprintf("webhook %s has no name", name))
		} else if webhookNames[h.Name] {
			failures = append(failures, fmt.Sprintf("webhook name %s not unique", name))
		}
		webhookNames[h.Name] = true

		if h.URL == "" {
			failures = append(failures, fmt.Sprintf("webhook %s has no url", name))
		} else if _, err := url.ParseRequestURI(h.URL); err != nil {
			failures = append(failures, fmt.Sprintf("webhook %s has invalid url: %s", name, err))
		} else if webhookURLs[h.URL] {
			failures = append(failures, fmt.Sprintf("webhook %s: url not unique", name))
		}
		webhookURLs[h.URL] = true
	}
	if len(config.Feeds) == 0 {
		failures = append(failures, "no feeds defined")
	}

	feedNames := make(map[string]bool)
	webhooksUsed := make(map[string]bool)
	for i, f := range config.Feeds {
		name := makeName(i, f.Name)
		if f.Name == "" {
			failures = append(failures, fmt.Sprintf("feed %s has no name", name))
		} else if feedNames[f.Name] {
			failures = append(failures, fmt.Sprintf("feed name %s not unique", name))
		}
		feedNames[f.Name] = true

		if f.Webhook == "" {
			failures = append(failures, fmt.Sprintf("feed %s has no webhook", name))
		} else if !webhookNames[f.Webhook] {
			failures = append(failures, fmt.Sprintf("feed %s: invalid webhook: %q", name, f.Webhook))
		}
		webhooksUsed[f.Webhook] = true

		if f.URL == "" {
			failures = append(failures, fmt.Sprintf("feed %s has no url", name))
		} else if _, err := url.ParseRequestURI(f.URL); err != nil {
			failures = append(failures, fmt.Sprintf("feed %s has invalid url: %s", name, err))
		}
	}
	if config.App.Timeout < 0 {
		warnings = append(warnings, fmt.Sprintf("timeout must be 0 or greater, got %d, using %d", config.App.Timeout, timeoutDefault))
	}
	if config.App.Timeout <= 0 {
		config.App.Timeout = timeoutDefault
	}
	if config.App.Oldest < OldestUnlimited {
		failures = append(failures, fmt.Sprintf("oldest must be %d or greater, got %d", OldestUnlimited, config.App.Oldest))
	}
	if config.App.Oldest == 0 {
		config.App.Oldest = oldestDefault
	}
	if config.App.Ticker < 0 {
		warnings = append(warnings, fmt.Sprintf("ticker must be 0 or greater, got %d, using %d", config.App.Ticker, tickerDefault))
	}
	if config.App.Ticker <= 0 {
		config.App.Ticker = tickerDefault
	}
	if _, ok := logLevels[strings.ToUpper(config.App.LogLevel)]; !ok && config.App.LogLevel != "" {
		warnings = append(warnings, fmt.Sprintf("unknown loglevel %q, using %s", config.App.LogLevel, logLevelDefault))
	}
	for i, h := range config.Webhooks {
		if h.Name != "" && !webhooksUsed[h.Name] {
			warnings = append(warnings, fmt.Sprintf("webhook %s defined, but not used", makeName(i, h.Name)))
		}
	}
	return warnings, failures
}
