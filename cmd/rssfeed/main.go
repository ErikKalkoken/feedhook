package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	bolt "go.etcd.io/bbolt"

	"example/rssfeed/internal/app"
)

const (
	configFilename = "config.toml"
	dbFileName     = "rssfeed.db"
)

func main() {
	config := app.ReadConfig(configFilename)
	db, err := bolt.Open(dbFileName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := app.SetupDB(db); err != nil {
		log.Fatal(err)
	}
	app := app.New(db, config)
	go app.Run()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
