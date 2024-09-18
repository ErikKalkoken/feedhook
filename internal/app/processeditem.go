package app

import (
	"bytes"
	"encoding/gob"
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

func (si *ProcessedItem) Key() []byte {
	return []byte(si.ID)
}

func (si *ProcessedItem) ToBytes() ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(*si); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func NewProcessedItemFromBytes(byt []byte) (*ProcessedItem, error) {
	b := bytes.NewBuffer(byt)
	d := gob.NewDecoder(b)
	var pi ProcessedItem
	if err := d.Decode(&pi); err != nil {
		return &pi, err
	}
	return &pi, nil
}
