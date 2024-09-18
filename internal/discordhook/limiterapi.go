package discordhook

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// limiterAPI implements a limiter from the API rate limit
// as communicated by "X-RateLimit-" response headers.
type limiterAPI struct {
	limit      int
	remaining  int
	resetAt    time.Time
	resetAfter float64
	bucket     string
	timestamp  time.Time
}

func (rl limiterAPI) String() string {
	return fmt.Sprintf(
		"limit:%d remaining:%d reset:%s resetAfter:%f",
		rl.limit,
		rl.remaining,
		rl.resetAt, time.Until(rl.resetAt).Seconds(),
	)
}

func (rl limiterAPI) wait() {
	if rl.isSet() {
		slog.Debug("API rate limit", "info", rl)
		if rl.limitExceeded(time.Now()) {
			retryAfter := roundUpDuration(time.Until(rl.resetAt), time.Second)
			slog.Warn("API rate limit exhausted. Waiting for reset", "retryAfter", retryAfter)
			time.Sleep(retryAfter)
		}
	}
}

func (rl limiterAPI) isSet() bool {
	return !rl.timestamp.IsZero()
}

func (rl limiterAPI) limitExceeded(now time.Time) bool {
	if !rl.isSet() {
		return false
	}
	if rl.remaining > 0 {
		return false
	}
	if rl.resetAt.Before(now) {
		return false
	}
	return true
}

func rateLimitFromHeader(h http.Header) (limiterAPI, error) {
	var r limiterAPI
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
	r.resetAt = time.Unix(int64(resetEpoch), 0).UTC()
	r.resetAfter, err = strconv.ParseFloat(resetAfter, 64)
	if err != nil {
		return r, err
	}
	r.bucket = bucket
	r.timestamp = time.Now().UTC()
	return r, nil
}
