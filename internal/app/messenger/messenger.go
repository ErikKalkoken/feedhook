package messenger

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/ErikKalkoken/go-dhook"
)

// A Messenger handles posting messages to a webhook.
// Failed messages are automatically retried and rate limits are respected.
// Unsent messages are queued and will be picked up again after a process restart.
type Messenger struct {
	cfg      config.Config
	shutdown chan struct{} // commence shutdown
	done     chan struct{} // shutdown completed
	dwh      *dhook.Webhook
	errCount atomic.Int64
	name     string
	queue    *pqueue.PQueue
	st       *storage.Storage

	mu        sync.Mutex
	isRunning bool
}

// NewMessenger returns a new Messenger.
func NewMessenger(client *dhook.Client, queue *pqueue.PQueue, name, url string, st *storage.Storage, cfg config.Config) *Messenger {
	mg := &Messenger{
		cfg:      cfg,
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
		dwh:      client.NewWebhook(url),
		name:     name,
		queue:    queue,
		st:       st,
	}
	return mg
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

func (mg *Messenger) Name() string {
	return mg.name
}

// Shutdown conducts a graceful shutdown of a messenger and frees it's resources.
// Reports wether a shutdown was actually conducted.
func (mg *Messenger) Shutdown() bool {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	if !mg.isRunning {
		return false
	}
	mg.shutdown <- struct{}{}
	<-mg.done
	mg.isRunning = false
	return true
}

// Start starts the service.
func (mg *Messenger) Start() error {
	if err := func() error {
		mg.mu.Lock()
		defer mg.mu.Unlock()
		if mg.isRunning {
			return fmt.Errorf("messenger %s already running", mg.name)
		}
		mg.isRunning = true
		return nil
	}(); err != nil {
		return err
	}

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
		myLog := slog.With("messenger", mg.name)
		myLog.Info("Started", "queued", mg.queue.Size())
	loop:
		for {
			v, err := mg.queue.GetWithContext(ctx)
			if err == context.Canceled {
				myLog.Debug("canceled")
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
					myLog.Debug("Canceled")
					break loop
				}
				attempt++
				_, err = mg.dwh.Execute(dm, nil)
				if err == nil {
					break
				}
				mg.errCount.Add(1)
				errHTTP, ok := err.(dhook.HTTPError)
				if ok && errHTTP.Status == http.StatusBadRequest {
					myLog.Error("Bad request. Discarding", "error", err, "message", dm)
					break
				}
				err429, ok := err.(dhook.TooManyRequestsError)
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
	return nil
}

func maxBackoffJitter(attempt int) time.Duration {
	const BASE = 100
	const MAX_DELAY = 30_000
	exponential := math.Pow(2, float64(attempt)) * BASE
	delay := min(exponential, MAX_DELAY)
	ms := math.Floor(rand.Float64() * delay)
	return time.Duration(ms) * time.Millisecond
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
