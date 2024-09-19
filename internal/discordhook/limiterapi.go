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

func (l limiterAPI) String() string {
	return fmt.Sprintf(
		"limit:%d remaining:%d reset:%s resetAfter:%f",
		l.limit,
		l.remaining,
		l.resetAt, time.Until(l.resetAt).Seconds(),
	)
}

func (l limiterAPI) wait() {
	slog.Debug("API rate limit", "info", l)
	if l.limitExceeded(time.Now()) {
		retryAfter := roundUpDuration(time.Until(l.resetAt), time.Second)
		slog.Warn("API rate limit exhausted. Waiting for reset", "retryAfter", retryAfter)
		time.Sleep(retryAfter)
	}
}

func (l limiterAPI) isSet() bool {
	return !l.timestamp.IsZero()
}

func (l limiterAPI) limitExceeded(now time.Time) bool {
	if !l.isSet() {
		return false
	}
	if l.remaining > 0 {
		return false
	}
	if l.resetAt.Before(now) {
		return false
	}
	return true
}

// updateFromHeader updates the limiter from a header.
func (l *limiterAPI) updateFromHeader(h http.Header) error {
	if l.remaining > 0 {
		l.remaining--
	}
	l2, err := newLimiterAPIFromHeader(h)
	if err != nil {
		return err
	}
	if !l2.isSet() {
		return nil
	}
	if l2.bucket == l.bucket && l2.resetAt == l.resetAt {
		return nil
	}
	*l = l2
	return nil
}

func newLimiterAPIFromHeader(h http.Header) (limiterAPI, error) {
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
