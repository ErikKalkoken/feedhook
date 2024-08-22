package app

import (
	"log"
	"log/slog"

	"github.com/BurntSushi/toml"
)

type configMain struct {
	Feeds      []configFeed
	Webhooks   []configWebhook
	WebhookMap map[string]string
}

type configFeed struct {
	Name    string `toml:"name"`
	URL     string `toml:"url"`
	Webhook string `toml:"webhook"`
}

type configWebhook struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

func ReadConfig(fn string) configMain {
	var config configMain
	if _, err := toml.DecodeFile(fn, &config); err != nil {
		log.Fatal(err)
	}
	webhooksUsed := make(map[string]bool)
	webhooks := make(map[string]string)
	for _, x := range config.Webhooks {
		webhooks[x.Name] = x.URL
	}
	for _, x := range config.Feeds {
		_, ok := webhooks[x.Webhook]
		if !ok {
			log.Fatalf("Config error: Invalid webhook name \"%s\" for feed \"%s\"", x.Webhook, x.Name)
		}
		webhooksUsed[x.Webhook] = true
	}
	for k, v := range webhooksUsed {
		if !v {
			slog.Warn("Webhook defined, but not used", "name", k)
		}
	}
	config.WebhookMap = webhooks
	return config
}
