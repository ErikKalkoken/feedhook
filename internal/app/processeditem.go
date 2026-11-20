// Package app is the parent package for the app and types shared across the app.
package app

import (
	"time"
)

type ItemState uint

const (
	StateProcessed ItemState = iota
	StateUpdated
	StateNew
)

// ProcessedItem represents a sent item
type ProcessedItem struct {
	ID        string
	Published time.Time
}

func (pi *ProcessedItem) Key() []byte {
	return []byte(pi.ID)
}
