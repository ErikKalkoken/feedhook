package main

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/service"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/consoletable"
	"github.com/ErikKalkoken/feedhook/internal/discordhook"
)

const (
	configFilename  = "config.toml"
	dbFileName      = "feedhook.db"
	boltOpenTimeout = 5 * time.Second
)

// Overwritten with current tag when released
var Version = "0.2.0"

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func main() {
	cfgPathFlag := flag.String("config", ".", "path to configuration file")
	dbPathFlag := flag.String("db", ".", "path to database file")
	statsFlag := flag.Bool("statistics", false, "show current statistics (does not work while running)")
	versionFlag := flag.Bool("v", false, "show version")
	pingFlag := flag.String("ping", "", "send ping to a configured webhook")
	flag.Usage = myUsage
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}
	p := filepath.Join(*cfgPathFlag, configFilename)
	cfg, err := app.ReadConfig(p)
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	slog.SetLogLoggerLevel(cfg.App.LoggerLevel())
	if *pingFlag != "" {
		if err := sendPing(*pingFlag, cfg); err != nil {
			slog.Error("Failed to send ping", "webhook", *pingFlag, "error", err)
			os.Exit(1)
		}
		os.Exit(1)
	}
	p = filepath.Join(*dbPathFlag, dbFileName)
	db, err := bolt.Open(p, 0600, &bolt.Options{Timeout: boltOpenTimeout})
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		log.Fatalf("DB init failed: %s", err)
	}
	if *statsFlag {
		printStatistics(st, cfg)
		os.Exit(0)
	}
	a := service.NewService(st, cfg, realtime{})
	a.Start()
	defer a.Close()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func printStatistics(st *storage.Storage, cfg app.MyConfig) {
	// Feed stats
	feedsTable := consoletable.New("Feeds", 6)
	feedsTable.AddRow([]any{"Name", "Received", "Last", "Errors", "Enabled", "Webhooks"})
	slices.SortFunc(cfg.Feeds, func(a, b app.ConfigFeed) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cf := range cfg.Feeds {
		o, err := st.GetFeedStats(cf.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		feedsTable.AddRow([]any{o.Name, o.ReceivedCount, o.ReceivedLast, o.ErrorCount, !cf.Disabled, cf.Webhooks})
	}
	feedsTable.Print()
	fmt.Println()
	// Webhook stats
	whTable := consoletable.New("Webhooks", 4)
	whTable.AddRow([]any{"Name", "Count", "Last", "Errors"})
	slices.SortFunc(cfg.Webhooks, func(a, b app.ConfigWebhook) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, cw := range cfg.Webhooks {
		o, err := st.GetWebhookStats(cw.Name)
		if err == storage.ErrNotFound {
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		whTable.AddRow([]any{o.Name, o.SentCount, o.SentLast, o.ErrorCount})
	}
	whTable.Print()
}

// myUsage writes a custom usage message to configured output stream.
func myUsage() {
	s := "Usage: feedhook [options]:\n\n" +
		"A service for forwarding RSS and Atom feeds to Discord webhooks.\n" +
		"For more information please see: https://github.com/ErikKalkoken/feedhook\n\n" +
		"Options:\n"
	fmt.Fprint(flag.CommandLine.Output(), s)
	flag.PrintDefaults()
}

func sendPing(name string, cfg app.MyConfig) error {
	var wh app.ConfigWebhook
	for _, w := range cfg.Webhooks {
		if w.Name == name {
			wh = w
			break
		}
	}
	if wh.Name == "" {
		return fmt.Errorf("no webhook found with the name %s", name)
	}
	dh := discordhook.New(http.DefaultClient, wh.URL)
	pl := discordhook.WebhookPayload{Content: "Ping from feedhook"}
	return dh.Send(pl)
}
