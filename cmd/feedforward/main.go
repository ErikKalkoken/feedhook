package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedforward/internal/app"
)

const (
	configFilename = "config.toml"
	dbFileName     = "feedforward.db"
)

// Current version need to be injected via ldflags
var Version = "0.0.0"

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func main() {
	cfgPathFlag := flag.String("config", ".", "path to configuration file")
	dbPathFlag := flag.String("db", ".", "path to database file")
	versionFlag := flag.Bool("v", false, "show version")
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
	p = filepath.Join(*dbPathFlag, dbFileName)
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	a := app.New(db, cfg, realtime{})
	if err := a.Init(); err != nil {
		log.Fatalf("Init failed: %s", err)
	}
	a.Start()
	defer a.Close()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

// myUsage writes a custom usage message to configured output stream.
func myUsage() {
	s := "Usage: feedforward [options]:\n\n" +
		"A service for forwarding RSS and Atom feeds to Discord webhooks.\n" +
		"For more information please see: https://github.com/ErikKalkoken/feedforward\n\n" +
		"Options:\n"
	fmt.Fprint(flag.CommandLine.Output(), s)
	flag.PrintDefaults()
}
