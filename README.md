# feedhook

A service for forwarding RSS and Atom feeds to Discord webhooks.

![GitHub Release](https://img.shields.io/github/v/release/ErikKalkoken/feedhook)
[![CI/CD](https://github.com/ErikKalkoken/feedhook/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/feedhook/actions/workflows/go.yml)
![GitHub License](https://img.shields.io/github/license/ErikKalkoken/feedhook)

## Content

- [Key Features](#key-features)
- [Installation](#installation)
- [Update](#update)
- [CLI tool](#cli-tool)
- [Attributions](#attributions)

## Key Features

- Forward RSS and Atom feeds to webhooks on Discord
- Respects Discord rate limits
- Build for high throughput
- Easy configuration
- Single executable file
- Restartable without data loss
- Live statistics

## Example

Here is how a forwarded RSS item looks on Discord:

![example](https://cdn.imgpile.com/f/s1P9K4y_xl.png)

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
> Please make sure update the URL and filename to the latest version.

Download configuration files:

```sh
wget https://raw.githubusercontent.com/ErikKalkoken/feedhook/main/config/supervisor.conf
wget https://raw.githubusercontent.com/ErikKalkoken/feedhook/main/config/config.toml
```

Setup and configure:

```sh
chmod +x feedhook
touch feedhook.log
```

Setup your initial feeds and webhooks by adding them to `config.toml`.

Then check your config is valid before continuing:

```sh
./feedhookcli check-config
```

We also recommend running a test ping to ensure the webhooks are setup correctly:

```sh
./feedhookcli ping WEBHOOK
```

Add feedhook to supervisor:

```sh
sudo ln -s /home/feedhook/supervisor.conf /etc/supervisor/conf.d/feedhook.conf
sudo systemctl restart supervisor
```

Restart feedhook to start feedhook.

```sh
sudo supervisorctl restart feedhook
```

> [!TIP]
> You can monitor your service with the `feedhookcli` tool. For example to get the current statistics you can run: `./feedhookcli stats`

> [!NOTE]
> Whenever you make changes to the configuration you need to restart the service to activate them.

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

Feedhook comes with a CLI tool. With it you can:

- Check if the configuration is valid
- See live statistics (e.g. how many items have been received from reach feed)
- Make pings to configured webhooks (useful for testing)
- Force a re-send of the latest feed item (useful for testing)

To see all commands please run the tool with the help flag: `feedhookcli -h`.

You can also get help for a specific command with the help flag: `feedhookcli COMMAND -h`.

## Attributions

- [Rss icons created by riajulislam - Flaticon](https://www.flaticon.com/free-icons/rss)
