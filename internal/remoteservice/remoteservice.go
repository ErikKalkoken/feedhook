// Package remoteservice contains the logic for communicating between cli and server process
package remoteservice

import (
	"cmp"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/consoletable"
	"github.com/ErikKalkoken/feedhook/internal/discordhook"
)

type EmptyArgs struct{}

type SendPingArgs struct {
	Name string
}

// RemoteService represents a service, which can be accessed remotely via RPC.
type RemoteService struct {
	cfg    app.MyConfig
	st     *storage.Storage
	client *discordhook.Client
}

func NewRemoteService(st *storage.Storage, cfg app.MyConfig) *RemoteService {
	client := discordhook.NewClient(http.DefaultClient)
	s := &RemoteService{cfg: cfg, st: st, client: client}
	return s
}

func (s *RemoteService) Statistics(args *EmptyArgs, reply *string) error {
	out := &strings.Builder{}
	// Feed stats
	feedsTable := consoletable.New("Feeds", 6)
	feedsTable.Target = out
	feedsTable.AddRow([]any{"Name", "Received", "Last", "Errors", "Enabled", "Webhooks"})
	slices.SortFunc(s.cfg.Feeds, func(a, b app.ConfigFeed) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cf := range s.cfg.Feeds {
		o, err := s.st.GetFeedStats(cf.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		feedsTable.AddRow([]any{o.Name, o.ReceivedCount, o.ReceivedLast, o.ErrorCount, !cf.Disabled, cf.Webhooks})
	}
	feedsTable.Print()
	fmt.Fprintln(out)
	// Webhook stats
	whTable := consoletable.New("Webhooks", 4)
	whTable.Target = out
	whTable.AddRow([]any{"Name", "Count", "Last", "Errors"})
	slices.SortFunc(s.cfg.Webhooks, func(a, b app.ConfigWebhook) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cw := range s.cfg.Webhooks {
		o, err := s.st.GetWebhookStats(cw.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		whTable.AddRow([]any{o.Name, o.SentCount, o.SentLast, o.ErrorCount})
	}
	whTable.Print()
	*reply = out.String()
	return nil
}

func (s *RemoteService) SendPing(args *SendPingArgs, reply *bool) error {
	var wh app.ConfigWebhook
	for _, w := range s.cfg.Webhooks {
		if w.Name == args.Name {
			wh = w
			break
		}
	}
	if wh.Name == "" {
		return fmt.Errorf("no webhook found with the name %s", args.Name)
	}
	dh := discordhook.NewWebhook(s.client, wh.URL)
	pl := discordhook.WebhookPayload{Content: "Ping from feedhook"}
	return dh.Send(pl)
}
