package app

import "time"

type FeedStats struct {
	Name      string
	SentCount int
	SentLast  time.Time
}

type WebhookStats struct {
	Name      string
	SentCount int
	SentLast  time.Time
}
