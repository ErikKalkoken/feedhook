package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"

	"example/feedexpress/internal/app"
)

const (
	configFilename = "config.toml"
	dbFileName     = "feedexpress.db"
)

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func main() {
	cfg, err := app.ReadConfig(configFilename)
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}
	db, err := bolt.Open(dbFileName, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	if err := app.SetupDB(db); err != nil {
		log.Fatalf("Failed to setup DB: %s", err)
	}
	app := app.New(db, cfg, realtime{})
	go app.Run()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
