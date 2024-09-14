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

const (
	maxAttempts                     = 3
	retryAfterTooManyRequestDefault = 60 * time.Second
)

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

// A Webhook handles posting messages to Discord webhooks.
// It has a permanent queue so it does not forget messages after a restart.
// It respects Discord rate limits and will automatically retry sending failed messages
type Webhook struct {
	client *http.Client
	name   string
	queue  *queue.Queue
	url    string
	arl    apiRateLimit
	wrl    webhookRateLimit
}

func New(client *http.Client, queue *queue.Queue, name, url string, clock clock) *Webhook {
	wh := &Webhook{
		client: client,
		name:   name,
		queue:  queue,
		url:    url,
		wrl:    newWebhookRateLimit(clock),
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

func (wh *Webhook) updateAPIRateLimit(h http.Header) {
	if wh.arl.remaining > 0 {
		wh.arl.remaining--
	}
	rl, err := rateLimitFromHeader(h)
	if err != nil {
		slog.Warn("failed to parse rate limit header", "error", err)
		return
	}
	if !rl.isSet() || rl.reset == wh.arl.reset {
		return
	}
	wh.arl = rl
}

func (wh *Webhook) sendToWebhook(payload WebhookPayload) error {
	slog.Debug("API rate limit", "info", wh.arl)
	if wh.arl.limitExceeded(time.Now()) {
		w := time.Until(wh.arl.reset)
		slog.Warn("API rate limit reached. Waiting for reset.", "duration", w)
		time.Sleep(w)
	}
	remaining, reset := wh.wrl.calc()
	slog.Debug("Webhook rate limit", "remaining", remaining, "reset", reset)
	if remaining == 0 {
		slog.Warn("Webhook rate limit reached. Waiting for reset.", "duration", reset)
		time.Sleep(reset)
	}
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
	wh.updateAPIRateLimit(resp.Header)
	wh.wrl.recordRequest()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Debug("response", "url", wh.url, "status", resp.Status, "headers", resp.Header, "body", string(body))
	if resp.StatusCode >= http.StatusBadRequest {
		slog.Warn("response", "url", wh.url, "status", resp.Status)
	} else {
		slog.Info("response", "url", wh.url, "status", resp.Status)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		var retryAfter time.Duration
		x, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			slog.Warn("Failed to parse retry after. Assuming default", "error", err)
			retryAfter = retryAfterTooManyRequestDefault
		} else {
			retryAfter = time.Duration(x) * time.Second
		}
		slog.Info("429 limit reached. Waiting for reset", "retryAfter", retryAfter)
		time.Sleep(retryAfter)
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
