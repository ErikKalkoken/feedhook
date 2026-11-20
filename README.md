# feedhook

A self-hosted service that posts new items from RSS and Atom feeds to Discord webhooks.

![GitHub Release](https://img.shields.io/github/v/release/ErikKalkoken/feedhook)
[![CI/CD](https://github.com/ErikKalkoken/feedhook/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/feedhook/actions/workflows/go.yml)
[![codecov](https://codecov.io/github/ErikKalkoken/feedhook/graph/badge.svg?token=jnRQ5oJ0AP)](https://codecov.io/github/ErikKalkoken/feedhook)
![GitHub License](https://img.shields.io/github/license/ErikKalkoken/feedhook)

## Content

- [Key Features](#key-features)
- [Example](#example)
- [Quick start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [Update](#update)
- [CLI tool](#cli-tool)
- [Attributions](#attributions)

## Key Features

- Posts new items from RSS and Atom feeds to one or more Discord webhooks
- Respects Discord rate limits
- Built for high throughput
- Easy configuration
- Restartable without data loss
- Live statistics

## Example

Here is how a forwarded RSS item looks on Discord:

![Example of a forwarded feed item on Discord](https://github.com/user-attachments/assets/f12b3b7c-6374-49b0-bd41-b9dc1f1a15ba)

## Quick start

This section shows how to try out **feedhook** on your machine. To run it permanently on a server, see [Installation](#installation).

Download and decompress the latest release from the [releases page](https://github.com/ErikKalkoken/feedhook/releases) and download the example configuration:

```sh
wget https://github.com/ErikKalkoken/feedhook/releases/download/vX.Y.Z/feedhook-X.Y.Z-linux-amd64.tar.gz
tar -xvzf feedhook-X.Y.Z-linux-amd64.tar.gz
wget https://raw.githubusercontent.com/ErikKalkoken/feedhook/main/config/config.toml
```

Open `config.toml` and replace the example webhook URL with the URL of your Discord webhook.

Start the service:

```sh
./feedhooksrv
```

Feedhook now polls the feed and posts new items to your webhook. On the first start this includes all items published in the last 2 hours. The database file `feedhook.db` is created in the current directory.

In a second terminal, send a test message to your webhook and see the statistics:

```sh
./feedhookcli ping Hook-1
./feedhookcli stats
```

Press `Ctrl+C` to stop the service.

## Installation

This section explains how to install **feedhook** as a service on a Unix-like server.

> [!NOTE]
> This guide uses [supervisor](http://supervisord.org/index.html) for running feedhook as a service. Please make sure it is installed on your system before continuing.

Create a "service" user with disabled login:

```sh
sudo adduser --disabled-login feedhook
```

Switch to the service user and move to the home directory:

```sh
sudo su feedhook
cd ~
```

Download and decompress the latest release from the [releases page](https://github.com/ErikKalkoken/feedhook/releases):

```sh
wget https://github.com/ErikKalkoken/feedhook/releases/download/vX.Y.Z/feedhook-X.Y.Z-linux-amd64.tar.gz
tar -xvzf feedhook-X.Y.Z-linux-amd64.tar.gz
```

> [!TIP]
> Please make sure to update the URL and filename to the latest version.

Download configuration files:

```sh
wget https://raw.githubusercontent.com/ErikKalkoken/feedhook/main/config/supervisor.conf
wget https://raw.githubusercontent.com/ErikKalkoken/feedhook/main/config/config.toml
```

Setup and configure:

```sh
chmod +x feedhooksrv feedhookcli
touch feedhook.log
```

Setup your initial feeds and webhooks by adding them to `config.toml`.

Switch back to your sudo user:

```sh
exit
```

Add feedhook to supervisor:

```sh
sudo ln -s /home/feedhook/supervisor.conf /etc/supervisor/conf.d/feedhook.conf
sudo systemctl restart supervisor
```

Start feedhook:

```sh
sudo supervisorctl restart feedhook
```

We also recommend running a test ping to ensure the webhooks are setup correctly:

```sh
/home/feedhook/feedhookcli ping WEBHOOK
```

> [!TIP]
> You can monitor your service with the `feedhookcli` tool. For example to get the current statistics you can run: `/home/feedhook/feedhookcli stats`

> [!NOTE]
> Whenever you make changes to the configuration you need to restart the service to activate them.

## Configuration

Feedhook is configured with the file `config.toml`. See [config/config.toml](config/config.toml) for an example.

### App

The optional `[app]` section contains general settings:

| Key | Default | Description |
| --- | --- | --- |
| `timeout` | `30` | Timeout in seconds for fetching feeds and posting to webhooks |
| `oldest` | `7200` | Items published longer ago than this many seconds are not forwarded. Set to `-1` to forward items of any age. |
| `ticker` | `30` | Interval in seconds for polling feeds |
| `loglevel` | `"INFO"` | One of `DEBUG`, `INFO`, `WARN` or `ERROR`. Unknown values fall back to `INFO`. |
| `branding_disabled` | `false` | When `false`, posts appear with the name "Feedhook" and its avatar. When `true`, the name and avatar configured for the webhook on Discord are used. |

### Webhooks

Each `[[webhooks]]` section defines a Discord webhook:

| Key | Description |
| --- | --- |
| `name` | Name of the webhook. Must be unique. |
| `url` | URL of the webhook. Must be unique. |

### Feeds

Each `[[feeds]]` section defines a RSS or Atom feed:

| Key | Description |
| --- | --- |
| `name` | Name of the feed. Must be unique. |
| `url` | URL of the feed |
| `webhooks` | Names of the webhooks this feed is forwarded to |
| `disabled` | Set to `true` to stop polling this feed. Default is `false`. |

Items without content and description are skipped. Items that change after being forwarded are forwarded again.

### Server options

`feedhooksrv` accepts these command line options:

| Option | Default | Description |
| --- | --- | --- |
| `-config` | `.` | Directory containing `config.toml` |
| `-db` | `.` | Directory where the database file `feedhook.db` is stored |
| `-port` | `2233` | Port of the RPC service used by `feedhookcli` |
| `-offline` | | Run only the RPC service, without fetching feeds or posting messages |
| `-v` | | Show the version and exit |

The RPC service only listens on `localhost`.

Example:

```sh
./feedhooksrv -config /etc/feedhook -db /var/lib/feedhook
```

## Update

Stop the feedhook service.

```sh
sudo supervisorctl stop feedhook
```

Login as your service user and move to the home directory:

```sh
sudo su feedhook
cd ~
```

Download the latest release and overwrite the outdated executables:

```sh
wget https://github.com/ErikKalkoken/feedhook/releases/download/vX.Y.Z/feedhook-X.Y.Z-linux-amd64.tar.gz
tar -xvzf feedhook-X.Y.Z-linux-amd64.tar.gz
```

Switch back to your sudo user and start the feedhook service again.

```sh
exit
sudo supervisorctl start feedhook
```

## CLI tool

Feedhook comes with a CLI tool for interacting with the running service. With it you can:

- Check if the configuration is valid
- See live statistics (e.g. how many items have been received from each feed)
- Make pings to configured webhooks (useful for testing)
- Force a re-send of the latest feed item (useful for testing)

To see all commands please run the tool with the help flag: `feedhookcli -h`.

You can also get help for a specific command with the help flag: `feedhookcli COMMAND -h`.

## Attributions

- [Rss icons created by riajulislam - Flaticon](https://www.flaticon.com/free-icons/rss)
