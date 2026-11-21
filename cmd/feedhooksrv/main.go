// Feedhooksrv is a self-hosted service that posts new items from RSS and Atom feeds to Discord webhooks.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
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
)

// Version is overwritten via build tag when released.
var Version = "0.0.0"

type realtime struct{}

func (rt realtime) Now() time.Time {
	return time.Now()
}

func main() {
	cfgPathFlag := flag.String("config", ".", "directory containing "+configFilename)
	dbPathFlag := flag.String("db", ".", "directory for the database file "+dbFileName)
	portFlag := flag.Int("port", remote.DefaultPort, "port for RPC service")
	versionFlag := flag.Bool("v", false, "show version")
	offlineFlag := flag.Bool("offline", false, "run RPC service only")
	flag.Usage = myUsage
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}
	configPath := filepath.Join(*cfgPathFlag, configFilename)
	cfg, warnings, failures, err := config.FromFile(configPath)
	for _, w := range warnings {
		slog.Warn("Config validation", "warning", w)
	}
	for _, f := range failures {
		slog.Error("Config validation", "failure", f)
	}
	if err != nil {
		slog.Error("Invalid configuration", "error", err)
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
	st := storage.New(db)
	if err := st.Init(cfg.FeedNames()); err != nil {
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
		defer d.Stop()
	}

	// start RPC service
	if err := startRPC(*portFlag, d, st, cfg); err != nil {
		slog.Error("Failed to start RPC service", "port", *portFlag, "error", err)
		os.Exit(1)
	}

	// Ensure graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

func startRPC(port int, d *dispatcher.Dispatcher, st *storage.Storage, cfg config.Config) error {
	server := rpc.NewServer()
	if err := server.Register(remote.NewRemoteService(d, st, cfg)); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	go func() {
		slog.Info("RPC service running", "port", port)
		for {
			conn, err := l.Accept()
			if errors.Is(err, net.ErrClosed) {
				slog.Info("RPC service closed")
				return
			}
			if err != nil {
				slog.Error("RPC service: Failed to accept connection", "error", err)
				time.Sleep(time.Second) // transient error delay
				continue
			}
			go server.ServeConn(conn)
		}
	}()
	return nil
}

// myUsage writes a custom usage message to configured output stream.
func myUsage() {
	s := "Usage: feedhooksrv [options]:\n\n" +
		"A self-hosted service that posts new items from RSS and Atom feeds to Discord webhooks.\n" +
		"For more information please see: https://github.com/ErikKalkoken/feedhook\n\n" +
		"Options:\n"
	fmt.Fprint(flag.CommandLine.Output(), s)
	flag.PrintDefaults()
}
