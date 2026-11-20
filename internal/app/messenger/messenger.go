// Package messenger provides the ability to post messages to a webhook.
package messenger

import (
	"context"
	"errors"
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
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/ErikKalkoken/go-dhook"
)

const queueRetryDelay = 10 * time.Second

type Status struct {
	ErrorCount int
	QueueSize  int
}

// A Messenger handles posting messages to a webhook.
// Failed messages are automatically retried and rate limits are respected.
// Unsent messages are queued and will be picked up again after a process restart.
type Messenger struct {
	dwh                *dhook.Webhook
	errCount           atomic.Int64
	isBrandingDisabled bool
	name               string
	queue              *pqueue.PQueue
	st                 *storage.Storage

	mu        sync.Mutex         // serializes Start and Shutdown
	cancel    context.CancelFunc // commence shutdown
	done      chan struct{}      // closed when shutdown is complete
	isRunning bool
}

// NewMessenger returns a new Messenger.
func NewMessenger(client *dhook.Client, queue *pqueue.PQueue, name, url string, st *storage.Storage, isBrandingDisabled bool) *Messenger {
	mg := &Messenger{
		dwh:                client.NewWebhook(url),
		isBrandingDisabled: isBrandingDisabled,
		name:               name,
		queue:              queue,
		st:                 st,
	}
	return mg
}

// AddMessage adds a new message for being send to to webhook
func (mg *Messenger) AddMessage(feedName string, feed *gofeed.Feed, item *gofeed.Item, isUpdated bool) error {
	p, err := newMessage(feedName, feed, item, isUpdated)
	if err != nil {
		return err
	}
	v, err := p.toBytes()
	if err != nil {
		return err
	}
	return mg.queue.Put(v)
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
	mg.cancel()
	<-mg.done
	mg.isRunning = false
	return true
}

func (mg *Messenger) Status() Status {
	x := Status{
		QueueSize:  mg.queue.Size(),
		ErrorCount: int(mg.errCount.Load()),
	}
	return x
}

// Start starts the service.
func (mg *Messenger) Start() error {
	var ctx context.Context
	var done chan struct{}
	err := func() error {
		mg.mu.Lock()
		defer mg.mu.Unlock()
		if mg.isRunning {
			return fmt.Errorf("messenger %s already running", mg.name)
		}
		mg.isRunning = true
		ctx, mg.cancel = context.WithCancel(context.Background())
		mg.done = make(chan struct{})
		done = mg.done
		return nil
	}()
	if err != nil {
		return err
	}
	go func() {
		defer close(done)
		myLog := slog.With("messenger", mg.name)
		myLog.Info("Started", "queued", mg.queue.Size())
		for {
			err := mg.queue.Process(ctx, func(v []byte) error {
				// The message stays in the queue when send returns an error and is removed otherwise.
				return mg.send(ctx, v, myLog)
			})
			if errors.Is(err, context.Canceled) {
				myLog.Debug("canceled")
				break
			}
			if err != nil {
				myLog.Error("Failed to process queue item. Retrying", "error", err, "wait", queueRetryDelay)
				if !sleepContext(ctx, queueRetryDelay) {
					break
				}
			}
		}
		myLog.Info("Stopped", "queued", mg.queue.Size())
	}()
	return nil
}

// send sends message v to the webhook.
func (mg *Messenger) send(ctx context.Context, v []byte, myLog *slog.Logger) error {
	m, err := newMessageFromBytes(v)
	if err != nil {
		myLog.Error("Failed to de-serialize message. Discarding", "error", err, "data", string(v))
		return nil
	}
	dm, err := m.Item.ToDiscordMessage(mg.isBrandingDisabled)
	if err != nil {
		myLog.Error("Failed to convert message for Discord. Discarding", "error", err, "message", m)
		return nil
	}
	if err := dm.Validate(); err != nil {
		myLog.Error("Discord Message not valid. Discarding", "error", err, "message", dm)
		return nil
	}
	var attempt int
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err = mg.dwh.ExecuteContext(ctx, dm, nil)
		if err == nil {
			break
		}
		if errors.Is(err, context.Canceled) {
			return err // shutting down
		}
		mg.errCount.Add(1)
		errHTTP, ok := err.(dhook.HTTPError)
		// 400 and 413 mean the message itself is invalid
		if ok && (errHTTP.Status == http.StatusBadRequest || errHTTP.Status == http.StatusRequestEntityTooLarge) {
			myLog.Error("Webhook rejected message. Discarding", "error", err, "message", dm)
			return nil
		}
		var d time.Duration
		if err429, ok := err.(dhook.TooManyRequestsError); ok {
			myLog.Error("API rate limited exceeded", "retryAfter", err429.RetryAfter)
			d = err429.RetryAfter
		} else {
			attempt++
			d = maxBackoffJitter(attempt)
			myLog.Error("Failed to send to webhook. Retrying.", "error", err, "attempt", attempt, "wait", d, "message", dm)
		}
		if !sleepContext(ctx, d) {
			return ctx.Err()
		}
	}
	err = mg.st.UpdateOrCreateWebhookStats(mg.name, func(ws *app.WebhookStats) error {
		ws.SentCount++
		ws.SentLast = time.Now().UTC()
		return nil
	})
	if err != nil {
		myLog.Error("Failed to update webhook stats", "error", err)
	}
	myLog.Info("Posted item", "feed", m.Item.FeedName, "title", m.Item.Title)
	return nil
}

func maxBackoffJitter(attempt int) time.Duration {
	const (
		base     = 100
		maxDelay = 30_000
	)
	exponential := math.Pow(2, float64(attempt)) * base
	delay := min(exponential, maxDelay)
	ms := math.Floor(rand.Float64() * delay)
	return time.Duration(ms) * time.Millisecond
}

// sleepContext waits for d or until ctx is done. Reports whether the full duration elapsed.
func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
