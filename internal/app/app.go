package app

import (
	"context"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

const (
	BucketFeeds = "feeds"
	oldest      = 24 * time.Hour
)

type message struct {
	payload *webhookPayload
	errC    chan error
}

type App struct {
	db       *bolt.DB
	config   configMain
	messageC map[string]chan message
	fp       *gofeed.Parser
}

func New(db *bolt.DB, config configMain) *App {
	app := &App{
		db:       db,
		config:   config,
		messageC: make(map[string]chan message),
		fp:       gofeed.NewParser(),
	}
	for name, url := range config.WebhookMap {
		c := make(chan message)
		app.messageC[name] = c
		go func(url string, message <-chan message) {
			for m := range message {
				timeout := time.Second * time.Duration(config.App.DiscordTimeout)
				err := sendToWebhook(m.payload, url, timeout)
				m.errC <- err
			}
		}(url, c)
	}
	return app
}

func (a *App) Run() {
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

func (a *App) processFeed(cf configFeed) {
	timeout := time.Second * time.Duration(a.config.App.DiscordTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	feed, err := a.fp.ParseURLWithContext(cf.URL, ctx)
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
			b := tx.Bucket([]byte(BucketFeeds))
			err := b.Put([]byte(cf.URL), []byte(newest.Format(time.RFC3339)))
			return err
		}); err != nil {
			log.Fatal(err)
		}
	}
}

func (a *App) fetchLastPublished(url string) time.Time {
	var lp time.Time
	a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketFeeds))
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

func SetupDB(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BucketFeeds))
		return err
	})
	return err
}
