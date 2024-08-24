package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	bolt "go.etcd.io/bbolt"
)

const (
	bucketFeeds = "feeds"
)

var errUserAborted = errors.New("aborted by user")

type Clock interface {
	Now() time.Time
}

// message represents a message send to a webhook.
// Consumers must listen on the errC channel to receive the result.
type message struct {
	payload *webhookPayload
	errC    chan error
}

// App represents this application and holds it's global data.
type App struct {
	client *http.Client
	db     *bolt.DB
	cfg    MyConfig
	fp     *gofeed.Parser
	clock  Clock
	done   chan bool // signals that the shutdown is complete
	quit   chan bool // closed to signal a shutdown
}

// New creates a new App instance and returns it.
func New(db *bolt.DB, cfg MyConfig, clock Clock) *App {
	client := &http.Client{
		Timeout: time.Duration(cfg.App.Timeout) * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = client
	app := &App{
		client: client,
		db:     db,
		cfg:    cfg,
		clock:  clock,
		fp:     fp,
		done:   make(chan bool),
		quit:   make(chan bool),
	}
	return app
}

// Start starts the main loop of the application.
// User should call Close() subsequently to shut down the loop gracefully.
func (a *App) Start() {
	// start goroutines for webhooks
	messageC := make(map[string]chan message)
	for _, h := range a.cfg.Webhooks {
		c := make(chan message)
		messageC[h.Name] = c
		go func(url string, message <-chan message) {
			for m := range message {
				err := sendToWebhook(a.client, m.payload, url)
				m.errC <- err
			}
		}(h.URL, c)
	}
	// process feeds until aborted
	var wg sync.WaitGroup
	ticker := time.NewTicker(time.Duration(a.cfg.App.Ticker) * time.Second)
	slog.Info("Started processing feeds", "count", len(a.cfg.Feeds))
	go func() {
	main:
		for {
			for _, cf := range a.cfg.Feeds {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := a.processFeed(cf, messageC[cf.Webhook]); err == errUserAborted {
						slog.Debug("user aborted")
						return
					} else if err != nil {
						slog.Error("Failed to process feed", "name", cf.Name, "error", err)
					}
				}()
			}
			wg.Wait()
		wait:
			for {
				select {
				case <-a.quit:
					break main
				case <-ticker.C:
					break wait
				}
			}
		}
		slog.Info("Stopped processing feeds")
		ticker.Stop()
		a.done <- true
	}()
}

// processFeed processes a configured feed.
func (a *App) processFeed(cf ConfigFeed, messageC chan<- message) error {
	feed, err := a.fp.ParseURL(cf.URL)
	if err != nil {
		return fmt.Errorf("failed to parse URL for feed %s: %w ", cf.Name, err)
	}
	lastPublished := a.fetchLastPublished(cf)
	oldest := time.Duration(a.cfg.App.Oldest) * time.Second
	var newest time.Time
	sort.Sort(feed)
	for _, item := range feed.Items {
		select {
		case <-a.quit:
			return errUserAborted
		default:
		}
		if !item.PublishedParsed.After(lastPublished) {
			continue
		}
		if item.PublishedParsed.Before(a.clock.Now().Add(-oldest)) {
			continue
		}
		payload, err := makePayload(feed, item)
		if err != nil {
			slog.Error("Failed to make payload", "feed", cf.Name, "error", "err")
			continue
		}
		m := message{payload: &payload, errC: make(chan error)}
		messageC <- m
		if err := <-m.errC; err != nil {
			return fmt.Errorf("failed to send payload to webhook: %w", err)
		}
		slog.Info("Posted item", "feed", cf.Name, "webhook", cf.Webhook, "title", item.Title)
		if !item.PublishedParsed.After(newest) {
			continue
		}
		newest = *item.PublishedParsed
		if err := a.db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(bucketFeeds))
			err := b.Put([]byte(cf.Name), []byte(newest.Format(time.RFC3339)))
			return err
		}); err != nil {
			return fmt.Errorf("failed to update database: %w", err)
		}
	}
	return nil
}

// fetchLastPublished returns the time of the last published item (if any).
func (a *App) fetchLastPublished(cf ConfigFeed) time.Time {
	var lp time.Time
	a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketFeeds))
		v := b.Get([]byte(cf.Name))
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

// Close conducts a graceful shutdown of the app.
func (a *App) Close() {
	close(a.quit)
	<-a.done
	slog.Info("application shutdown completed")
}

// SetupDB initialized the database, e.g. by creating all buckets if needed.
func SetupDB(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketFeeds))
		return err
	})
	return err
}
