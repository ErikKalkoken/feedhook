// Feedhookcli is a CLI tool for interacting with a running service.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/urfave/cli/v2"
)

const (
	portRPC = 2233
)

// Overwritten with current tag when released
var Version = "0.0.0"

func main() {
	var client remote.Client
	app := &cli.App{
		Name:  "feedhookcli",
		Usage: "CLI tool for interacting with a running service",
		Action: func(*cli.Context) error {
			fmt.Println("Command not found")
			return nil
		},
		Version: Version,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "port",
				Usage: "port where the RPC service of feedhooksrv is running",
				Value: portRPC,
			},
		},
		Before: func(ctx *cli.Context) error {
			client = remote.NewClient(ctx.Int("port"))
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "check-config",
				Usage: "checks wether the config is valid",
				Action: func(cCtx *cli.Context) error {
					if err := client.CheckConfig(); err != nil {
						return err
					}
					fmt.Println("Config is valid")
					return nil
				},
			},
			{
				Name:      "ping",
				Usage:     "send a test message to a webhook",
				ArgsUsage: "webhook-name",
				Action: func(cCtx *cli.Context) error {
					hookName := cCtx.Args().First()
					if hookName == "" {
						return errors.New("no webhook specified")
					}
					if err := client.SendPing(hookName); err != nil {
						return err
					}
					fmt.Printf("Ping sent to %s\n", hookName)
					return nil
				},
			},
			{
				Name:      "post-latest",
				Usage:     "posts the latest feed item to configured webhooks",
				ArgsUsage: "feed-name",
				Action: func(cCtx *cli.Context) error {
					feedName := cCtx.Args().First()
					if feedName == "" {
						return errors.New("no feed specified")
					}
					if err := client.PostLatestFeedItem(feedName); err != nil {
						return err
					}
					fmt.Printf("Posted latest item from \"%s\"\n", feedName)
					return nil
				},
			},
			{
				Name:  "restart",
				Usage: "restarts the service",
				Action: func(cCtx *cli.Context) error {
					if err := client.Restart(); err != nil {
						return err
					}
					fmt.Println("Restarted")
					return nil
				},
			},
			{
				Name:  "stats",
				Usage: "show current statistics",
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
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}
