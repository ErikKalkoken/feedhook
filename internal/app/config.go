package app

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	timeoutDefault  = 30
	oldestDefault   = 48 * 3600
	tickerDefault   = 30
	logLevelDefault = slog.LevelInfo
)

type MyConfig struct {
	App      ConfigApp
	Feeds    []ConfigFeed
	Webhooks []ConfigWebhook
}

type ConfigApp struct {
	LogLevel string `toml:"loglevel"`
	Oldest   int    `toml:"oldest"`
	Ticker   int    `toml:"ticker"`
	Timeout  int    `toml:"timeout"`
}

func (ca ConfigApp) LoggerLevel() slog.Level {
	m := map[string]slog.Level{"DEBUG": slog.LevelDebug, "INFO": slog.LevelInfo, "WARN": slog.LevelWarn, "ERROR": slog.LevelError}
	v, ok := m[strings.ToUpper(ca.LogLevel)]
	if !ok {
		return logLevelDefault
	}
	return v
}

type ConfigFeed struct {
	Name    string `toml:"name"`
	URL     string `toml:"url"`
	Webhook string `toml:"webhook"`
}

type ConfigWebhook struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

func ReadConfig(path string) (MyConfig, error) {
	var config MyConfig
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return config, err
	}
	if err := parseConfig(&config); err != nil {
		return config, err
	}
	return config, nil
}

func parseConfig(config *MyConfig) error {
	webhookNames := make(map[string]bool)
	webhookURLs := make(map[string]bool)
	for _, x := range config.Webhooks {
		if x.Name == "" {
			return fmt.Errorf("one webhook has no name")
		}
		if x.URL == "" {
			return fmt.Errorf("webhook %s has no url", x.Name)
		}
		if _, err := url.ParseRequestURI(x.URL); err != nil {
			return fmt.Errorf("webhook %s has invalid url: %w", x.Name, err)
		}
		if webhookNames[x.Name] {
			return fmt.Errorf("webhook name %s no unique", x.Name)
		}
		webhookNames[x.Name] = true
		if webhookURLs[x.URL] {
			return fmt.Errorf("webhook name %s no unique", x.Name)
		}
		webhookURLs[x.URL] = true
	}
	if len(config.Feeds) == 0 {
		return fmt.Errorf("no feeds defined")
	}
	feedNames := make(map[string]bool)
	webhooksUsed := make(map[string]bool)
	for _, x := range config.Feeds {
		if x.Name == "" {
			return fmt.Errorf("one feed has no name")
		}
		if x.URL == "" {
			return fmt.Errorf("feed %s has no url", x.Name)
		}
		if feedNames[x.Name] {
			return fmt.Errorf("feed name %s not unique", x.Name)
		}
		feedNames[x.Name] = true
		if _, err := url.ParseRequestURI(x.URL); err != nil {
			return fmt.Errorf("feed %s has invalid url: %w", x.Name, err)
		}
		if !webhookNames[x.Webhook] {
			return fmt.Errorf("invalid webhook name \"%s\" for feed \"%s\"", x.Webhook, x.Name)
		}
		webhooksUsed[x.Webhook] = true
	}
	for k, v := range webhooksUsed {
		if !v {
			slog.Warn("Webhook defined, but not used", "name", k)
		}
	}
	if config.App.Timeout <= 0 {
		config.App.Timeout = timeoutDefault
	}
	if config.App.Oldest <= 0 {
		config.App.Oldest = oldestDefault
	}
	if config.App.Ticker <= 0 {
		config.App.Ticker = tickerDefault
	}
	return nil
}
