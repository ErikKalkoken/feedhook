package messenger

import (
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/discordhook"
	"github.com/ErikKalkoken/feedhook/internal/queue"
)

// A Messenger handles posting messages to webhooks.
// Failed messages are automatically retried and rate limits are respected.
// Messages are kept in a permanent queue, which can survive process restarts.
type Messenger struct {
	cfg      app.MyConfig
	errCount atomic.Int64
	dwh      *discordhook.Webhook
	name     string
	queue    *queue.Queue
	st       *storage.Storage
}

func New(client *discordhook.Client, queue *queue.Queue, name, url string, st *storage.Storage, cfg app.MyConfig) *Messenger {
	mg := &Messenger{
		cfg:   cfg,
		dwh:   discordhook.NewWebhook(client, url),
		name:  name,
		queue: queue,
		st:    st,
	}
	return mg
}

func (mg *Messenger) Name() string {
	return mg.name
}

// Start starts the service.
func (mg *Messenger) Start() {
	go func() {
		myLog := slog.With("webhook", mg.name)
		myLog.Info("Started webhook", "queued", mg.queue.Size())
		for {
			v, err := mg.queue.Get()
			if err != nil {
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
				attempt++
				err = mg.dwh.Execute(dm)
				if err == nil {
					break
				}
				mg.errCount.Add(1)
				errHTTP, ok := err.(discordhook.HTTPError)
				if ok && errHTTP.Status == http.StatusBadRequest {
					myLog.Error("Bad request. Discarding", "error", err, "message", dm)
					break
				}
				err429, ok := err.(discordhook.TooManyRequestsError)
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
				myLog.Error("failed to update webhook stats", "error", err)
			}
			myLog.Info("Posted item", "feed", m.Item.FeedName, "title", m.Item.Title, "queued", mg.queue.Size())
		}
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
