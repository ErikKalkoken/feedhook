package webhook

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type webhookPayload struct {
	Content string  `json:"content,omitempty"`
	Embeds  []embed `json:"embeds,omitempty"`
}

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

func newPayload(feed *gofeed.Feed, item *gofeed.Item) (webhookPayload, error) {
	var description string
	var err error
	if item.Content != "" {
		description, err = converter.ConvertString(item.Content)
	} else {
		description, err = converter.ConvertString(item.Description)
	}
	if err != nil {
		return webhookPayload{}, fmt.Errorf("failed to parse description to markdown: %w", err)
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
	payload := webhookPayload{
		Embeds: []embed{em},
	}
	return payload, nil
}

func (wp webhookPayload) ToBytes() ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	if err := e.Encode(wp); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func newPayloadFromBytes(byt []byte) (webhookPayload, error) {
	b := bytes.NewBuffer(byt)
	d := gob.NewDecoder(b)
	var wp webhookPayload
	if err := d.Decode(&wp); err != nil {
		return wp, err
	}
	return wp, nil
}
