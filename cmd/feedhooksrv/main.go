package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

const (
	configFilename  = "config.toml"
	dbFileName      = "feedhook.db"
	boltOpenTimeout = 5 * time.Second
	portRPC         = 2233
)

// Overwritten with current tag when released
var Version = "0.2.1"

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func main() {
	cfgPathFlag := flag.String("config", ".", "path to configuration file")
	dbPathFlag := flag.String("db", ".", "path to database file")
	portFlag := flag.Int("port", portRPC, "port for RPC service")
	versionFlag := flag.Bool("v", false, "show version")
	offlineFlag := flag.Bool("offline", false, "run RPC service only")
	flag.Usage = myUsage
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}
	configPath := filepath.Join(*cfgPathFlag, configFilename)
	cfg, err := config.FromFile(configPath)
	if err != nil {
		slog.Error("Invalid config", "error", err)
		os.Exit(1)
	}
	slog.SetLogLoggerLevel(cfg.App.LoggerLevel())
	p := filepath.Join(*dbPathFlag, dbFileName)
	db, err := bolt.Open(p, 0600, &bolt.Options{Timeout: boltOpenTimeout})
	if err != nil {
		slog.Error("Failed to open DB", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		slog.Error("DB init failed", "error", err)
		os.Exit(1)
	}

	// start the dispatcher
	d := dispatcher.New(st, cfg, realtime{})
	if !*offlineFlag {
		if err := d.Start(); err != nil {
			slog.Error("Failed to start dispatcher", "error", err)
			os.Exit(1)
		}
		defer d.Close()
	}

	// start RPC service
	if err := startRPC(*portFlag, d, st, cfg, configPath); err != nil {
		slog.Error("Failed to start RPC service", "port", portRPC, "error", err)
		os.Exit(1)
	}

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func startRPC(port int, d *dispatcher.Dispatcher, st *storage.Storage, cfg config.Config, configPath string) error {
	rpc.Register(remote.NewRemoteService(d, st, cfg, configPath))
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return err
	}
	go func() {
		slog.Info("RPC service running", "port", port)
		err := http.Serve(l, nil)
		slog.Error("RPC service aborted", "error", err)
	}()
	return nil
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
