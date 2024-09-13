package webhook

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

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
	Payload   WebhookPayload
}

// WebhookPayload represents a Discord post for a webhook.
type WebhookPayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

// Embed represents a Discord Embed.
type Embed struct {
	Author      EmbedAuthor    `json:"author,omitempty"`
	Description string         `json:"description,omitempty"`
	Image       EmbedImage     `json:"image,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Title       string         `json:"title,omitempty"`
	Thumbnail   EmbedThumbnail `json:"thumbnail,omitempty"`
	URL         string         `json:"url,omitempty"`
}

type EmbedAuthor struct {
	Name    string `json:"name,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
	URL     string `json:"url,omitempty"`
}

type EmbedImage struct {
	URL string `json:"url,omitempty"`
}

type EmbedThumbnail struct {
	URL string `json:"url,omitempty"`
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
	em := Embed{
		Description: truncateString(description, embedDescriptionMaxLength),
		Timestamp:   item.PublishedParsed.Format(time.RFC3339),
		Title:       truncateString(item.Title, embedMaxFieldLength),
		URL:         item.Link,
	}
	em.Author.Name = truncateString(feed.Title, embedMaxFieldLength)
	em.Author.URL = feed.Link
	if feed.Image != nil {
		em.Author.IconURL = feed.Image.URL
	}
	if item.Image != nil {
		em.Image.URL = item.Image.URL
	}
	wpl := WebhookPayload{
		Embeds: []Embed{em},
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
func truncateString(s string, maxLen int) string {
	if maxLen < 3 {
		panic("Length can not be below 3")
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[0:maxLen-3]) + "..."
}
