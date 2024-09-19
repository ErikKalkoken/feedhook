package discordhook

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
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
	mu             sync.Mutex
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

// Execute posts a message to the configured webhook.
//
// Execute respects Discord's rate limits and will wait until there is a free slot to post the message.
// Execute can is thread safe.
//
// HTTP status codes of 400 or above are returns as [HTTPError],
// except for 429s, which are returns as [TooManyRequestsError].
func (wh *Webhook) Execute(payload Message) error {
	wh.mu.Lock()
	defer wh.mu.Unlock()
	if retryAfter := wh.brl.retryAfter(); retryAfter > 0 {
		return TooManyRequestsError{RetryAfter: retryAfter}
	}
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	wh.client.limiterGlobal.wait()
	wh.limiterAPI.Wait()
	wh.limiterWebhook.wait()
	slog.Debug("request", "url", wh.url, "body", string(dat))
	resp, err := wh.client.httpClient.Post(wh.url, "application/json", bytes.NewBuffer(dat))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := wh.limiterAPI.UpdateFromHeader(resp.Header); err != nil {
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
