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

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/app/webhook"
	"github.com/ErikKalkoken/feedhook/internal/discordhook"
	"github.com/ErikKalkoken/feedhook/internal/queue"
	"github.com/ErikKalkoken/feedhook/internal/syncx"
)

var errUserAborted = errors.New("aborted by user")

type Clock interface {
	Now() time.Time
}

// Service represents this core application logic and holds it's global data.
type Service struct {
	cfg    app.MyConfig
	client *discordhook.Client
	clock  Clock
	done   chan bool // signals that the shutdown is complete
	fp     *gofeed.Parser
	hooks  *syncx.Map[string, *webhook.Webhook]
	quit   chan bool // closed to signal a shutdown
	st     *storage.Storage
}

// New creates a new App instance and returns it.
func New(st *storage.Storage, cfg app.MyConfig, clock Clock) *Service {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.App.Timeout) * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = httpClient
	s := &Service{
		client: discordhook.NewClient(httpClient),
		cfg:    cfg,
		clock:  clock,
		done:   make(chan bool),
		fp:     fp,
		hooks:  syncx.NewMap[string, *webhook.Webhook](),
		quit:   make(chan bool),
		st:     st,
	}
	return s
}

// Close conducts a graceful shutdown of the app.
func (s *Service) Close() {
	close(s.quit)
	<-s.done
	slog.Info("Graceful shutdown completed")
}

// Start starts the main loop of the application.
// User should call Close() subsequently to shut down the loop gracefully and free resources.
func (s *Service) Start() {
	// Create and start webhooks
	for _, h := range s.cfg.Webhooks {
		q, err := queue.New(s.st.DB(), h.Name)
		if err != nil {
			panic(err)
		}
		wh := webhook.New(s.client, q, h.Name, h.URL, s.st, s.cfg)
		wh.Start()
		s.hooks.Store(h.Name, wh)
	}
	// process feeds until aborted
	var wg sync.WaitGroup
	ticker := time.NewTicker(time.Duration(s.cfg.App.Ticker) * time.Second)
	feeds := s.cfg.EnabledFeeds()
	slog.Info("Started", "feeds", len(feeds), "webhooks", len(s.cfg.Webhooks))
	go func() {
	main:
		for {
			for _, cf := range feeds {
				wg.Add(1)
				go func() {
					defer wg.Done()
					usedHooks := make([]*webhook.Webhook, 0)
					for _, name := range cf.Webhooks {
						wh, ok := s.hooks.Load(name)
						if !ok {
							panic("expected webhook not found: " + name)
						}
						usedHooks = append(usedHooks, wh)
					}
					if err := s.processFeed(cf, usedHooks); err == errUserAborted {
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
				case <-s.quit:
					break main
				case <-ticker.C:
					break wait
				}
			}
		}
		slog.Info("Stopped")
		ticker.Stop()
		s.done <- true
	}()
}

// processFeed processes a configured feed.
func (s *Service) processFeed(cf app.ConfigFeed, hooks []*webhook.Webhook) error {
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
			if err := hook.EnqueueMessage(cf.Name, feed, item, state == app.StateUpdated); err != nil {
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

func (s *Service) WebhookQueueSize(name string) (int, error) {
	wh, ok := s.hooks.Load(name)
	if !ok {
		return 0, fmt.Errorf("webhook not found")
	}
	return wh.QueueSize(), nil
}
