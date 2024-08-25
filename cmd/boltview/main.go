package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	bolt "go.etcd.io/bbolt"
)

func main() {
	flag.Parse()
	path := flag.Arg(0)
	if path == "" {
		fmt.Println("Must provide path to DB")
		os.Exit(1)
	}
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	if err := db.View(func(tx *bolt.Tx) error {
		tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			fmt.Println(string(name))
			b.ForEachBucket(func(k []byte) error {
				fmt.Println("    " + string(k))
				b2 := b.Bucket(k)
				if b2 != nil {
					b2.ForEach(func(k, v []byte) error {
						fmt.Println("        " + string(k))
						return nil
					})
				}
				return nil
			})
			return nil
		})
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}
