package app

import (
	"time"
)

// SentItem represents a sent item
type SentItem struct {
	ID        string
	Published time.Time
}

func (si *SentItem) Key() []byte {
	return []byte(si.ID)
}

func (si *SentItem) Value() []byte {
	return []byte(si.Published.Format(time.RFC3339))
}
