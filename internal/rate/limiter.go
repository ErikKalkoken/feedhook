// Package rate provides types for dealing with rate limits.
package rate

import (
	"log/slog"
	"sync"
	"time"
)

// Limiter represents a rate Limiter implementing the sliding log algorithm.
// This type is safe to use concurrently.
type Limiter struct {
	max    int
	name   string
	period time.Duration

	mu      sync.Mutex
	entries []time.Time
	index   int
}

// NewLimiter returns a new Limiter object.
func NewLimiter(period time.Duration, max int, name string) *Limiter {
	l := Limiter{
		index:  0,
		max:    max,
		name:   name,
		period: period,
	}
	l.entries = make([]time.Time, max)
	before := time.Now().Add(-2 * period)
	for i := range max {
		l.entries[i] = before
	}
	return &l
}

// Wait will register a new event.
// In case the current tick is exhausted it will block until the tick is reset.
// The Wait duration will be rounded up to the next rate tick (e.g. 100ms if the rate is 10/sec)
func (l *Limiter) Wait() {
	l.mu.Lock()
	defer l.mu.Unlock()
	last := l.entries[l.index]
	next := last.Add(l.period)
	if now := time.Now(); now.Before(next) {
		d := roundUpDuration(next.Sub(now), l.period/time.Duration(l.max))
		slog.Info("Rate limit exhausted. Waiting for reset", "retryAfter", d, "name", l.name)
		time.Sleep(d)
	}
	l.entries[l.index] = time.Now()
	l.index = l.index + 1
	if l.index == l.max {
		l.index = 0
	}
}

func roundUpDuration(d time.Duration, m time.Duration) time.Duration {
	x := d.Round(m)
	if x < d {
		return x + m
	}
	return x
}
