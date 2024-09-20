package discordhook

import (
	"log/slog"
	"sync"
	"time"
)

// limiter represents a rate limiter implementing the sliding log algorithm.
// This type is safe to use concurrently.
type limiter struct {
	max    int
	name   string
	period time.Duration

	mu      sync.Mutex
	entries []time.Time
	index   int
}

// newLimiter returns a new Limiter object.
func newLimiter(period time.Duration, max int, name string) *limiter {
	l := limiter{
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

// wait will register a new event.
// In case the current tick is exhausted it will block until the tick is reset.
// The wait duration will be rounded up to the next rate tick (e.g. 100ms if the rate is 10/sec)
func (l *limiter) wait() {
	l.mu.Lock()
	defer l.mu.Unlock()
	last := l.entries[l.index]
	next := last.Add(l.period)
	if now := time.Now(); now.Before(next) {
		d := roundUpDuration(next.Sub(now), l.period/time.Duration(l.max))
		slog.Warn("Rate limit exhausted. Waiting for reset", "retryAfter", d, "name", l.name)
		time.Sleep(d)
	}
	l.entries[l.index] = time.Now()
	l.index = l.index + 1
	if l.index == l.max {
		l.index = 0
	}
}
