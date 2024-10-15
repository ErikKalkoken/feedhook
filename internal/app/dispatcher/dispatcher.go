// Package dispatcher contains the dispatcher service.
package dispatcher

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/messenger"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/dhooks"
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/ErikKalkoken/feedhook/internal/syncedmap"
)

var ErrNotFound = errors.New("not found")
var errUserAborted = errors.New("aborted by user")

type Clock interface {
	Now() time.Time
}

// Dispatcher is a service that fetches items from feeds and forwards them to webhooks.
type Dispatcher struct {
	cfg        config.Config
	client     *dhooks.Client
	clock      Clock
	stopped    chan struct{} // signals that the shutdown is complete
	fp         *gofeed.Parser
	messengers *syncedmap.SyncedMap[string, *messenger.Messenger]
	quit       chan struct{} // closed to signal a shutdown
	st         *storage.Storage
}

// New creates a new App instance and returns it.
func New(st *storage.Storage, cfg config.Config, clock Clock) *Dispatcher {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.App.Timeout) * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = httpClient
	d := &Dispatcher{
		client:     dhooks.NewClient(httpClient),
		cfg:        cfg,
		clock:      clock,
		stopped:    make(chan struct{}),
		fp:         fp,
		messengers: syncedmap.New[string, *messenger.Messenger](),
		quit:       make(chan struct{}),
		st:         st,
	}
	return d
}

// Close conducts a graceful shutdown of the dispatcher.
func (d *Dispatcher) Close() {
	close(d.quit)
	<-d.stopped
	slog.Info("Dispatcher stopped")
	var wg sync.WaitGroup
	for _, mg := range d.messengers.All() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mg.Close()
		}()
	}
	wg.Wait()
	slog.Info("Graceful shutdown completed")
}

// Start starts the dispatcher
// User should call Close() subsequently to shut down dispatcher gracefully
// and prevent any potential data loss.
func (d *Dispatcher) Start() error {
	// Create and start webhooks
	for _, h := range d.cfg.Webhooks {
		q, err := pqueue.New(d.st.DB(), h.Name)
		if err != nil {
			return err
		}
		ms := messenger.NewMessenger(d.client, q, h.Name, h.URL, d.st, d.cfg)
		d.messengers.Store(h.Name, ms)
		ms.Start()
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
						wh, ok := d.messengers.Load(name)
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
			select {
			case <-d.quit:
				break main
			case <-ticker.C:
			}
		}
		slog.Debug("Dispatcher loop stopped")
		ticker.Stop()
		d.stopped <- struct{}{}
	}()
	return nil
}

// processFeed checks a feed for new items and hands them over to configured messengers.
func (d *Dispatcher) processFeed(cf config.ConfigFeed, hooks []*messenger.Messenger) error {
	myLog := slog.With("feed", cf.Name)
	feed, err := d.fp.ParseURL(cf.URL)
	if err != nil {
		return fmt.Errorf("parse URL for feed %s: %w ", cf.Name, err)
	}
	oldest := time.Duration(d.cfg.App.Oldest) * time.Second
	sort.Sort(feed)
	for _, item := range feed.Items {
		if item.Content == "" && item.Description == "" {
			continue
		}
		select {
		case <-d.quit:
			return errUserAborted
		default:
		}
		if oldest != 0 && item.PublishedParsed != nil && item.PublishedParsed.Before(d.clock.Now().Add(-oldest)) {
			continue
		}
		state, err := d.st.GetItemState(cf, item)
		if err != nil {
			slog.Warn("Failed to read item state from DB. Assuming item is new.", "title", item.Title)
			state = app.StateNew
		} else if state == app.StateProcessed {
			continue
		}
		for _, hook := range hooks {
			if err := hook.AddMessage(cf.Name, feed, item, state == app.StateUpdated); err != nil {
				myLog.Error("Failed to add item to webhook queue", "hook", hook.Name(), "error", err)
				if err := d.st.UpdateFeedStats(cf.Name, func(fs *app.FeedStats) error {
					fs.ErrorCount++
					return nil
				}); err != nil {
					myLog.Error("failed to update feed stats", "error", err)
				}
				continue
			}
		}
		if err := d.st.RecordItem(cf, item); err != nil {
			return fmt.Errorf("record item: %w", err)
		}
		if err := d.st.UpdateFeedStats(cf.Name, func(fs *app.FeedStats) error {
			fs.ReceivedCount++
			fs.ReceivedLast = time.Now().UTC()
			return nil
		}); err != nil {
			myLog.Error("failed to update feed stats", "error", err)
		}
		myLog.Info("Received item", "title", item.Title)
	}
	err = d.st.CullItems(cf, 1000)
	return err
}

// MessengerStatus returns the current status of a messenger.
func (d *Dispatcher) MessengerStatus(webhookName string) (messenger.Status, error) {
	wh, ok := d.messengers.Load(webhookName)
	if !ok {
		return messenger.Status{}, fmt.Errorf("webhook \"%s\": %w", webhookName, ErrNotFound)

	}
	return wh.Status(), nil
}

func (d *Dispatcher) PostLatestFeedItem(feedName string) error {
	var cf config.ConfigFeed
	for _, f := range d.cfg.Feeds {
		if f.Name == feedName {
			cf = f
			break
		}
	}
	if cf.Name == "" {
		return fmt.Errorf("feed \"%s\": %w", feedName, ErrNotFound)
	}
	hooks := make([]config.ConfigWebhook, 0)
	for _, name := range cf.Webhooks {
		for _, h := range d.cfg.Webhooks {
			if h.Name == name {
				hooks = append(hooks, h)
			}
		}
	}
	if len(hooks) == 0 {
		return fmt.Errorf("no webhooks configured for feed: %s ", feedName)
	}
	feed, err := d.fp.ParseURL(cf.URL)
	if err != nil {
		return fmt.Errorf("parse URL for feed: %w ", err)
	}
	items := make([]*gofeed.Item, 0)
	for _, i := range feed.Items {
		if i.PublishedParsed != nil {
			items = append(items, i)
		}
	}
	if len(items) == 0 {
		return fmt.Errorf("no items found in feed")
	}
	latest := slices.MaxFunc(items, func(a, b *gofeed.Item) int {
		return a.PublishedParsed.Compare(*b.PublishedParsed)
	})
	fi := messenger.NewFeedItem(feedName, feed, latest, false)
	m, err := fi.ToDiscordMessage(false)
	if err != nil {
		return fmt.Errorf("convert item to Discord message: %w", err)
	}
	if err := m.Validate(); err != nil {
		return fmt.Errorf("convert item to Discord message: %w", err)
	}
	c := dhooks.NewClient(http.DefaultClient)
	for _, hook := range hooks {
		wh := dhooks.NewWebhook(c, hook.URL)
		if err := wh.Execute(m); err != nil {
			return fmt.Errorf("post item to webhook: %w", err)
		}
	}
	return nil
}
