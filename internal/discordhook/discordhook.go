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

// ErrHTTP represents a HTTP error, e.g. 400 Bad Request
type ErrHTTP struct {
	status  int
	message string
}

func (e ErrHTTP) Error() string {
	return e.message
}

type RateLimitType uint

func (rlt RateLimitType) String() string {
	switch rlt {
	case APIRateLimit:
		return "api"
	case WebhookRateLimit:
		return "webhook"
	case BreachedRateLimit:
		return "breached"
	}
	panic("not implemented")
}

const (
	APIRateLimit RateLimitType = iota
	WebhookRateLimit
	BreachedRateLimit
)

type ErrRateLimited struct {
	RetryAfter time.Duration
	Type       RateLimitType
}

func (e ErrRateLimited) Error() string {
	return fmt.Sprintf("%s rate limited. Retry After %v", e.Type, e.RetryAfter)
}

type Clock interface {
	Now() time.Time
}

// DiscordWebhook represents a Discord webhook.
type DiscordWebhook struct {
	client *http.Client
	arl    apiRateLimit
	brl    breachedRateLimit
	wrl    webhookRateLimit
	url    string
}

type breachedRateLimit struct {
	resetAt time.Time
}

func (brl breachedRateLimit) String() string {
	return fmt.Sprintf("resetAt: %v", brl.resetAt)
}

func (brl *breachedRateLimit) retryAfter() time.Duration {
	if brl.resetAt.IsZero() {
		return 0
	}
	d := time.Until(brl.resetAt)
	if d < 0 {
		brl.resetAt = time.Time{}
	}
	return d
}

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

// New returns a new webhook.
func New(client *http.Client, url string) *DiscordWebhook {
	wh := &DiscordWebhook{
		client: client,
		url:    url,
		wrl:    newWebhookRateLimit(realtime{}),
	}
	return wh
}

// Send sends a payload to the webhook.
//
// Returns ErrHTTP errors for HTTP errors.
// And ErrRateLimited errors when a rate limit is reached or exceeded.
func (wh *DiscordWebhook) Send(payload WebhookPayload) error {
	slog.Debug("Breached rate limit", "info", wh.brl)
	if retryAfter := wh.brl.retryAfter(); retryAfter > 0 {
		return ErrRateLimited{RetryAfter: retryAfter, Type: BreachedRateLimit}
	}
	if wh.arl.isSet() {
		slog.Debug("API rate limit", "info", wh.arl)
		if wh.arl.limitExceeded(time.Now()) {
			retryAfter := roundUpDuration(time.Until(wh.arl.resetAt), time.Second)
			return ErrRateLimited{RetryAfter: retryAfter, Type: APIRateLimit}
		}
	}
	remaining, retryAfter := wh.wrl.calc()
	slog.Debug("Webhook rate limit", "remaining", remaining, "reset", retryAfter)
	if remaining == 0 {
		return ErrRateLimited{RetryAfter: retryAfter, Type: WebhookRateLimit}
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
		return ErrRateLimited{RetryAfter: retryAfter, Type: BreachedRateLimit}
	}
	if resp.StatusCode >= 400 {
		err := ErrHTTP{
			status:  resp.StatusCode,
			message: resp.Status,
		}
		return err
	}
	return nil
}

func (wh *DiscordWebhook) updateAPIRateLimit(h http.Header) {
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
