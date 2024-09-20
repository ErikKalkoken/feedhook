// Package discordhook provides the ability to send messages to Discord webhooks.
package discordhook

import (
	"net/http"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/rate"
)

const (
	globalRateLimitPeriod   = 1 * time.Second
	globalRateLimitRequests = 50
)

// Client represents a shared client used by all webhooks to access the Discord API.
//
// The shared client enabled dealing with the global rate limit and ensures a shared http client is used.
type Client struct {
	httpClient    *http.Client
	limiterGlobal *rate.Limiter

	rl rate.RateLimited
}

// NewClient returns a new client for webhook. All webhook share the provided HTTP client.
func NewClient(httpClient *http.Client) *Client {
	s := &Client{
		httpClient:    httpClient,
		limiterGlobal: rate.NewLimiter(globalRateLimitPeriod, globalRateLimitRequests, "global"),
	}
	return s
}
