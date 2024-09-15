package discordhook

import (
	"fmt"
	"time"
)

type breachedRateLimit struct {
	resetAt time.Time
}

func (brl breachedRateLimit) String() string {
	return fmt.Sprintf("resetAt: %v", brl.resetAt)
}

func (brl *breachedRateLimit) retryAfter() time.Duration {
	if brl.resetAt.IsZero() {
		return 0
	}
	d := time.Until(brl.resetAt)
	if d < 0 {
		brl.resetAt = time.Time{}
	}
	return d
}
