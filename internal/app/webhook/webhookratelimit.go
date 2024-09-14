package webhook

import (
	"slices"
	"time"
)

const (
	webhookRateLimitPeriod   = 60 * time.Second
	webhookRateLimitRequests = 30
)

type clock interface {
	Now() time.Time
}

// webhookRateLimit allows handling of the undocumented webhook rate limit
type webhookRateLimit struct {
	s     []time.Time
	clock clock
}

func newWebhookRateLimit(clock clock) webhookRateLimit {
	ts := webhookRateLimit{clock: clock}
	ts.s = make([]time.Time, 0)
	return ts
}

// recordRequest records the time of a request
func (rl *webhookRateLimit) recordRequest() {
	rl.s = append(rl.s, rl.clock.Now())
}

// calc reports how many request are remaining and when the current period will be reset
func (rl *webhookRateLimit) calc() (int, time.Duration) {
	deadline := rl.clock.Now().Add(-webhookRateLimitPeriod)
	s2 := make([]time.Time, 0)
	for _, v := range rl.s {
		if v.After(deadline) {
			s2 = append(s2, v)
		}
	}
	rl.s = s2
	remaining := max(0, webhookRateLimitRequests-len(rl.s))
	var reset time.Duration
	if len(rl.s) == 0 {
		reset = 0
	} else {
		oldest := slices.MinFunc(rl.s, func(a time.Time, b time.Time) int {
			return a.Compare(b)
		})
		reset = roundUpDuration(time.Until(oldest.Add(webhookRateLimitPeriod)), time.Second)
	}
	return remaining, reset
}

func roundUpDuration(d time.Duration, m time.Duration) time.Duration {
	x := d.Round(m)
	if x < d {
		return x + m
	}
	return x
}
