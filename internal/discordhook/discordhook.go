package discordhook

import (
	"net/http"
	"time"
)

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
	BreachedRateLimit
	GlobalRateLimit
	WebhookRateLimit
)

type TooManyRequestsError struct {
	RetryAfter time.Duration
}

func (e TooManyRequestsError) Error() string {
	return "too many requests"
}

// Client represents a shared client used by each webhook to access the Discord API.
// A shared client enables dealing with the global rate limit and ensures a shared http client is used.
type Client struct {
	httpClient *http.Client
	grl        rateLimit
}

func NewClient(httpClient *http.Client) *Client {
	s := &Client{
		httpClient: httpClient,
		grl:        newRateLimit(GlobalRateLimit, realtime{}),
	}
	return s
}
