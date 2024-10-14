package messenger

import (
	"context"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/dhooks"
	"github.com/ErikKalkoken/feedhook/internal/queue"
)

// A Messenger handles posting messages to a webhook.
// Failed messages are automatically retried and rate limits are respected.
// Unsent messages are queued and will be picked up again after a process restart.
type Messenger struct {
	cfg      config.Config
	shutdown chan struct{} // commence shutdown
	done     chan struct{} // shutdown completed
	dwh      *dhooks.Webhook
	errCount atomic.Int64
	name     string
	queue    *queue.Queue
	st       *storage.Storage
}

// NewMessenger returns a new Messenger.
func NewMessenger(client *dhooks.Client, queue *queue.Queue, name, url string, st *storage.Storage, cfg config.Config) *Messenger {
	mg := &Messenger{
		cfg:      cfg,
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
		dwh:      dhooks.NewWebhook(client, url),
		name:     name,
		queue:    queue,
		st:       st,
	}
	return mg
}

// Close conducts a graceful shutdown of a message and frees it's resources.
func (mg *Messenger) Close() {
	mg.shutdown <- struct{}{}
	<-mg.done
}

func (mg *Messenger) Name() string {
	return mg.name
}

// Start starts the service.
func (mg *Messenger) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	// shutdown main goroutine when signal is received
	go func() {
		<-mg.shutdown
		cancel()
		<-stopped
		mg.done <- struct{}{}
	}()
	// main goroutine
	go func() {
		myLog := slog.With("webhook", mg.name)
		myLog.Info("Started", "queued", mg.queue.Size())
	loop:
		for {
			v, err := mg.queue.GetWithContext(ctx)
			if err == context.Canceled {
				myLog.Info("canceled")
				break
			} else if err != nil {
				myLog.Error("Failed to read from queue", "error", err)
				continue
			}
			m, err := newMessageFromBytes(v)
			if err != nil {
				myLog.Error("Failed to de-serialize message. Discarding", "error", err, "data", string(v))
				continue
			}
			dm, err := m.Item.ToDiscordMessage(mg.cfg.App.BrandingDisabled)
			if err != nil {
				myLog.Error("Failed to convert message for Discord. Discarding", "error", err, "message", m)
				continue
			}
			if err := dm.Validate(); err != nil {
				myLog.Error("Discord Message not valid. Discarding", "error", err, "message", dm)
				continue
			}
			var attempt int
			for {
				if ctx.Err() == context.Canceled {
					myLog.Info("Canceled")
					break loop
				}
				attempt++
				err = mg.dwh.Execute(dm)
				if err == nil {
					break
				}
				mg.errCount.Add(1)
				errHTTP, ok := err.(dhooks.HTTPError)
				if ok && errHTTP.Status == http.StatusBadRequest {
					myLog.Error("Bad request. Discarding", "error", err, "message", dm)
					break
				}
				err429, ok := err.(dhooks.TooManyRequestsError)
				if ok {
					myLog.Error("API rate limited exceeded", "retryAfter", err429.RetryAfter)
					time.Sleep(err429.RetryAfter)
					continue
				}
				d := maxBackoffJitter(attempt)
				myLog.Error("Failed to send to webhook. Retrying.", "error", err, "attempt", attempt, "wait", d, "message", dm)
				time.Sleep(d)
			}
			if err := mg.st.UpdateWebhookStats(mg.name, func(ws *app.WebhookStats) error {
				ws.SentCount++
				ws.SentLast = time.Now().UTC()
				return nil
			}); err != nil {
				myLog.Error("Failed to update webhook stats", "error", err)
			}
			myLog.Info("Posted item", "feed", m.Item.FeedName, "title", m.Item.Title, "queued", mg.queue.Size())
		}
		myLog.Info("Stopped")
		stopped <- struct{}{}
	}()
}

// AddMessage adds a new message for being send to to webhook
func (wh *Messenger) AddMessage(feedName string, feed *gofeed.Feed, item *gofeed.Item, isUpdated bool) error {
	p, err := newMessage(feedName, feed, item, isUpdated)
	if err != nil {
		return err
	}
	v, err := p.toBytes()
	if err != nil {
		return err
	}
	return wh.queue.Put(v)
}

type Status struct {
	QueueSize  int
	ErrorCount int
}

func (mg *Messenger) Status() Status {
	x := Status{
		QueueSize:  mg.queue.Size(),
		ErrorCount: int(mg.errCount.Load()),
	}
	return x
}

func maxBackoffJitter(attempt int) time.Duration {
	const BASE = 100
	const MAX_DELAY = 30_000
	exponential := math.Pow(2, float64(attempt)) * BASE
	delay := min(exponential, MAX_DELAY)
	ms := math.Floor(rand.Float64() * delay)
	return time.Duration(ms) * time.Millisecond
}
