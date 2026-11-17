// Package dispatcher contains the dispatcher service.
package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/ErikKalkoken/go-dhook"
	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/messenger"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/ErikKalkoken/feedhook/internal/syncedmap"
)

var ErrNotFound = errors.New("not found")
var errUserAborted = errors.New("aborted by user")

type Clock interface {
	Now() time.Time
}

// Dispatcher is a service that fetches items from feeds and forwards them to webhooks.
//
// A dispatcher can be started, stopped and restarted.
type Dispatcher struct {
	cfg        config.Config
	client     *dhook.Client
	clock      Clock
	stopped    chan struct{}  // shutdown is complete
	fp         *gofeed.Parser // use ParseURLWithContext: ParseURL has a fixed 30s timeout
	messengers *syncedmap.SyncedMap[string, *messenger.Messenger]
	st         *storage.Storage

	mu        sync.Mutex
	isRunning bool
	cancel    context.CancelFunc // commence shutdown
}

// New creates a new App instance and returns it.
func New(st *storage.Storage, cfg config.Config, clock Clock) *Dispatcher {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.App.Timeout) * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = httpClient
	opts := []dhook.ClientOption{dhook.WithHTTPClient(httpClient)}
	if cfg.App.Timeout > 0 {
		// dhook applies its own 30s default otherwise.
		opts = append(opts, dhook.WithHTTPTimeout(time.Duration(cfg.App.Timeout)*time.Second))
	}
	client := dhook.NewClient(opts...)
	d := &Dispatcher{
		client:     client,
		cfg:        cfg,
		clock:      clock,
		stopped:    make(chan struct{}),
		fp:         fp,
		messengers: syncedmap.New[string, *messenger.Messenger](),
		st:         st,
	}
	return d
}

// Stop stops the dispatcher, which includes the gracefully shutdown of all messengers.
// Trying to stop an already stopped dispatcher does nothing.
// Reports whether a stop was actually performed.
func (d *Dispatcher) Stop() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.isRunning {
		return false
	}
	d.cancel()
	<-d.stopped
	slog.Info("Dispatcher stopped")
	var wg sync.WaitGroup
	for _, mg := range d.messengers.All() {
		wg.Go(func() {
			mg.Shutdown()
		})
	}
	wg.Wait()
	slog.Info("shutdown completed")
	d.isRunning = false
	return true
}

func (d *Dispatcher) Restart() error {
	d.Stop()
	return d.Start()
}

// Start starts the dispatcher
// User should call Close() subsequently to shut down dispatcher gracefully
// and prevent any potential data loss.
func (d *Dispatcher) Start() error {
	// Create all messengers first, so that nothing is started when one fails.
	messengers := make([]*messenger.Messenger, 0, len(d.cfg.Webhooks))
	for _, h := range d.cfg.Webhooks {
		q, err := pqueue.New(d.st.DB(), h.Name)
		if err != nil {
			return err
		}
		messengers = append(messengers, messenger.NewMessenger(d.client, q, h.Name, h.URL, d.st, d.cfg))
	}
	var ctx context.Context
	err := func() error {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.isRunning {
			return fmt.Errorf("dispatcher already running")
		}
		d.isRunning = true
		ctx, d.cancel = context.WithCancel(context.Background())
		return nil
	}()
	if err != nil {
		return err
	}
	for _, ms := range messengers {
		d.messengers.Store(ms.Name(), ms)
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
				if len(cf.Webhooks) == 0 {
					slog.Warn("Skipping feed without webhooks", "name", cf.Name)
					continue
				}
				wg.Go(func() {
					usedHooks := make([]*messenger.Messenger, 0)
					for _, name := range cf.Webhooks {
						wh, ok := d.messengers.Load(name)
						if !ok {
							panic("expected webhook not found: " + name)
						}
						usedHooks = append(usedHooks, wh)
					}
					err := d.processFeed(ctx, cf, usedHooks)
					if err == errUserAborted {
						slog.Debug("user aborted")
						return
					}
					if err != nil {
						slog.Error("Failed to process feed", "feed", cf.Name, "error", err)
					}
				})
			}
			wg.Wait()
			slog.Info("Finished processing feeds", "feeds", len(feeds))
			select {
			case <-ctx.Done():
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
func (d *Dispatcher) processFeed(ctx context.Context, cf config.ConfigFeed, hooks []*messenger.Messenger) error {
	myLog := slog.With("feed", cf.Name)
	feed, err := d.fp.ParseURLWithContext(cf.URL, ctx)
	if err != nil {
		if ctx.Err() != nil {
			return errUserAborted
		}
		return fmt.Errorf("parse URL for feed %s: %w ", cf.Name, err)
	}
	oldest := time.Duration(d.cfg.App.Oldest) * time.Second
	sort.Sort(feed)
	for _, item := range feed.Items {
		if item.Content == "" && item.Description == "" {
			continue
		}
		if ctx.Err() != nil {
			return errUserAborted
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

// SendPing sends a ping message to a configured webhook.
func (d *Dispatcher) SendPing(webhookName string) error {
	wh, ok := d.cfg.WebhookByName(webhookName)
	if !ok {
		return fmt.Errorf("webhook \"%s\": %w", webhookName, ErrNotFound)
	}
	_, err := d.client.NewWebhook(wh.URL).Execute(dhook.Message{Content: "Ping from feedhook"}, nil)
	return err
}

func (d *Dispatcher) PostLatestFeedItem(feedName string) error {
	cf, ok := d.cfg.FeedByName(feedName)
	if !ok {
		return fmt.Errorf("feed \"%s\": %w", feedName, ErrNotFound)
	}
	hooks := make([]config.ConfigWebhook, 0)
	for _, name := range cf.Webhooks {
		if h, ok := d.cfg.WebhookByName(name); ok {
			hooks = append(hooks, h)
		}
	}
	if len(hooks) == 0 {
		return fmt.Errorf("no webhooks configured for feed: %s ", feedName)
	}
	feed, err := d.fp.ParseURLWithContext(cf.URL, context.Background())
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
	for _, hook := range hooks {
		wh := d.client.NewWebhook(hook.URL)
		if _, err := wh.Execute(m, nil); err != nil {
			return fmt.Errorf("post item to webhook: %w", err)
		}
	}
	return nil
}
