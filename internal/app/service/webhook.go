package service

import (
	"log/slog"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/discordhook"
	"github.com/ErikKalkoken/feedhook/internal/queue"
)

const (
	maxAttempts = 3
)

// A Webhook handles posting messages to webhooks.
// Messages are kept in a permanent queue and do not disappear after a restart.
// Failed messages are automatically retried and rate limits are respected.
type Webhook struct {
	cfg   app.MyConfig
	dwh   *discordhook.Webhook
	name  string
	queue *queue.Queue
	st    *storage.Storage
}

func NewWebhook(client *discordhook.Client, queue *queue.Queue, name, url string, st *storage.Storage, cfg app.MyConfig) *Webhook {
	wh := &Webhook{
		cfg:   cfg,
		dwh:   discordhook.NewWebhook(client, url),
		name:  name,
		queue: queue,
		st:    st,
	}
	return wh
}

// Start starts the service.
func (wh *Webhook) Start() {
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
				myLog.Error("Failed to de-serialize message", "error", err, "data", string(v))
				continue
			}
			pl, err := m.Item.ToDiscordPayload(wh.cfg.App.BrandingDisabled)
			if err != nil {
				myLog.Error("Failed to convert message to payload", "error", err, "data", string(v))
			}
			for {
				err = wh.dwh.Send(pl)
				if err == nil {
					break
				}
				errRateLimit, ok := err.(discordhook.RateLimitedError)
				if !ok {
					break
				}
				myLog.Warn("rate limited", "type", errRateLimit.Type, "retryAfter", errRateLimit.RetryAfter)
				time.Sleep(errRateLimit.RetryAfter)
			}
			if err != nil {
				m.Attempt++
				myLog.Error("Failed to send to webhook", "error", err, "attempt", m.Attempt)
				if err := wh.st.UpdateWebhookStats(wh.name, func(ws *app.WebhookStats) error {
					ws.ErrorCount++
					return nil
				}); err != nil {
					myLog.Error("failed to update webhook stats", "error", err)
				}
				if m.Attempt == maxAttempts {
					myLog.Error("Discarding message after too many attempts")
					continue
				}
				v, err := m.toBytes()
				if err != nil {
					myLog.Error("Failed to serialize message after failure", "error", err)
					continue
				}
				if err := wh.queue.Put(v); err != nil {
					myLog.Error("Failed to enqueue message after failure", "error", err)
				}
				continue
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

// Add adds a new message for being send to to webhook
func (wh *Webhook) Add(feedName string, feed *gofeed.Feed, item *gofeed.Item) error {
	p, err := newMessage(feedName, feed, item)
	if err != nil {
		return err
	}
	v, err := p.toBytes()
	if err != nil {
		return err
	}
	return wh.queue.Put(v)
}
