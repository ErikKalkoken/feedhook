package app

import "time"

type FeedStats struct {
	Name          string
	ErrorCount    int // failed feed runs, e.g. fetch errors
	ReceivedCount int
	ReceivedLast  time.Time
}

type WebhookStats struct {
	Name      string
	SentCount int
	SentLast  time.Time
}
