// Feedhookcli is a CLI tool for feedhook.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/urfave/cli/v2"
)

// Version is overwritten via build tag when released.
var Version = "0.0.0"

func main() {
	app := newApp(remote.NewClient())
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err)
		os.Exit(1)
	}
}

func newApp(client *remote.Client) *cli.App {
	return &cli.App{
		Name:  "feedhookcli",
		Usage: "CLI tool for feedhook",
		Action: func(cCtx *cli.Context) error {
			showHelpOnError(cCtx, cli.ShowAppHelp)
			if cCtx.NArg() > 0 {
				return fmt.Errorf("unknown command: %s", cCtx.Args().First())
			}
			return errors.New("missing command")
		},
		OnUsageError: onAppUsageError,
		Version:      Version,
		Commands: []*cli.Command{
			{
				Name:            "check-config",
				Usage:           "checks whether a config file is valid",
				ArgsUsage:       "path",
				OnUsageError:    onUsageError,
				HideHelpCommand: true,
				Before:          requireOneArg,
				Action: func(cCtx *cli.Context) error {
					path := cCtx.Args().First()
					_, warnings, failures, err := config.FromFile(path)
					for _, w := range warnings {
						fmt.Fprintf(cCtx.App.ErrWriter, "warning: %s\n", w)
					}
					for _, f := range failures {
						fmt.Fprintf(cCtx.App.ErrWriter, "error: %s\n", f)
					}
					if err != nil {
						return err
					}
					fmt.Fprintf(cCtx.App.Writer, "%s is valid\n", path)
					return nil
				},
			},
			{
				Name:            "ping",
				Usage:           "send a test message to a webhook",
				ArgsUsage:       "webhook-name",
				Flags:           makeClientFlags(client),
				OnUsageError:    onUsageError,
				HideHelpCommand: true,
				Before:          requireOneArg,
				Action: func(cCtx *cli.Context) error {
					hookName := cCtx.Args().First()
					if err := client.SendPing(hookName); err != nil {
						return err
					}
					fmt.Fprintf(cCtx.App.Writer, "Ping sent to %s\n", hookName)
					return nil
				},
			},
			{
				Name:            "post-latest",
				Usage:           "posts the latest feed item to its configured webhook",
				ArgsUsage:       "feed-name",
				Flags:           makeClientFlags(client),
				OnUsageError:    onUsageError,
				HideHelpCommand: true,
				Before:          requireOneArg,
				Action: func(cCtx *cli.Context) error {
					feedName := cCtx.Args().First()
					if err := client.PostLatestFeedItem(feedName); err != nil {
						return err
					}
					fmt.Fprintf(cCtx.App.Writer, "Posted latest item from \"%s\"\n", feedName)
					return nil
				},
			},
			{
				Name:            "restart",
				Usage:           "restarts the service",
				Flags:           makeClientFlags(client),
				OnUsageError:    onUsageError,
				HideHelpCommand: true,
				Before:          requireNoArgs,
				Action: func(cCtx *cli.Context) error {
					if err := client.Restart(); err != nil {
						return err
					}
					fmt.Fprintln(cCtx.App.Writer, "Restarted")
					return nil
				},
			},
			{
				Name:            "stats",
				Usage:           "show current statistics",
				Flags:           makeClientFlags(client),
				OnUsageError:    onUsageError,
				HideHelpCommand: true,
				Before:          requireNoArgs,
				Action: func(cCtx *cli.Context) error {
					text, err := client.Statistics()
					if err != nil {
						return err
					}
					fmt.Fprintln(cCtx.App.Writer, text)
					return nil
				},
			},
		},
	}
}

func makeClientFlags(client *remote.Client) []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Usage:   "port where the RPC service of feedhooksrv is running",
			Value:   remote.DefaultPort,
			Action: func(cCtx *cli.Context, v int) error {
				return client.SetPort(v)
			},
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Usage:   "max time to wait for feedhooksrv to respond",
			Value:   remote.DefaultTimeout,
			Action: func(cCtx *cli.Context, v time.Duration) error {
				return client.SetTimeout(v)
			},
		},
	}
}

func requireOneArg(cCtx *cli.Context) error {
	if cCtx.NArg() == 1 {
		return nil
	}
	showHelpOnError(cCtx, cli.ShowSubcommandHelp)
	if cCtx.NArg() == 0 {
		return fmt.Errorf("expected one argument: %s", cCtx.Command.ArgsUsage)
	}
	return fmt.Errorf("expected one argument: %s, got: %s", cCtx.Command.ArgsUsage, strings.Join(cCtx.Args().Slice(), " "))
}

func requireNoArgs(cCtx *cli.Context) error {
	if cCtx.NArg() == 0 {
		return nil
	}
	showHelpOnError(cCtx, cli.ShowSubcommandHelp)
	return fmt.Errorf("expected no arguments, got: %s", strings.Join(cCtx.Args().Slice(), " "))
}

// onUsageError shows help on the error output and returns the flag error.
func onUsageError(cCtx *cli.Context, err error, _ bool) error {
	showHelpOnError(cCtx, cli.ShowSubcommandHelp)
	return err
}

func onAppUsageError(cCtx *cli.Context, err error, _ bool) error {
	showHelpOnError(cCtx, cli.ShowAppHelp)
	return err
}

// showHelpOnError shows help on the error output, followed by an empty line.
func showHelpOnError(cCtx *cli.Context, show func(*cli.Context) error) {
	w := cCtx.App.Writer
	cCtx.App.Writer = cCtx.App.ErrWriter
	defer func() { cCtx.App.Writer = w }()
	_ = show(cCtx)
	fmt.Fprintln(cCtx.App.ErrWriter)
}
