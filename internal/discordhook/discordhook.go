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

type HttpError struct {
	status  int
	message string
}

func (e HttpError) Error() string {
	return e.message
}

type Clock interface {
	Now() time.Time
}

type Webhook struct {
	client *http.Client
	url    string
	arl    apiRateLimit
	wrl    webhookRateLimit
}

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func New(client *http.Client, url string) *Webhook {
	wh := &Webhook{
		client: client,
		url:    url,
		wrl:    newWebhookRateLimit(realtime{}),
	}
	return wh
}

func (wh *Webhook) Send(payload WebhookPayload) error {
	if wh.arl.isSet() {
		slog.Debug("API rate limit", "info", wh.arl)
		if wh.arl.limitExceeded(time.Now()) {
			retryAfter := roundUpDuration(time.Until(wh.arl.reset), time.Second)
			slog.Warn("API rate limit reached. Waiting for reset.", "duration", retryAfter)
			time.Sleep(retryAfter)
		}
	}
	remaining, retryAfter := wh.wrl.calc()
	slog.Debug("Webhook rate limit", "remaining", remaining, "reset", retryAfter)
	if remaining == 0 {
		slog.Warn("Webhook rate limit reached. Waiting for reset.", "duration", retryAfter)
		time.Sleep(retryAfter)
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
		err := HttpError{
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
	if !rl.isSet() || rl.reset == wh.arl.reset {
		return
	}
	wh.arl = rl
}
