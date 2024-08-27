package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
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

// Close conducts a graceful shutdown of the app.
func (a *App) Close() {
	close(a.quit)
	<-a.done
	slog.Info("Graceful shutdown completed")
}

// Init initialized the app, e.g. by creating buckets in the DB as needed.
func (a *App) Init() error {
	names := make(map[string]bool)
	for _, f := range a.cfg.Feeds {
		names[f.Name] = true
	}
	tx, err := a.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	bkt, err := tx.CreateBucketIfNotExists([]byte(bucketFeeds))
	if err != nil {
		return err
	}
	for n := range names {
		if _, err := bkt.CreateBucketIfNotExists([]byte(n)); err != nil {
			return err
		}
	}
	bkt.ForEach(func(k, v []byte) error {
		if name := string(k); !names[name] {
			if err := bkt.DeleteBucket(k); err != nil {
				return err
			}
			slog.Info("Deleted obsolete bucket for feed", "name", name)
		}
		return nil
	})
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Start starts the main loop of the application.
// User should call Close() subsequently to shut down the loop gracefully and free resources.
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
	feeds := a.cfg.EnabledFeeds()
	slog.Info("Started", "feeds", len(feeds), "webhooks", len(a.cfg.Webhooks))
	go func() {
	main:
		for {
			for _, cf := range feeds {
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
			slog.Info("Finished processing feeds", "feeds", len(feeds))
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
		slog.Info("Stopped")
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
	oldest := time.Duration(a.cfg.App.Oldest) * time.Second
	sort.Sort(feed)
	for _, item := range feed.Items {
		select {
		case <-a.quit:
			return errUserAborted
		default:
		}
		if oldest != 0 && item.PublishedParsed != nil && item.PublishedParsed.Before(a.clock.Now().Add(-oldest)) {
			continue
		}
		if !a.isItemNew(cf, item) {
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
		if err := a.recordItem(cf, item); err != nil {
			return fmt.Errorf("failed to record item: %w", err)
		}
	}
	err = a.cullFeed(cf, 1000)
	return err
}

func (a *App) recordItem(cf ConfigFeed, item *gofeed.Item) error {
	err := a.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		var t time.Time
		if item.PublishedParsed != nil {
			t = *item.PublishedParsed
		} else {
			t = time.Now().UTC()
		}
		return b.Put([]byte(itemUniqueID(item)), []byte(t.Format(time.RFC3339)))
	})
	return err
}

// isItemNew reports wether an item in a feed is new
func (a *App) isItemNew(cf ConfigFeed, item *gofeed.Item) bool {
	var isNew bool
	a.db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		v := b.Get([]byte(itemUniqueID(item)))
		if v == nil {
			isNew = true
		}
		return nil
	})
	return isNew
}

type item struct {
	time time.Time
	key  []byte
}

// cullFeed deletes the oldest items when there are more items then a limit
func (a *App) cullFeed(cf ConfigFeed, limit int) error {
	err := a.db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket([]byte(bucketFeeds))
		b := root.Bucket([]byte(cf.Name))
		items := make([]item, 0)
		b.ForEach(func(k, v []byte) error {
			t, err := time.Parse(time.RFC3339, string(v))
			if err != nil {
				return err
			}
			items = append(items, item{time: t, key: k})
			return nil
		})
		slices.SortFunc(items, func(a item, b item) int {
			return a.time.Compare(b.time) * -1
		})
		if len(items) <= limit {
			return nil
		}
		for _, i := range items[limit:] {
			if err := b.Delete(i.key); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// itemUniqueID returns a unique ID of an item.
func itemUniqueID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	s := item.Title + item.Description + item.Content
	return makeHash(s)
}

func makeHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
