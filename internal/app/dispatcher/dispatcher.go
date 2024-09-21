// Package dispatcher contains the dispatcher service.
package dispatcher

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/messenger"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/discordhook"
	"github.com/ErikKalkoken/feedhook/internal/queue"
	"github.com/ErikKalkoken/feedhook/internal/syncx"
)

var errUserAborted = errors.New("aborted by user")

type Clock interface {
	Now() time.Time
}

// Dispatcher is a service that fetches items from feeds and forwards them to webhooks.
type Dispatcher struct {
	cfg    app.MyConfig
	client *discordhook.Client
	clock  Clock
	done   chan bool // signals that the shutdown is complete
	fp     *gofeed.Parser
	hooks  *syncx.Map[string, *messenger.Messenger]
	quit   chan bool // closed to signal a shutdown
	st     *storage.Storage
}

// New creates a new App instance and returns it.
func New(st *storage.Storage, cfg app.MyConfig, clock Clock) *Dispatcher {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.App.Timeout) * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = httpClient
	d := &Dispatcher{
		client: discordhook.NewClient(httpClient),
		cfg:    cfg,
		clock:  clock,
		done:   make(chan bool),
		fp:     fp,
		hooks:  syncx.NewMap[string, *messenger.Messenger](),
		quit:   make(chan bool),
		st:     st,
	}
	return d
}

// Close conducts a graceful shutdown of the dispatcher.
func (d *Dispatcher) Close() {
	close(d.quit)
	<-d.done
	slog.Info("Graceful shutdown completed")
}

// Start starts the dispatcher
// User should call Close() subsequently to shut down dispatcher gracefully
// and prevent any potential data loss.
func (d *Dispatcher) Start() {
	// Create and start webhooks
	for _, h := range d.cfg.Webhooks {
		q, err := queue.New(d.st.DB(), h.Name)
		if err != nil {
			panic(err)
		}
		wh := messenger.New(d.client, q, h.Name, h.URL, d.st, d.cfg)
		wh.Start()
		d.hooks.Store(h.Name, wh)
	}
	// process feeds until aborted
	var wg sync.WaitGroup
	ticker := time.NewTicker(time.Duration(d.cfg.App.Ticker) * time.Second)
	feeds := d.cfg.EnabledFeeds()
	slog.Info("Started", "feeds", len(feeds), "webhooks", len(d.cfg.Webhooks))
	go func() {
	main:
		for {
			for _, cf := range feeds {
				wg.Add(1)
				go func() {
					defer wg.Done()
					usedHooks := make([]*messenger.Messenger, 0)
					for _, name := range cf.Webhooks {
						wh, ok := d.hooks.Load(name)
						if !ok {
							panic("expected webhook not found: " + name)
						}
						usedHooks = append(usedHooks, wh)
					}
					if err := d.processFeed(cf, usedHooks); err == errUserAborted {
						slog.Debug("user aborted")
						return
					} else if err != nil {
						slog.Error("Failed to process feed", "feed", cf.Name, "error", err)
					}
				}()
			}
			wg.Wait()
			slog.Info("Finished processing feeds", "feeds", len(feeds))
		wait:
			for {
				select {
				case <-d.quit:
					break main
				case <-ticker.C:
					break wait
				}
			}
		}
		slog.Info("Stopped")
		ticker.Stop()
		d.done <- true
	}()
}

// processFeed processes a configured feed.
func (s *Dispatcher) processFeed(cf app.ConfigFeed, hooks []*messenger.Messenger) error {
	myLog := slog.With("feed", cf.Name)
	feed, err := s.fp.ParseURL(cf.URL)
	if err != nil {
		return fmt.Errorf("failed to parse URL for feed %s: %w ", cf.Name, err)
	}
	oldest := time.Duration(s.cfg.App.Oldest) * time.Second
	sort.Sort(feed)
	for _, item := range feed.Items {
		select {
		case <-s.quit:
			return errUserAborted
		default:
		}
		if oldest != 0 && item.PublishedParsed != nil && item.PublishedParsed.Before(s.clock.Now().Add(-oldest)) {
			continue
		}
		state, err := s.st.GetItemState(cf, item)
		if err != nil {
			slog.Warn("Failed to read item state from DB. Assuming item is new.", "title", item.Title)
			state = app.StateNew
		} else if state == app.StateProcessed {
			continue
		}
		for _, hook := range hooks {
			if err := hook.AddMessage(cf.Name, feed, item, state == app.StateUpdated); err != nil {
				myLog.Error("Failed to add item to webhook queue", "hook", hook.Name(), "error", "err")
				if err := s.st.UpdateFeedStats(cf.Name, func(fs *app.FeedStats) error {
					fs.ErrorCount++
					return nil
				}); err != nil {
					myLog.Error("failed to update feed stats", "error", err)
				}
				continue
			}
		}
		if err := s.st.RecordItem(cf, item); err != nil {
			return fmt.Errorf("failed to record item: %w", err)
		}
		if err := s.st.UpdateFeedStats(cf.Name, func(fs *app.FeedStats) error {
			fs.ReceivedCount++
			fs.ReceivedLast = time.Now().UTC()
			return nil
		}); err != nil {
			myLog.Error("failed to update feed stats", "error", err)
		}
		myLog.Info("Received item", "title", item.Title)
	}
	err = s.st.CullItems(cf, 1000)
	return err
}

// WebhookQueueSize returns the current size of a webhook queue.
func (d *Dispatcher) WebhookQueueSize(name string) (int, error) {
	wh, ok := d.hooks.Load(name)
	if !ok {
		return 0, fmt.Errorf("webhook not found")
	}
	return wh.QueueSize(), nil
}
