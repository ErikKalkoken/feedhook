// Feedhookcli is a CLI tool for feedhook.
package main

import (
	"errors"
	"fmt"
	"os"
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
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}

func newApp(client *remote.Client) *cli.App {
	return &cli.App{
		Name:  "feedhookcli",
		Usage: "CLI tool for feedhook",
		Action: func(cCtx *cli.Context) error {
			cli.ShowAppHelp(cCtx)
			fmt.Fprintln(cCtx.App.Writer)
			if cCtx.NArg() > 0 {
				return fmt.Errorf("unknown command: %s", cCtx.Args().First())
			}
			return errors.New("missing command")
		},
		Version: Version,
		Commands: []*cli.Command{
			{
				Name:            "check-config",
				Usage:           "checks whether a config file is valid",
				ArgsUsage:       "path",
				HideHelpCommand: true,
				Before:          requireOneArg,
				Action: func(cCtx *cli.Context) error {
					path := cCtx.Args().First()
					if _, err := config.FromFile(path); err != nil {
						return err
					}
					fmt.Printf("%s is valid\n", path)
					return nil
				},
			},
			{
				Name:            "ping",
				Usage:           "send a test message to a webhook",
				ArgsUsage:       "webhook-name",
				Flags:           makeClientFlags(client),
				HideHelpCommand: true,
				Before:          requireOneArg,
				Action: func(cCtx *cli.Context) error {
					hookName := cCtx.Args().First()
					if err := client.SendPing(hookName); err != nil {
						return err
					}
					fmt.Printf("Ping sent to %s\n", hookName)
					return nil
				},
			},
			{
				Name:            "post-latest",
				Usage:           "posts the latest feed item to its configured webhook",
				ArgsUsage:       "feed-name",
				Flags:           makeClientFlags(client),
				HideHelpCommand: true,
				Before:          requireOneArg,
				Action: func(cCtx *cli.Context) error {
					feedName := cCtx.Args().First()
					if err := client.PostLatestFeedItem(feedName); err != nil {
						return err
					}
					fmt.Printf("Posted latest item from \"%s\"\n", feedName)
					return nil
				},
			},
			{
				Name:            "restart",
				Usage:           "restarts the service",
				Flags:           makeClientFlags(client),
				HideHelpCommand: true,
				Before:          requireNoArgs,
				Action: func(cCtx *cli.Context) error {
					if err := client.Restart(); err != nil {
						return err
					}
					fmt.Println("Restarted")
					return nil
				},
			},
			{
				Name:            "stats",
				Usage:           "show current statistics",
				Flags:           makeClientFlags(client),
				HideHelpCommand: true,
				Before:          requireNoArgs,
				Action: func(cCtx *cli.Context) error {
					text, err := client.Statistics()
					if err != nil {
						return err
					}
					fmt.Println(text)
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
	_ = cli.ShowSubcommandHelp(cCtx)
	fmt.Fprintln(cCtx.App.Writer)
	if cCtx.NArg() == 0 {
		return fmt.Errorf("expected one argument: %s", cCtx.Command.ArgsUsage)
	}
	return fmt.Errorf("expected one argument: %s (options must come before arguments)", cCtx.Command.ArgsUsage)
}

func requireNoArgs(cCtx *cli.Context) error {
	if cCtx.NArg() == 0 {
		return nil
	}
	_ = cli.ShowSubcommandHelp(cCtx)
	fmt.Fprintln(cCtx.App.Writer)
	return errors.New("expected no arguments (options must come before arguments)")
}
