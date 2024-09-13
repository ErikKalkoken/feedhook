package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedforward/internal/queue"
)

const maxAttempts = 3

var converter = md.NewConverter("", true, nil)

type httpError struct {
	status  int
	message string
}

func (e httpError) Error() string {
	return e.message
}

func init() {
	x := md.Rule{
		Filter: []string{"img"},
		Replacement: func(_ string, _ *goquery.Selection, _ *md.Options) *string {
			return md.String("")
		},
	}
	converter.AddRules(x)
}

type Webhook struct {
	client *http.Client
	name   string
	queue  *queue.Queue
	url    string
}

func New(client *http.Client, queue *queue.Queue, name, url string) *Webhook {
	wh := &Webhook{
		client: client,
		name:   name,
		queue:  queue,
		url:    url,
	}
	return wh
}

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
				slog.Error("Failed to de-serialize payload", "error", err, "data", string(v))
				continue
			}
			if err := wh.sendToWebhook(m.Payload); err != nil {
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

func (wh *Webhook) Send(feedName string, feed *gofeed.Feed, item *gofeed.Item) error {
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

func (wh *Webhook) sendToWebhook(payload WebhookPayload) error {
	time.Sleep(1 * time.Second)
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	v := url.Values{}
	v.Set("wait", "true")
	u := fmt.Sprintf("%s?%s", wh.url, v.Encode())
	slog.Debug("request", "url", wh.url, "body", string(dat))
	resp, err := wh.client.Post(u, "application/json", bytes.NewBuffer(dat))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Debug("response", "url", wh.url, "status", resp.Status, "headers", resp.Header, "body", string(body))
	if resp.StatusCode >= 400 {
		slog.Warn("response", "url", wh.url, "status", resp.Status)
	} else {
		slog.Info("response", "url", wh.url, "status", resp.Status)
	}
	if resp.StatusCode == 429 {
		retryAfter, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			slog.Error("failed to parse retry after. Assuming default.", "error", err)
			retryAfter = 300
		}
		slog.Warn("Waiting as requested by 429 error", "retryAfter", retryAfter)
		time.Sleep(time.Duration(retryAfter) * time.Second)
	}
	if resp.StatusCode >= 400 {
		err := httpError{
			status:  resp.StatusCode,
			message: resp.Status,
		}
		return err
	}
	return nil
}
