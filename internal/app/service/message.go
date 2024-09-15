package service

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"github.com/ErikKalkoken/feedforward/internal/discordhook"
	"github.com/mmcdole/gofeed"
)

const (
	// contentMaxLength = 2000
	embedMaxFieldLength       = 256 // title, author name, field names
	embedDescriptionMaxLength = 4096
)

// Message represents an item that can be queued and contains the payload to be sent and header information.
type Message struct {
	Title     string
	Feed      string
	Timestamp time.Time
	Attempt   int
	Payload   discordhook.WebhookPayload
}

// newMessage returns a new message from a feed item.
func newMessage(feedName string, feed *gofeed.Feed, item *gofeed.Item) (Message, error) {
	var description string
	var err error
	if item.Content != "" {
		description, err = converter.ConvertString(item.Content)
	} else {
		description, err = converter.ConvertString(item.Description)
	}
	if err != nil {
		return Message{}, fmt.Errorf("failed to parse description to markdown: %w", err)
	}
	desc, truncated := truncateString(description, embedDescriptionMaxLength)
	if truncated {
		slog.Warn("description was truncated", "description", description)
	}
	title, truncated := truncateString(item.Title, embedMaxFieldLength)
	if truncated {
		slog.Warn("title was truncated", "title", title)
	}
	em := discordhook.Embed{
		Description: desc,
		Timestamp:   item.PublishedParsed.Format(time.RFC3339),
		Title:       title,
		URL:         item.Link,
	}
	em.Author.Name, truncated = truncateString(feed.Title, embedMaxFieldLength)
	if truncated {
		slog.Warn("author name was truncated", "feed.Title", feed.Title)
	}
	em.Author.URL = feed.Link
	if feed.Image != nil {
		em.Author.IconURL = feed.Image.URL
	}
	if item.Image != nil {
		em.Image.URL = item.Image.URL
	}
	wpl := discordhook.WebhookPayload{
		Embeds: []discordhook.Embed{em},
	}
	m := Message{
		Feed:      feedName,
		Title:     item.Title,
		Timestamp: time.Now().UTC(),
		Payload:   wpl,
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

func newMessageFromBytes(byt []byte) (Message, error) {
	b := bytes.NewBuffer(byt)
	d := gob.NewDecoder(b)
	var m Message
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	return m, nil
}

// truncateString truncates a given string if it longer then a limit
// and also adds an ellipsis at the end of truncated strings.
// It returns the new string.
func truncateString(s string, maxLen int) (string, bool) {
	if maxLen < 3 {
		panic("Length can not be below 3")
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s, false
	}
	return string(runes[0:maxLen-3]) + "...", true
}
