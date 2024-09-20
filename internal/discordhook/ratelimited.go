package discordhook

import (
	"sync"
	"time"
)

// rateLimited holds information wether a client is being rate limited.
// This type is safe to use concurrently.
type rateLimited struct {
	mu      sync.Mutex
	resetAt time.Time
}

// getOrReset reports wether the rate limit is active and also return the duration until reset.
// Or resets the rate limit if it is expired.
func (rl *rateLimited) getOrReset() (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.resetAt.IsZero() {
		return false, 0
	}
	d := time.Until(rl.resetAt)
	if d < 0 {
		rl.resetAt = time.Time{}
		return false, 0
	}
	return true, d
}

func (rl *rateLimited) set(retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.resetAt = time.Now().UTC().Add(retryAfter)
}
