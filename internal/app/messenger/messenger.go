package messenger

import (
	"errors"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
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
	cfg   app.MyConfig
	dwh   *discordhook.Webhook
	name  string
	queue *queue.Queue
	st    *storage.Storage
}

func New(client *discordhook.Client, queue *queue.Queue, name, url string, st *storage.Storage, cfg app.MyConfig) *Messenger {
	wh := &Messenger{
		cfg:   cfg,
		dwh:   discordhook.NewWebhook(client, url),
		name:  name,
		queue: queue,
		st:    st,
	}
	return wh
}

func (wh *Messenger) Name() string {
	return wh.name
}

// Start starts the service.
func (wh *Messenger) Start() {
	go func() {
		myLog := slog.With("webhook", wh.name)
		myLog.Info("Started webhook", "queued", wh.queue.Size())
		for {
			v, err := wh.queue.Get()
			if err != nil {
				myLog.Error("Failed to read from queue", "error", err)
				continue
			}
			m, err := newMessageFromBytes(v)
			if err != nil {
				myLog.Error("Failed to de-serialize message. Discarding", "error", err, "data", string(v))
				continue
			}
			dm, err := m.Item.ToDiscordMessage(wh.cfg.App.BrandingDisabled)
			if err != nil {
				myLog.Error("Failed to convert message for Discord. Discarding", "error", err, "message", m)
				continue
			}
			var attempt int
			for {
				attempt++
				err = wh.dwh.Execute(dm)
				if err == nil {
					break
				}
				if err := wh.st.UpdateWebhookStats(wh.name, func(ws *app.WebhookStats) error {
					ws.ErrorCount++
					return nil
				}); err != nil {
					myLog.Error("failed to update webhook stats", "error", err)
				}
				if errors.Is(err, discordhook.ErrInvalidMessage) {
					myLog.Error("Discord Message not valid. Discarding", "error", err, "message", dm)
					break
				}
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
			if err := wh.st.UpdateWebhookStats(wh.name, func(ws *app.WebhookStats) error {
				ws.SentCount++
				ws.SentLast = time.Now().UTC()
				return nil
			}); err != nil {
				myLog.Error("failed to update webhook stats", "error", err)
			}
			myLog.Info("Posted item", "feed", m.Item.FeedName, "title", m.Item.Title, "queued", wh.queue.Size())
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

func (wh *Messenger) QueueSize() int {
	return wh.queue.Size()
}

func maxBackoffJitter(attempt int) time.Duration {
	const BASE = 100
	const MAX_DELAY = 30_000
	exponential := math.Pow(2, float64(attempt)) * BASE
	delay := min(exponential, MAX_DELAY)
	ms := math.Floor(rand.Float64() * delay)
	return time.Duration(ms) * time.Millisecond
}
