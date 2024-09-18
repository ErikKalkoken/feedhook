package discordhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
	return rt.now
}

func TestRateLimit(t *testing.T) {
	now := time.Now()
	t.Run("should report false when no requests recorded", func(t *testing.T) {
		rl := newRateLimit(WebhookRateLimit, faketime{})
		remaining, _ := rl.calc()
		assert.Equal(t, 30, remaining)
	})
	t.Run("should report false when below limit", func(t *testing.T) {
		rl := newRateLimit(WebhookRateLimit, faketime{now})
		rl.s = []time.Time{now, now, now}
		remaining, _ := rl.calc()
		assert.Equal(t, 27, remaining)
	})
	t.Run("should report true when above limit", func(t *testing.T) {
		rl := newRateLimit(WebhookRateLimit, faketime{now})
		oldest := now.Add(-30 * time.Second)
		rl.s = append(rl.s, oldest)
		rl.s = append(rl.s, now.Add(-61*time.Second))
		for range 29 {
			rl.s = append(rl.s, now)
		}
		remaining, reset := rl.calc()
		assert.Equal(t, 0, remaining)
		assert.WithinDuration(t, oldest.Add(60*time.Second), now.Add(reset), 1*time.Second)
	})
}

func TestRoundOn(t *testing.T) {
	cases := []struct {
		in   time.Duration
		m    time.Duration
		want time.Duration
	}{
		{1*time.Second + 100*time.Millisecond, 1 * time.Second, 2 * time.Second},
		{1*time.Second + 900*time.Millisecond, 1 * time.Second, 2 * time.Second},
		{2 * time.Second, 1 * time.Second, 2 * time.Second},
		{1*time.Minute + 10*time.Second, 1 * time.Minute, 2 * time.Minute},
	}
	for _, tc := range cases {
		got := roundUpDuration(tc.in, tc.m)
		assert.Equal(t, tc.want, got)
	}
}
