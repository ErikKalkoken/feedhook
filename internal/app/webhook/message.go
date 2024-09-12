package webhook

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

// message represents an item that can be queued and contains the payload to be sent and header information.
type message struct {
	Title     string         `json:"title,omitempty"`
	Feed      string         `json:"feed,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
	Payload   webhookPayload `json:"payload,omitempty"`
}

// webhookPayload represents a Discord post for a webhook.
type webhookPayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []embed `json:"embeds,omitempty"`
}

// embed represents a Discord embed.
type embed struct {
	Author struct {
		Name    string `json:"name,omitempty"`
		IconURL string `json:"icon_url,omitempty"`
		URL     string `json:"url,omitempty"`
	} `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	Image       struct {
		URL string `json:"url,omitempty"`
	} `json:"image,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Title     string `json:"title,omitempty"`
	Thumbnail struct {
		URL string `json:"url,omitempty"`
	} `json:"thumbnail,omitempty"`
	URL string `json:"url,omitempty"`
}

// newMessage returns a new message from a feed item.
func newMessage(feedName string, feed *gofeed.Feed, item *gofeed.Item) (message, error) {
	var description string
	var err error
	if item.Content != "" {
		description, err = converter.ConvertString(item.Content)
	} else {
		description, err = converter.ConvertString(item.Description)
	}
	if err != nil {
		return message{}, fmt.Errorf("failed to parse description to markdown: %w", err)
	}
	em := embed{
		Description: description,
		Timestamp:   item.PublishedParsed.Format(time.RFC3339),
		Title:       item.Title,
		URL:         item.Link,
	}
	em.Author.Name = feed.Title
	em.Author.URL = feed.Link
	if feed.Image != nil {
		em.Author.IconURL = feed.Image.URL
	}
	if item.Image != nil {
		em.Image.URL = item.Image.URL
	}
	wpl := webhookPayload{
		Embeds: []embed{em},
	}
	m := message{
		Feed:      feedName,
		Title:     item.Title,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   wpl,
	}
	return m, nil
}

func (m message) toBytes() ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(m); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func newMessageFromBytes(byt []byte) (message, error) {
	b := bytes.NewBuffer(byt)
	d := gob.NewDecoder(b)
	var m message
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	return m, nil
}
