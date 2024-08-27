package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

var converter = md.NewConverter("", true, nil)

func init() {
	x := md.Rule{
		Filter: []string{"img"},
		Replacement: func(_ string, _ *goquery.Selection, _ *md.Options) *string {
			return md.String("")
		},
	}
	converter.AddRules(x)
}

// Message represents a Message send to a webhook.
// Consumers must listen on the errC channel to receive the result.
type Message struct {
	payload *webhookPayload
	errC    chan error
}

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

func makePayload(feed *gofeed.Feed, item *gofeed.Item) (webhookPayload, error) {
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

type Webhook struct {
	client   *http.Client
	messageC chan Message
	url      string
}

func New(client *http.Client, url string) *Webhook {
	wh := &Webhook{
		client:   client,
		messageC: make(chan Message),
		url:      url,
	}
	return wh
}

func (wh *Webhook) Start() {
	go func() {
		for m := range wh.messageC {
			m.errC <- wh.sendToWebhook(m.payload)
		}
	}()
}

func (wh *Webhook) Send(feed *gofeed.Feed, item *gofeed.Item) error {
	p, err := makePayload(feed, item)
	if err != nil {
		return err
	}
	m := Message{payload: &p, errC: make(chan error)}
	wh.messageC <- m
	return <-m.errC
}

func (wh *Webhook) sendToWebhook(payload *webhookPayload) error {
	time.Sleep(1 * time.Second)
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := wh.client.Post(wh.url, "application/json", bytes.NewBuffer(dat))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Debug("response", "url", wh.url, "status", resp.Status, "headers", resp.Header, "body", string(body))
	return nil
}
