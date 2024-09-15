package service

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

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
	name  string
	queue *queue.Queue
	dwh   *discordhook.DiscordWebhook
}

func NewWebhook(httpClient *http.Client, queue *queue.Queue, name, url string) *Webhook {
	wc := &Webhook{
		name:  name,
		queue: queue,
		dwh:   discordhook.New(httpClient, url),
	}
	return wc
}

// Start starts the service.
func (wc *Webhook) Start() {
	go func() {
		slog.Info("Started webhook", "name", wc.name, "queued", wc.queue.Size())
		for {
			v, err := wc.queue.Get()
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
				err = wc.dwh.Send(pl)
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
				if m.Attempt == maxAttempts {
					slog.Error("Discarding message after too many attempts")
					continue
				}
				v, err := m.toBytes()
				if err != nil {
					slog.Error("Failed to serialize message after failure", "error", err)
					continue
				}
				if err := wc.queue.Put(v); err != nil {
					slog.Error("Failed to enqueue message after failure", "error", err)
				}
				continue
			}
			slog.Info("Posted item", "webhook", wc.name, "feed", m.Item.FeedName, "title", m.Item.Title, "queued", wc.queue.Size())
		}
	}()
}

// Add adds a new message for being send to to webhook
func (wc *Webhook) Add(feedName string, feed *gofeed.Feed, item *gofeed.Item) error {
	p, err := newMessage(feedName, feed, item)
	if err != nil {
		return err
	}
	v, err := p.toBytes()
	if err != nil {
		return err
	}
	return wc.queue.Put(v)
}
