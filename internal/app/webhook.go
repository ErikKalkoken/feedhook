package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mmcdole/gofeed"
)

var converter = md.NewConverter("", true, nil)

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
	content, err := converter.ConvertString(item.Content)
	if err != nil {
		return webhookPayload{}, fmt.Errorf("failed to parse content to markdown: %w", err)
	}
	description, err := converter.ConvertString(item.Description)
	if err != nil {
		return webhookPayload{}, fmt.Errorf("failed to parse description to markdown: %w", err)
	}
	if content != "" {
		description = content
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

func sendToWebhook(payload *webhookPayload, url string, timeout time.Duration) error {
	time.Sleep(1 * time.Second)
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(dat))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	slog.Debug("response", "url", url, "status", resp.Status, "headers", resp.Header, "body", string(body))
	return nil
}
