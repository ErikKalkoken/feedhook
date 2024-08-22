package main

type configMain struct {
	Feeds    []configFeed
	Webhooks []configWebhook
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
