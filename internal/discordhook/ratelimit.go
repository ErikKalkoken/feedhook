package discordhook

import (
	"slices"
	"time"
)

const (
	globalRateLimitPeriod    = 1 * time.Second
	globalRateLimitRequests  = 50
	webhookRateLimitPeriod   = 60 * time.Second
	webhookRateLimitRequests = 30
)

type clock interface {
	Now() time.Time
}

// rateLimit keeps track of rate limits.
type rateLimit struct {
	s        []time.Time
	clock    clock
	type_    RateLimitType
	period   time.Duration
	requests int
}

func newRateLimit(type_ RateLimitType, clock clock) rateLimit {
	rl := rateLimit{clock: clock, type_: type_}
	rl.s = make([]time.Time, 0)
	switch type_ {
	case GlobalRateLimit:
		rl.period, rl.requests = globalRateLimitPeriod, globalRateLimitRequests
	case WebhookRateLimit:
		rl.period, rl.requests = webhookRateLimitPeriod, webhookRateLimitRequests
	default:
		panic("not implemented")
	}
	return rl
}

// recordRequest records the time of a request
func (rl *rateLimit) recordRequest() {
	rl.s = append(rl.s, rl.clock.Now())
}

// calc reports how many request are remaining and the duration until the limit will reset
func (rl *rateLimit) calc() (int, time.Duration) {
	deadline := rl.clock.Now().Add(-rl.period)
	s2 := make([]time.Time, 0)
	for _, v := range rl.s {
		if v.After(deadline) {
			s2 = append(s2, v)
		}
	}
	rl.s = s2
	remaining := max(0, rl.requests-len(rl.s))
	var reset time.Duration
	if len(rl.s) == 0 {
		reset = 0
	} else {
		oldest := slices.MinFunc(rl.s, func(a time.Time, b time.Time) int {
			return a.Compare(b)
		})
		reset = roundUpDuration(time.Until(oldest.Add(rl.period)), time.Second)
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
