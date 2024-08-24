package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"

	"example/feedforward/internal/app"
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
	versionFlag := flag.Bool("v", false, "show version")
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}
	cfg, err := app.ReadConfig(configFilename)
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	slog.SetLogLoggerLevel(cfg.App.LoggerLevel())
	db, err := bolt.Open(dbFileName, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	if err := app.SetupDB(db); err != nil {
		log.Fatalf("Failed to setup DB: %s", err)
	}
	app := app.New(db, cfg, realtime{})
	app.Start()
	defer app.Close()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
