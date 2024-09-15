package service

import (
	"log/slog"
	"net/http"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedforward/internal/discordhook"
	"github.com/ErikKalkoken/feedforward/internal/queue"
)

const (
	maxAttempts = 3
)

var converter = md.NewConverter("", true, nil)

func init() {
	x := md.Rule{
		Filter: []string{"img"},
		Replacement: func(_ string, _ *goquery.Selection, _ *md.Options) *string {
			return md.String("")
		},
	}
	converter.AddRules(x)
}

// A WebhookClient handles posting messages to webhooks.
// Messages are kept in a permanent queue and do not disappear after a restart.
// Failed messages are automatically retried and rate limits are respected.
type WebhookClient struct {
	name  string
	queue *queue.Queue
	wh    *discordhook.DiscordWebhook
}

func NewWebhookClient(httpClient *http.Client, queue *queue.Queue, name, url string) *WebhookClient {
	wh := &WebhookClient{
		name:  name,
		queue: queue,
		wh:    discordhook.New(httpClient, url),
	}
	return wh
}

// Start starts the service.
func (wh *WebhookClient) Start() {
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
				slog.Error("Failed to de-serialize payload", "error", err, "data", string(v))
				continue
			}
			for {
				err = wh.wh.Send(m.Payload)
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
				if err := wh.queue.Put(v); err != nil {
					slog.Error("Failed to enqueue message after failure", "error", err)
				}
				continue
			}
			slog.Info("Posted item", "webhook", wh.name, "feed", m.Feed, "title", m.Title, "queued", wh.queue.Size())
		}
	}()
}

// Add adds a new message for being send to to webhook
func (wh *WebhookClient) Add(feedName string, feed *gofeed.Feed, item *gofeed.Item) error {
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
