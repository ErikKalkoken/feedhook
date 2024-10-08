package dhooks

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

var ErrInvalidMessage = errors.New("invalid message")

// Discord message limit
const (
	authorNameLength    = 256
	contentLength       = 2000
	descriptionLength   = 4096
	embedCombinedLength = 6000
	embedsQuantity      = 10
	fieldNameLength     = 256
	fieldsQuantity      = 25
	fieldValueLength    = 1024
	footerTextLength    = 2048
	titleLength         = 256
	usernameLength      = 80
)

// Message represents a message that can be send to a Discord webhook.
type Message struct {
	AllowedMentions bool    `json:"allowed_mentions,omitempty"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	Content         string  `json:"content,omitempty"`
	Embeds          []Embed `json:"embeds,omitempty"`
	Username        string  `json:"username,omitempty"`
}

// Validate checks the message against known Discord limits and requirements.
// Messages not passing the validation will usually lead to 400 Bad Request responses from Discord.
// Returns an [ErrInvalidMessage] error in case a limit is violated.
func (m Message) Validate() error {
	if len(m.Content) == 0 && len(m.Embeds) == 0 {
		return fmt.Errorf("need to contain content or embeds: %w", ErrInvalidMessage)
	}
	if length(m.Content) > contentLength {
		return fmt.Errorf("content too long: %w", ErrInvalidMessage)
	}
	if length(m.Username) > usernameLength {
		return fmt.Errorf("username too long: %w", ErrInvalidMessage)
	}
	if len(m.Embeds) > embedsQuantity {
		return fmt.Errorf("too many embeds: %w", ErrInvalidMessage)
	}
	var totalSize int
	for _, em := range m.Embeds {
		if err := em.validate(); err != nil {
			return err
		}
		totalSize += em.size()
	}
	if totalSize > embedCombinedLength {
		return fmt.Errorf("too many characters in combined embeds: %w", ErrInvalidMessage)
	}
	return nil
}

// Embed represents a Discord Embed.
type Embed struct {
	Author      EmbedAuthor    `json:"author,omitempty"`
	Color       int            `json:"color,omitempty"`
	Description string         `json:"description,omitempty"`
	Fields      []EmbedField   `json:"fields,omitempty"`
	Footer      EmbedFooter    `json:"footer,omitempty"`
	Image       EmbedImage     `json:"image,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Title       string         `json:"title,omitempty"`
	Thumbnail   EmbedThumbnail `json:"thumbnail,omitempty"`
	URL         string         `json:"url,omitempty"`
}

func (em Embed) size() int {
	x := length(em.Title) + length(em.Description) + length(em.Author.Name) + length(em.Footer.Text)
	for _, f := range em.Fields {
		x += f.size()
	}
	return x
}

func (em Embed) validate() error {
	em.Author.validate()
	if length(em.Description) > descriptionLength {
		return fmt.Errorf("embed description too long: %w", ErrInvalidMessage)
	}
	em.Footer.validate()
	if len(em.Fields) > fieldsQuantity {
		return fmt.Errorf("embed has too many fields: %w", ErrInvalidMessage)
	}
	for _, f := range em.Fields {
		if err := f.validate(); err != nil {
			return err
		}
	}
	if length(em.Title) > titleLength {
		return fmt.Errorf("embed title too long: %w", ErrInvalidMessage)
	}
	if em.Timestamp != "" {
		_, err := time.Parse(time.RFC3339, em.Timestamp)
		if err != nil {
			return fmt.Errorf("embed timestamp does not conform to RFC3339: %w", ErrInvalidMessage)
		}
	}
	if err := em.Author.validate(); err != nil {
		return err
	}
	if err := em.Footer.validate(); err != nil {
		return err
	}
	if err := em.Image.validate(); err != nil {
		return err
	}
	if err := em.Thumbnail.validate(); err != nil {
		return err
	}
	return nil
}

type EmbedAuthor struct {
	Name    string `json:"name,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
	URL     string `json:"url,omitempty"`
}

func (ea EmbedAuthor) validate() error {
	if length(ea.Name) > authorNameLength {
		return fmt.Errorf("embed author name too long: %w", ErrInvalidMessage)
	}
	if ea.IconURL != "" && !isValidPublicURL(ea.IconURL) {
		return fmt.Errorf("embed author icon URL not valid: %w", ErrInvalidMessage)
	}
	if ea.URL != "" && !isValidPublicURL(ea.URL) {
		return fmt.Errorf("embed author URL not valid: %w", ErrInvalidMessage)
	}
	return nil
}

type EmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

func (ef EmbedField) size() int {
	return length(ef.Name) + length(ef.Value)
}

func (ef EmbedField) validate() error {
	if length(ef.Name) > fieldNameLength {
		return fmt.Errorf("embed field name too long: %w", ErrInvalidMessage)
	}
	if length(ef.Value) > fieldNameLength {
		return fmt.Errorf("embed field value too long: %w", ErrInvalidMessage)
	}
	return nil
}

type EmbedFooter struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

func (ef EmbedFooter) validate() error {
	if length(ef.Text) > footerTextLength {
		return fmt.Errorf("embed footer text too long: %w", ErrInvalidMessage)
	}
	if ef.IconURL != "" && !isValidPublicURL(ef.IconURL) {
		return fmt.Errorf("footer icon URL not valid: %w", ErrInvalidMessage)
	}
	return nil
}

type EmbedImage struct {
	URL string `json:"url,omitempty"`
}

func (ei EmbedImage) validate() error {
	if ei.URL != "" && !isValidPublicURL(ei.URL) {
		return fmt.Errorf("embed image URL not valid: %w", ErrInvalidMessage)
	}
	return nil
}

type EmbedThumbnail struct {
	URL string `json:"url,omitempty"`
}

func (et EmbedThumbnail) validate() error {
	if et.URL != "" && !isValidPublicURL(et.URL) {
		return fmt.Errorf("embed thumbnail URL not valid: %w", ErrInvalidMessage)
	}
	return nil
}

// length returns the number of runes in a string.
func length(s string) int {
	return len([]rune(s))
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
