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
	limiterAPI     limiterAPI
	brl            breachedRateLimit
	client         *Client
	url            string
	limiterWebhook *limiter
}

// NewWebhook returns a new webhook.
func NewWebhook(client *Client, url string) *Webhook {
	wh := &Webhook{
		client:         client,
		url:            url,
		limiterWebhook: newLimiter(webhookRateLimitPeriod, webhookRateLimitRequests, "webhook"),
	}
	return wh
}

// Send sends a payload to the webhook.
//
// If a rate limit is exceeded the method will wait until the reset.
// HTTP errors are returns as HTTPError. 429s are returns as TooManyRequestsError.
func (wh *Webhook) Send(payload WebhookPayload) error {
	if retryAfter := wh.brl.retryAfter(); retryAfter > 0 {
		return TooManyRequestsError{RetryAfter: retryAfter}
	}
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	v := url.Values{}
	v.Set("wait", "true")
	u := fmt.Sprintf("%s?%s", wh.url, v.Encode())
	wh.client.limiterGlobal.wait()
	wh.limiterAPI.wait()
	wh.limiterWebhook.wait()
	slog.Debug("request", "url", wh.url, "body", string(dat))
	resp, err := wh.client.httpClient.Post(u, "application/json", bytes.NewBuffer(dat))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := wh.limiterAPI.updateFromHeader(resp.Header); err != nil {
		slog.Error("Failed to update API limiter from header", "error", err)
	}
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
		wh.brl.resetAt = time.Now().Add(retryAfter)
		return TooManyRequestsError{RetryAfter: retryAfter}
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
