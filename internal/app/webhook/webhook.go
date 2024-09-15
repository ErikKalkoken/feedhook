package webhook

import (
	"log/slog"
	"net/http"

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

// A WebhookService handles posting messages to webhooks.
// It has a permanent queue so it does not forget messages after a restart
// and automatically retries failed posts.
type WebhookService struct {
	name  string
	queue *queue.Queue
	wh    *discordhook.Webhook
}

func NewWebhookService(client *http.Client, queue *queue.Queue, name, url string) *WebhookService {
	wh := &WebhookService{
		name:  name,
		queue: queue,
		wh:    discordhook.New(client, url),
	}
	return wh
}

// Start starts the service.
func (wh *WebhookService) Start() {
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
			if err := wh.wh.Send(m.Payload); err != nil {
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
func (wh *WebhookService) Add(feedName string, feed *gofeed.Feed, item *gofeed.Item) error {
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
