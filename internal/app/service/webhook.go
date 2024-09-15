package service

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedforward/internal/app"
	"github.com/ErikKalkoken/feedforward/internal/app/storage"
	"github.com/ErikKalkoken/feedforward/internal/discordhook"
	"github.com/ErikKalkoken/feedforward/internal/queue"
)

const (
	maxAttempts = 3
)

// A Webhook handles posting messages to webhooks.
// Messages are kept in a permanent queue and do not disappear after a restart.
// Failed messages are automatically retried and rate limits are respected.
type Webhook struct {
	dwh   *discordhook.DiscordWebhook
	name  string
	queue *queue.Queue
	st    *storage.Storage
}

func NewWebhook(httpClient *http.Client, queue *queue.Queue, name, url string, st *storage.Storage) *Webhook {
	wh := &Webhook{
		dwh:   discordhook.New(httpClient, url),
		name:  name,
		queue: queue,
		st:    st,
	}
	return wh
}

// Start starts the service.
func (wh *Webhook) Start() {
	go func() {
		slog.Info("Started webhook", "name", wh.name, "queued", wh.queue.Size())
		for {
			v, err := wh.queue.Get()
			if err != nil {
				slog.Error("Failed to read from queue", "error", err)
				continue
			}
			m, err := newMessageFromBytes(v)
			if err != nil {
				slog.Error("Failed to de-serialize message", "error", err, "data", string(v))
				continue
			}
			pl, err := m.Item.ToDiscordPayload()
			if err != nil {
				slog.Error("Failed to convert message to payload", "error", err, "data", string(v))
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
				slog.Warn("rate limited", "type", errRateLimit.Type, "retryAfter", errRateLimit.RetryAfter)
				time.Sleep(errRateLimit.RetryAfter)
			}
			if err != nil {
				m.Attempt++
				slog.Error("Failed to send to webhook", "error", err, "attempt", m.Attempt)
				if err := wh.st.UpdateWebhookStats(wh.name, func(ws *app.WebhookStats) error {
					ws.ErrorCount++
					return nil
				}); err != nil {
					slog.Error("failed to update webhook stats", "name", wh.name, "error", err)
				}
				if m.Attempt == maxAttempts {
					slog.Error("Discarding message after too many attempts")
					continue
				}
				v, err := m.toBytes()
				if err != nil {
					slog.Error("Failed to serialize message after failure", "error", err)
					continue
				}
				if err := wh.queue.Put(v); err != nil {
					slog.Error("Failed to enqueue message after failure", "error", err)
				}
				continue
			}
			if err := wh.st.UpdateWebhookStats(wh.name, func(ws *app.WebhookStats) error {
				ws.SentCount++
				ws.SentLast = time.Now().UTC()
				return nil
			}); err != nil {
				slog.Error("failed to update webhook stats", "name", wh.name, "error", err)
			}
			slog.Info("Posted item", "webhook", wh.name, "feed", m.Item.FeedName, "title", m.Item.Title, "queued", wh.queue.Size())
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
