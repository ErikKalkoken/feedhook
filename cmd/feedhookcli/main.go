// Feedhookcli is a CLI tool for feedhook.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/remote"
)

// Version is overwritten via build tag when released.
var Version = "0.0.0"

// command describes a CLI command.
type command struct {
	name        string
	argsUsage   string // empty when the command takes no args
	desc        string
	nArgs       int  // number of positional args: 0 or 1
	clientFlags bool // whether the command talks to feedhooksrv
	run         func(client *remote.Client, args []string) error
}

var commands = []command{
	{
		name:      "check-config",
		argsUsage: "path",
		desc:      "checks whether a config file is valid",
		nArgs:     1,
		run: func(_ *remote.Client, args []string) error {
			path := args[0]
			_, warnings, failures, err := config.FromFile(path)
			for _, w := range warnings {
				fmt.Printf("warning: %s\n", w)
			}
			for _, f := range failures {
				fmt.Printf("error: %s\n", f)
			}
			if err != nil {
				return err
			}
			fmt.Printf("%s is valid\n", path)
			return nil
		},
	},
	{
		name:        "ping",
		argsUsage:   "webhook-name",
		desc:        "send a test message to a webhook",
		nArgs:       1,
		clientFlags: true,
		run: func(client *remote.Client, args []string) error {
			hookName := args[0]
			if err := client.SendPing(hookName); err != nil {
				return err
			}
			fmt.Printf("Ping sent to %s\n", hookName)
			return nil
		},
	},
	{
		name:        "post-latest",
		argsUsage:   "feed-name",
		desc:        "posts the latest feed item to its configured webhook",
		nArgs:       1,
		clientFlags: true,
		run: func(client *remote.Client, args []string) error {
			feedName := args[0]
			if err := client.PostLatestFeedItem(feedName); err != nil {
				return err
			}
			fmt.Printf("Posted latest item from \"%s\"\n", feedName)
			return nil
		},
	},
	{
		name:        "restart",
		desc:        "restarts the service",
		clientFlags: true,
		run: func(client *remote.Client, _ []string) error {
			if err := client.Restart(); err != nil {
				return err
			}
			fmt.Println("Restarted")
			return nil
		},
	},
	{
		name:        "stats",
		desc:        "show current statistics",
		clientFlags: true,
		run: func(client *remote.Client, _ []string) error {
			text, err := client.Statistics()
			if err != nil {
				return err
			}
			fmt.Println(text)
			return nil
		},
	},
}

func main() {
	if err := run(os.Args[1:], remote.NewClient()); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}

func run(args []string, client *remote.Client) error {
	root := flag.NewFlagSet("feedhookcli", flag.ExitOnError)
	root.Usage = func() {
		w := root.Output()
		fmt.Fprint(w, "Usage: feedhookcli [-version] <command> [options] [args]\n\nCommands:\n")
		for _, c := range commands {
			fmt.Fprintf(w, "  %-14s%s\n", c.name, c.desc)
		}
		fmt.Fprint(w, "\nRun \"feedhookcli <command> -h\" for help on a command.\n\n")
	}
	version := root.Bool("version", false, "show version")
	_ = root.Parse(args) // exits on error
	if *version {
		fmt.Println(Version)
		return nil
	}
	if root.NArg() == 0 {
		root.Usage()
		return errors.New("missing command")
	}
	name, args := root.Arg(0), root.Args()[1:]

	i := slices.IndexFunc(commands, func(c command) bool { return c.name == name })
	if i == -1 {
		root.Usage()
		return fmt.Errorf("unknown command: %s", name)
	}
	cmd := commands[i]

	fs := newFlagSet(cmd)
	var port *int
	var timeout *time.Duration
	if cmd.clientFlags {
		port = fs.Int("port", remote.DefaultPort, "port where the RPC service of feedhooksrv is running")
		timeout = fs.Duration("timeout", remote.DefaultTimeout, "max time to wait for feedhooksrv to respond")
	}
	_ = fs.Parse(args) // exits on error
	if err := checkArgs(fs, cmd); err != nil {
		return err
	}
	if cmd.clientFlags {
		if err := client.SetPort(*port); err != nil {
			return err
		}
		if err := client.SetTimeout(*timeout); err != nil {
			return err
		}
	}
	return cmd.run(client, fs.Args())
}

// newFlagSet returns a flag set for a command with a usage message.
func newFlagSet(cmd command) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd.name, flag.ExitOnError)
	fs.Usage = func() {
		w := fs.Output()
		line := "feedhookcli " + cmd.name
		if cmd.clientFlags {
			line += " [options]"
		}
		if cmd.argsUsage != "" {
			line += " " + cmd.argsUsage
		}
		fmt.Fprintf(w, "Usage: %s\n\n%s\n", line, cmd.desc)
		if cmd.clientFlags {
			fmt.Fprintln(w, "\nOptions:")
			fs.PrintDefaults()
		}
		fmt.Fprintln(w)
	}
	return fs
}

// checkArgs reports an error when the number of positional args is wrong.
func checkArgs(fs *flag.FlagSet, cmd command) error {
	if fs.NArg() == cmd.nArgs {
		return nil
	}
	fs.Usage()
	got := strings.Join(fs.Args(), " ")
	switch {
	case cmd.nArgs == 0:
		return fmt.Errorf("expected no arguments, got: %s", got)
	case fs.NArg() == 0:
		return fmt.Errorf("expected one argument: %s", cmd.argsUsage)
	default:
		return fmt.Errorf("expected one argument: %s, got: %s", cmd.argsUsage, got)
	}
}
