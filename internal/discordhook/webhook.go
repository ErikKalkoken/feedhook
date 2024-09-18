package discordhook

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
)

const (
	retryAfterTooManyRequestDefault = 60 * time.Second
)

// HTTPError represents a HTTP error, e.g. 400 Bad Request
type HTTPError struct {
	status  int
	message string
}

func (e HTTPError) Error() string {
	return e.message
}

// Webhook represents a Discord webhook which respects rate limits.
type Webhook struct {
	arl    apiRateLimit
	brl    breachedRateLimit
	client *Client
	url    string
	wrl    rateLimit
}

// NewWebhook returns a new webhook.
func NewWebhook(client *Client, url string) *Webhook {
	wh := &Webhook{
		client: client,
		url:    url,
		wrl:    newRateLimit(WebhookRateLimit, realtime{}),
	}
	return wh
}

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

// Send sends a payload to the webhook.
//
// When a rate limit is exceeded or breached a RateLimitedError error is returned.
// HTTP errors are returns as HTTPError errors.
func (wh *Webhook) Send(payload WebhookPayload) error {
	slog.Debug("Breached rate limit", "info", wh.brl)
	if retryAfter := wh.brl.retryAfter(); retryAfter > 0 {
		return RateLimitedError{RetryAfter: retryAfter, Type: BreachedRateLimit}
	}
	if wh.arl.isSet() {
		slog.Debug("API rate limit", "info", wh.arl)
		if wh.arl.limitExceeded(time.Now()) {
			retryAfter := roundUpDuration(time.Until(wh.arl.resetAt), time.Second)
			return RateLimitedError{RetryAfter: retryAfter, Type: APIRateLimit}
		}
	}
	remaining, retryAfter := wh.wrl.calc()
	slog.Debug("Webhook rate limit", "remaining", remaining, "reset", retryAfter)
	if remaining == 0 {
		return RateLimitedError{RetryAfter: retryAfter, Type: WebhookRateLimit}
	}
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	v := url.Values{}
	v.Set("wait", "true")
	u := fmt.Sprintf("%s?%s", wh.url, v.Encode())
	slog.Debug("request", "url", wh.url, "body", string(dat))
	resp, err := wh.client.httpClient.Post(u, "application/json", bytes.NewBuffer(dat))
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
		return RateLimitedError{RetryAfter: retryAfter, Type: BreachedRateLimit}
	}
	if resp.StatusCode >= 400 {
		err := HTTPError{
			status:  resp.StatusCode,
			message: resp.Status,
		}
		return err
	}
	return nil
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
	if !rl.isSet() || rl.resetAt == wh.arl.resetAt {
		return
	}
	wh.arl = rl
}
