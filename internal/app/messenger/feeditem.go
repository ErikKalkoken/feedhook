package messenger

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

	"github.com/ErikKalkoken/feedhook/internal/dhook"
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
	removeIMGTags := md.Rule{
		Filter: []string{"img"},
		Replacement: func(_ string, _ *goquery.Selection, _ *md.Options) *string {
			return md.String("")
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
	converter.AddRules(removeIMGTags, sanitizeInvalidLinks)
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

// ToDiscordMessage generates a DiscordMessage from a FeedItem.
func (fi FeedItem) ToDiscordMessage(brandingDisabled bool) (dhook.Message, error) {
	var dm dhook.Message
	description, err := converter.ConvertString(fi.Description)
	if err != nil {
		return dm, fmt.Errorf("failed to parse description to markdown: %w", err)
	}
	desc, truncated := truncateString(description, embedDescriptionMaxLength)
	if truncated {
		slog.Warn("description was truncated", "title", fi.Title)
	}
	t := fi.Title
	if fi.IsUpdated {
		t = fmt.Sprintf("UPDATED: %s", t)
	}
	title, truncated := truncateString(t, embedMaxFieldLength)
	if truncated {
		slog.Warn("title was truncated", "title", fi.Title)
	}
	em := dhook.Embed{
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
	if !brandingDisabled {
		dm.Username = username
		dm.AvatarURL = avatarURL
	}
	em.Footer = dhook.EmbedFooter{Text: fi.FeedName}
	dm.Embeds = []dhook.Embed{em}
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
