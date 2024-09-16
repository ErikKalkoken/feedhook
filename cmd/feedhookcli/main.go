package main

import (
	"flag"
	"fmt"
	"log"
	"net/rpc"
	"os"

	"github.com/ErikKalkoken/feedhook/internal/remoteservice"
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
	if *statsFlag {
		printStatistics(*portFlag)
		os.Exit(0)
	}
	if *pingFlag != "" {
		sendPing(*portFlag, *pingFlag)
		os.Exit(0)
	}
	flag.Usage()
}

func printStatistics(port int) {
	var reply string
	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatal("dialing:", err)
	}
	args := remoteservice.EmptyArgs{}
	if err = client.Call("RemoteService.Statistics", args, &reply); err != nil {
		log.Fatal(err)
	}
	fmt.Println(reply)
}

func sendPing(port int, name string) {
	var reply bool
	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatal("dialing:", err)
	}
	args := remoteservice.SendPingArgs{Name: name}
	if err = client.Call("RemoteService.SendPing", args, &reply); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Ping sent to %s\n", name)
}
