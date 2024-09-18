package service

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/mmcdole/gofeed"
)

// Message represents a wrapper around a feed item with additional header information for queue processing.
type Message struct {
	Attempt   int
	Item      FeedItem
	Timestamp time.Time
}

// newMessage returns a new message from a feed item.
func newMessage(feedName string, feed *gofeed.Feed, item *gofeed.Item, isUpdated bool) (Message, error) {
	var description string
	if item.Content != "" {
		description = item.Content
	} else {
		description = item.Description
	}
	fi := FeedItem{
		Description: description,
		FeedName:    feedName,
		FeedTitle:   feed.Title,
		FeedURL:     feed.Link,
		IsUpdated:   isUpdated,
		ItemURL:     item.Link,
		Title:       item.Title,
	}
	if item.PublishedParsed != nil {
		fi.Published = *item.PublishedParsed
	}
	if feed.Image != nil {
		fi.IconURL = feed.Image.URL
	}
	if item.Image != nil {
		fi.ImageURL = item.Image.URL
	}
	m := Message{
		Item:      fi,
		Timestamp: time.Now().UTC(),
	}
	return m, nil
}

func newMessageFromBytes(byt []byte) (Message, error) {
	b := bytes.NewBuffer(byt)
	d := gob.NewDecoder(b)
	var m Message
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	return m, nil
}

func (m Message) toBytes() ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(m); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
