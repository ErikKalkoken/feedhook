// Package remote contains the logic for communicating between cli and server process.
package remote

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/consoletable"
)

const (
	// DefaultPort is the default port of the RPC service.
	DefaultPort = 2233
)

// EmptyArgs are the args for RPC methods without parameters.
type EmptyArgs struct{}

// SendPingArgs are the args for SendPing.
type SendPingArgs struct {
	WebhookName string
}

// SendLatestArgs are the args for PostLatestFeedItem.
type SendLatestArgs struct {
	FeedName string
}

// RemoteService is a service for providing remote access to the app via RPC.
type RemoteService struct {
	cfg config.Config
	d   *dispatcher.Dispatcher
	st  *storage.Storage
}

// NewRemoteService returns a new RemoteService.
func NewRemoteService(d *dispatcher.Dispatcher, st *storage.Storage, cfg config.Config) *RemoteService {
	x := &RemoteService{
		cfg: cfg,
		d:   d,
		st:  st,
	}
	return x
}

// PostLatestFeedItem posts the latest item of a feed to its webhook.
func (s *RemoteService) PostLatestFeedItem(args *SendLatestArgs, reply *bool) error {
	return s.d.PostLatestFeedItem(args.FeedName)
}

// Restart restarts the dispatcher.
func (s *RemoteService) Restart(args *EmptyArgs, reply *bool) error {
	return s.d.Restart()
}

// Statistics returns feed and webhook statistics as text tables.
func (s *RemoteService) Statistics(args *EmptyArgs, reply *string) error {
	out := &strings.Builder{}
	// Feed stats
	feedsTable := consoletable.New("Feeds", 6)
	feedsTable.Target = out
	feedsTable.AddRow([]any{"Name", "Enabled", "Webhook", "Received", "Last", "Errors"})
	feeds := slices.SortedFunc(slices.Values(s.cfg.Feeds), func(a, b config.ConfigFeed) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cf := range feeds {
		o, err := s.st.GetFeedStats(cf.Name)
		if errors.Is(err, storage.ErrNotFound) {
			o = &app.FeedStats{}
		} else if err != nil {
			return err
		}
		feedsTable.AddRow([]any{cf.Name, !cf.Disabled, cf.Webhook, o.ReceivedCount, o.ReceivedLast, o.ErrorCount})
	}
	feedsTable.Print()
	fmt.Fprintln(out)
	// Webhook stats
	whTable := consoletable.New("Webhooks", 5)
	whTable.Target = out
	whTable.AddRow([]any{"Name", "Queued", "Sent", "Last", "Errors"})
	webhooks := slices.SortedFunc(slices.Values(s.cfg.Webhooks), func(a, b config.ConfigWebhook) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cw := range webhooks {
		o, err := s.st.GetWebhookStats(cw.Name)
		if errors.Is(err, storage.ErrNotFound) {
			o = &app.WebhookStats{}
		} else if err != nil {
			return err
		}
		ms, err := s.d.MessengerStatus(cw.Name)
		if err != nil {
			slog.Error("Failed to fetch queue size for webhook", "webhook", cw.Name)
		}
		whTable.AddRow([]any{cw.Name, ms.QueueSize, o.SentCount, o.SentLast, ms.ErrorCount})
	}
	whTable.Print()
	*reply = out.String()
	return nil
}

// SendPing sends a test message to a webhook.
func (s *RemoteService) SendPing(args *SendPingArgs, reply *bool) error {
	return s.d.SendPing(args.WebhookName)
}
