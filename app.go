package main

import (
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

type message struct {
	payload *webhookPayload
	errC    chan error
}

type app struct {
	db       *bolt.DB
	config   configMain
	messageC map[string]chan message
	fp       *gofeed.Parser
}

func NewApp(db *bolt.DB, config configMain) *app {
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
	app := &app{
		db:       db,
		config:   config,
		messageC: make(map[string]chan message),
		fp:       gofeed.NewParser(),
	}
	for name, url := range webhooks {
		c := make(chan message)
		app.messageC[name] = c
		go func(url string, message <-chan message) {
			for m := range message {
				err := sendToWebhook(m.payload, url)
				m.errC <- err
			}
		}(url, c)
	}
	return app
}

func (a *app) run() {
	var wg sync.WaitGroup
	ticker := time.NewTicker(5 * time.Second)
	for {
		slog.Info("Started processing feeds", "count", len(a.config.Feeds))
		for _, cf := range a.config.Feeds {
			wg.Add(1)
			go func() {
				defer wg.Done()
				a.processFeed(cf)
			}()
		}
		wg.Wait()
		slog.Info("Completed processing feeds", "count", len(a.config.Feeds))
		<-ticker.C
	}
}

func (a *app) processFeed(cf configFeed) {
	feed, err := a.fp.ParseURL(cf.URL)
	if err != nil {
		slog.Error("failed to parse feed URL", "feed", cf.Name, "url", cf.URL)
		return
	}
	lastPublished := a.fetchLastPublished(cf.URL)
	var newest time.Time
	for _, item := range feed.Items {
		if !item.PublishedParsed.After(lastPublished) {
			continue
		}
		if item.PublishedParsed.Before(time.Now().Add(-oldest)) {
			continue
		}
		payload, err := makePayload(feed.Title, item)
		if err != nil {
			slog.Error("Failed to make payload", "feed", cf.Name, "error", "err")
			continue
		}
		m := message{payload: &payload, errC: make(chan error)}
		a.messageC[cf.Webhook] <- m
		if err := <-m.errC; err != nil {
			slog.Error("Failed to send payload to webhook", "feed", cf.Name, "error", err)
			continue
		}
		if !item.PublishedParsed.After(newest) {
			continue
		}
		newest = *item.PublishedParsed
		if err := a.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketFeeds))
			err := b.Put([]byte(cf.URL), []byte(newest.Format(time.RFC3339)))
			return err
		}); err != nil {
			log.Fatal(err)
		}
	}
}

func (a *app) fetchLastPublished(url string) time.Time {
	var lp time.Time
	a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketFeeds))
		v := b.Get([]byte(url))
		if v != nil {
			t, err := time.Parse(time.RFC3339, string(v))
			if err != nil {
				slog.Error("failed to parse last published", "value", v, "error", err)
			} else {
				lp = t
			}
		}
		return nil
	})
	return lp
}
