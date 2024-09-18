package discordhook

import (
	"fmt"
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

type RateLimitedError struct {
	RetryAfter time.Duration
	Type       RateLimitType
}

func (e RateLimitedError) Error() string {
	return fmt.Sprintf("%s rate limited. Retry After %v", e.Type, e.RetryAfter)
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
