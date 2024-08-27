package webhook

import (
	"bytes"
	"encoding/json"
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

// message represents a message send to a webhook.
// Consumers must listen on the errC channel to receive the result.
type message struct {
	payload *webhookPayload
	errC    chan error
}

type Webhook struct {
	client   *http.Client
	messageC chan message
	url      string
}

func New(client *http.Client, url string) *Webhook {
	wh := &Webhook{
		client:   client,
		messageC: make(chan message),
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
	m := message{payload: &p, errC: make(chan error)}
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
