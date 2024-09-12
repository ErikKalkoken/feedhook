package service

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedforward/internal/app"
	"github.com/ErikKalkoken/feedforward/internal/app/storage"
	"github.com/ErikKalkoken/feedforward/internal/app/webhook"
	"github.com/ErikKalkoken/feedforward/internal/queue"
)

var errUserAborted = errors.New("aborted by user")

type Clock interface {
	Now() time.Time
}

// App represents this application and holds it's global data.
type App struct {
	client *http.Client
	st     *storage.Storage
	cfg    app.MyConfig
	fp     *gofeed.Parser
	clock  Clock
	done   chan bool // signals that the shutdown is complete
	quit   chan bool // closed to signal a shutdown
}

// New creates a new App instance and returns it.
func New(st *storage.Storage, cfg app.MyConfig, clock Clock) *App {
	client := &http.Client{
		Timeout: time.Duration(cfg.App.Timeout) * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = client
	app := &App{
		client: client,
		st:     st,
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

// Start starts the main loop of the application.
// User should call Close() subsequently to shut down the loop gracefully and free resources.
func (a *App) Start() {
	// Create and start webhooks
	hooks := make(map[string]*webhook.Webhook)
	for _, h := range a.cfg.Webhooks {
		q, err := queue.New(a.st.DB(), h.Name)
		if err != nil {
			panic(err)
		}
		hooks[h.Name] = webhook.New(a.client, q, h.Name, h.URL)
		hooks[h.Name].Start()
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
					if err := a.processFeed(cf, hooks[cf.Webhook]); err == errUserAborted {
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
func (a *App) processFeed(cf app.ConfigFeed, hook *webhook.Webhook) error {
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
		if !a.st.IsItemNew(cf, item) {
			continue
		}
		if err := hook.Send(feed, item); err != nil {
			slog.Error("Failed to add item to send queue", "feed", cf.Name, "error", "err")
			continue
		}
		if err := a.st.RecordItem(cf, item); err != nil {
			return fmt.Errorf("failed to record item: %w", err)
		}
		if err := a.st.UpdateFeedStats(cf.Name); err != nil {
			slog.Error("failed to update feed stats", "name", cf.Name, "error", err)
		}
		if err := a.st.UpdateWebhookStats(cf.Webhook); err != nil {
			slog.Error("failed to update webhook stats", "name", cf.Webhook, "error", err)
		}
		slog.Info("Received item", "feed", cf.Name, "webhook", cf.Webhook, "title", item.Title)
	}
	err = a.st.CullItems(cf, 1000)
	return err
}
