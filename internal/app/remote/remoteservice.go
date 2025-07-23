// Package remote contains the logic for communicating between cli and server process.
package remote

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/consoletable"
	"github.com/ErikKalkoken/go-dhook"
)

type EmptyArgs struct{}

type SendPingArgs struct {
	WebhookName string
}

type SendLatestArgs struct {
	FeedName string
}

// RemoteService is a service for providing remote access to the app via RPC.
type RemoteService struct {
	cfg        config.Config
	configPath string
	client     *dhook.Client
	d          *dispatcher.Dispatcher
	st         *storage.Storage
}

func NewRemoteService(d *dispatcher.Dispatcher, st *storage.Storage, cfg config.Config, configPath string) *RemoteService {
	x := &RemoteService{
		cfg:        cfg,
		client:     dhook.NewClient(),
		d:          d,
		st:         st,
		configPath: configPath,
	}
	return x
}

func (s *RemoteService) CheckConfig(args *EmptyArgs, reply *bool) error {
	_, err := config.FromFile(s.configPath)
	return err
}

func (s *RemoteService) PostLatestFeedItem(args *SendLatestArgs, reply *bool) error {
	return s.d.PostLatestFeedItem(args.FeedName)
}

func (s *RemoteService) Restart(args *EmptyArgs, reply *bool) error {
	return s.d.Restart()
}

func (s *RemoteService) Statistics(args *EmptyArgs, reply *string) error {
	out := &strings.Builder{}
	// Feed stats
	feedsTable := consoletable.New("Feeds", 6)
	feedsTable.Target = out
	feedsTable.AddRow([]any{"Name", "Enabled", "Webhooks", "Received", "Last", "Errors"})
	slices.SortFunc(s.cfg.Feeds, func(a, b config.ConfigFeed) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cf := range s.cfg.Feeds {
		o, err := s.st.GetFeedStats(cf.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			return err
		}
		feedsTable.AddRow([]any{o.Name, !cf.Disabled, cf.Webhooks, o.ReceivedCount, o.ReceivedLast, o.ErrorCount})
	}
	feedsTable.Print()
	fmt.Fprintln(out)
	// Webhook stats
	whTable := consoletable.New("Webhooks", 5)
	whTable.Target = out
	whTable.AddRow([]any{"Name", "Queued", "Sent", "Last", "Errors"})
	slices.SortFunc(s.cfg.Webhooks, func(a, b config.ConfigWebhook) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cw := range s.cfg.Webhooks {
		o, err := s.st.GetWebhookStats(cw.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			return err
		}
		ms, err := s.d.MessengerStatus(cw.Name)
		if err != nil {
			slog.Error("Failed to fetch queue size for webhook", "webhook", cw.Name)
		}
		whTable.AddRow([]any{o.Name, ms.QueueSize, o.SentCount, o.SentLast, ms.ErrorCount})
	}
	whTable.Print()
	*reply = out.String()
	return nil
}

func (s *RemoteService) SendPing(args *SendPingArgs, reply *bool) error {
	var wh config.ConfigWebhook
	for _, w := range s.cfg.Webhooks {
		if w.Name == args.WebhookName {
			wh = w
			break
		}
	}
	if wh.Name == "" {
		return fmt.Errorf("no webhook found with the name %s", args.WebhookName)
	}
	dh := s.client.NewWebhook(wh.URL)
	_, err := dh.Execute(dhook.Message{Content: "Ping from feedhook"}, nil)
	return err
}
