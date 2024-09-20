package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ErikKalkoken/feedhook/internal/app/remote"
)

const (
	portRPC = 2233
)

// Overwritten with current tag when released
var Version = "0.0.0"

func main() {
	pingFlag := flag.String("ping", "", "send ping to a configured webhook")
	portFlag := flag.Int("port", portRPC, "port for RPC service")
	statsFlag := flag.Bool("statistics", false, "show current statistics (does not work while running)")
	versionFlag := flag.Bool("v", false, "show version")
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}
	client := remote.NewClient(*portFlag)
	if *statsFlag {
		text, err := client.Statistics()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(text)
		os.Exit(0)
	}
	if *pingFlag != "" {
		if err := client.SendPing(*pingFlag); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Ping sent to %s\n", *pingFlag)
		os.Exit(0)
	}
	flag.Usage()
}
