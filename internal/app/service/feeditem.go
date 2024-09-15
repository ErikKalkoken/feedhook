package service

import (
	"fmt"
	"log/slog"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

	"github.com/ErikKalkoken/feedforward/internal/discordhook"
)

const (
	// contentMaxLength = 2000
	embedMaxFieldLength       = 256 // title, author name, field names
	embedDescriptionMaxLength = 4096
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

// FeedItem represents a feed item to be posted to a webhook
type FeedItem struct {
	Description string
	FeedName    string
	FeedTitle   string
	FeedURL     string
	IconURL     string
	ImageURL    string
	ItemURL     string
	Published   time.Time
	Title       string
}

func (fi FeedItem) ToDiscordPayload() (discordhook.WebhookPayload, error) {
	var wpl discordhook.WebhookPayload
	description, err := converter.ConvertString(fi.Description)
	if err != nil {
		return wpl, fmt.Errorf("failed to parse description to markdown: %w", err)
	}
	desc, truncated := truncateString(description, embedDescriptionMaxLength)
	if truncated {
		slog.Warn("description was truncated", "description", description)
	}
	title, truncated := truncateString(fi.Title, embedMaxFieldLength)
	if truncated {
		slog.Warn("title was truncated", "title", fi.Title)
	}
	em := discordhook.Embed{
		Description: desc,
		Title:       title,
		URL:         fi.ItemURL,
	}
	if !fi.Published.IsZero() {
		em.Timestamp = fi.Published.Format(time.RFC3339)
	}
	em.Author.Name, truncated = truncateString(fi.FeedTitle, embedMaxFieldLength)
	if truncated {
		slog.Warn("author name was truncated", "FeedTitle", fi.FeedTitle)
	}
	em.Author.URL = fi.FeedURL
	if fi.IconURL != "" {
		em.Author.IconURL = fi.IconURL
	}
	if fi.ImageURL != "" {
		em.Image.URL = fi.ImageURL
	}
	wpl.Embeds = []discordhook.Embed{em}
	return wpl, nil
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
