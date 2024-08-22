package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketFeeds = "feeds"
	oldest      = 24 * time.Hour
)

func main() {
	config := readConfig()
	db, err := bolt.Open("rssfeed.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketFeeds))
		return err
	}); err != nil {
		log.Fatal(err)
	}
	app := NewApp(db, config)
	go app.run()

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
