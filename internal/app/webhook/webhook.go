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

type Webhook struct {
	client *http.Client
	name   string
	queue  *queue.Queue
	url    string
	arl    apiRateLimit
	wrl    webhookRateLimit
}

// apiRateLimit represents the official API rate limit
// as communicated through through "X-RateLimit-" headers.
type apiRateLimit struct {
	limit      int
	remaining  int
	reset      time.Time
	resetAfter float64
	bucket     string
	timestamp  time.Time
}

func (rl apiRateLimit) String() string {
	return fmt.Sprintf(
		"limit:%d remaining:%d reset:%s resetAfter:%f",
		rl.limit,
		rl.remaining,
		rl.reset, time.Until(rl.reset).Seconds(),
	)
}

func (rl apiRateLimit) IsSet() bool {
	return !rl.timestamp.IsZero()
}

func (rl apiRateLimit) limitExceeded(now time.Time) bool {
	if !rl.IsSet() {
		return false
	}
	if rl.remaining > 0 {
		return false
	}
	if rl.reset.Before(now) {
		return false
	}
	return true
}

func rateLimitFromHeader(h http.Header) (apiRateLimit, error) {
	var r apiRateLimit
	var err error
	limit := h.Get("X-RateLimit-Limit")
	if limit == "" {
		return r, nil
	}
	remaining := h.Get("X-RateLimit-Remaining")
	if remaining == "" {
		return r, nil
	}
	reset := h.Get("X-RateLimit-Reset")
	if reset == "" {
		return r, nil
	}
	resetAfter := h.Get("X-RateLimit-Reset-After")
	if resetAfter == "" {
		return r, nil
	}
	bucket := h.Get("X-RateLimit-Bucket")
	if bucket == "" {
		return r, nil
	}
	r.limit, err = strconv.Atoi(limit)
	if err != nil {
		return r, err
	}
	r.remaining, err = strconv.Atoi(remaining)
	if err != nil {
		return r, err
	}
	resetEpoch, err := strconv.Atoi(reset)
	if err != nil {
		return r, err
	}
	r.reset = time.Unix(int64(resetEpoch), 0).UTC()
	r.resetAfter, err = strconv.ParseFloat(resetAfter, 64)
	if err != nil {
		return r, err
	}
	r.bucket = bucket
	r.timestamp = time.Now().UTC()
	return r, nil
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
	if !rl.IsSet() || rl.reset == wh.arl.reset {
		return
	}
	wh.arl = rl
}

func (wh *Webhook) sendToWebhook(payload WebhookPayload) error {
	slog.Info("API rate limit", "info", wh.arl)
	if wh.arl.limitExceeded(time.Now()) {
		w := time.Until(wh.arl.reset)
		slog.Info("API rate limit reached. Waiting for reset.", "duration", w)
		time.Sleep(w)
	}
	remaining, reset := wh.wrl.calc()
	slog.Info("Webhook rate limit", "remaining", remaining, "reset", reset)
	if remaining == 0 {
		slog.Info("Webhook rate limit reached. Waiting for reset.", "duration", reset)
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
	wh.wrl.record()
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
