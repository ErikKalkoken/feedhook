package app

import (
	"time"
)

// ProcessedItem represents a sent item
type ProcessedItem struct {
	ID        string
	Published time.Time
}

func (si *ProcessedItem) Key() []byte {
	return []byte(si.ID)
}

func (si *ProcessedItem) Value() []byte {
	return []byte(si.Published.Format(time.RFC3339))
}
