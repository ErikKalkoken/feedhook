package discordhook

import (
	"fmt"
	"time"
)

type rateLimitExceeded struct {
	resetAt time.Time
}

func (brl *rateLimitExceeded) String() string {
	return fmt.Sprintf("resetAt: %v", brl.resetAt)
}

func (brl *rateLimitExceeded) isActive() (bool, time.Duration) {
	if brl.resetAt.IsZero() {
		return false, 0
	}
	d := time.Until(brl.resetAt)
	if d < 0 {
		brl.resetAt = time.Time{}
		return false, 0
	}
	return true, d
}
