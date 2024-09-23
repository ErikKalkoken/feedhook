package messenger

import (
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"

	"github.com/ErikKalkoken/feedhook/internal/dhooks"
)

const (
	// contentMaxLength = 2000
	embedMaxFieldLength       = 256 // title, author name, field names
	embedDescriptionMaxLength = 4096
	avatarURL                 = "https://cdn.imgpile.com/f/aQ1yR7t_xl.png"
	username                  = "Feedhook"
)

var converter = md.NewConverter("", true, nil)

func init() {
	removeImgTags := md.Rule{
		Filter: []string{"img"},
		Replacement: func(_ string, _ *goquery.Selection, _ *md.Options) *string {
			return md.String("")
		},
	}
	removeFigureTags := md.Rule{
		Filter: []string{"figure"},
		Replacement: func(_ string, _ *goquery.Selection, _ *md.Options) *string {
			return md.String("")
		},
	}
	sanitizeMailToLinks := md.Rule{
		Filter: []string{"a"},
		Replacement: func(content string, selec *goquery.Selection, options *md.Options) *string {
			href := selec.AttrOr("href", "#")
			if strings.HasPrefix(href, "mailto:") {
				return md.String(content)
			}
			return nil
		},
	}
	sanitizeInvalidLinks := md.Rule{
		Filter: []string{"a"},
		Replacement: func(content string, selec *goquery.Selection, options *md.Options) *string {
			_, err := url.ParseRequestURI(content)
			if err == nil {
				href := selec.AttrOr("href", "#")
				return md.String("[Link](" + href + ")")
			}
			return nil
		},
	}
	converter.AddRules(removeImgTags, removeFigureTags, sanitizeMailToLinks, sanitizeInvalidLinks)
}

// FeedItem represents a feed item to be posted to a webhook
type FeedItem struct {
	Description string
	FeedName    string
	FeedTitle   string
	FeedURL     string
	IconURL     string
	ImageURL    string
	IsUpdated   bool
	ItemURL     string
	Published   time.Time
	Title       string
}

func NewFeedItem(feedName string, feed *gofeed.Feed, item *gofeed.Item, isUpdated bool) FeedItem {
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
	return fi
}

// ToDiscordMessage generates a DiscordMessage from a FeedItem.
func (fi FeedItem) ToDiscordMessage(brandingDisabled bool) (dhooks.Message, error) {
	var dm dhooks.Message
	description, err := converter.ConvertString(fi.Description)
	if err != nil {
		return dm, fmt.Errorf("failed to parse description to markdown: %w", err)
	}
	desc, truncated := truncateString(description, embedDescriptionMaxLength)
	if truncated {
		slog.Warn("description was truncated", "title", fi.Title)
	}
	t := html.UnescapeString(fi.Title)
	if fi.IsUpdated {
		t = fmt.Sprintf("UPDATED: %s", t)
	}
	title, truncated := truncateString(t, embedMaxFieldLength)
	if truncated {
		slog.Warn("title was truncated", "title", fi.Title)
	}
	em := dhooks.Embed{
		Description: desc,
		Title:       title,
	}
	if fi.ItemURL != "" && isValidPublicURL(fi.ItemURL) {
		em.URL = fi.ItemURL
	}
	if !fi.Published.IsZero() {
		em.Timestamp = fi.Published.Format(time.RFC3339)
	}
	ft := html.UnescapeString(fi.FeedTitle)
	em.Author.Name, truncated = truncateString(ft, embedMaxFieldLength)
	if truncated {
		slog.Warn("author name was truncated", "FeedTitle", fi.FeedTitle)
	}
	if fi.FeedURL != "" && isValidPublicURL(fi.FeedURL) {
		em.Author.URL = fi.FeedURL
	}
	if fi.IconURL != "" {
		em.Author.IconURL = fi.IconURL
	}
	if fi.ImageURL != "" && isValidPublicURL(fi.ImageURL) {
		em.Image.URL = fi.ImageURL
	}
	if !brandingDisabled {
		dm.Username = username
		dm.AvatarURL = avatarURL
	}
	em.Footer = dhooks.EmbedFooter{Text: fi.FeedName}
	dm.Embeds = []dhooks.Embed{em}
	return dm, nil
}

// truncateString truncates a given string if it longer then a limit
// and also adds an ellipsis at the end of truncated strings.
// It returns the new string.
func truncateString(s string, maxLen int) (string, bool) {
	if maxLen < 3 {
		panic("max length can not be below 3")
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s, false
	}
	x := string(runes[0 : maxLen-3])
	return x + "...", true
}

// isValidPublicURL reports wether a raw URL is both a public and valid URL.
func isValidPublicURL(rawURL string) bool {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		slog.Warn("error when trying to parse URL", "url", rawURL, "err", err)
		return false
	}
	if u.Scheme == "https" || u.Scheme == "http" {
		return true
	}
	return false
}
