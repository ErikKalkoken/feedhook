# feedhook

A service for forwarding RSS and Atom feeds to Discord webhooks.

![GitHub Release](https://img.shields.io/github/v/release/ErikKalkoken/feedhook)
[![CI/CD](https://github.com/ErikKalkoken/feedhook/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/feedhook/actions/workflows/go.yml)
![GitHub License](https://img.shields.io/github/license/ErikKalkoken/feedhook)

## Key Features

- Forward RSS and Atom feeds to webhooks on Discord
- Respects Discord rate limits
- Build for high throughput
- Easy configuration
- Single executable file
- Restartable without data loss

## Example

Here is how a forwarded RSS item looks on Discord:

![example](https://cdn.imgpile.com/f/s1P9K4y_xl.png)

## Installation (WIP)

This section explains how to install **feedhook** as a service on a Unix-like server.

> [!NOTE]
> This guide uses [supervisor](http://supervisord.org/index.html) for running feedhook as service. Please install it first.

Create a "service" user with disabled login:

```sh
sudo adduser --disabled-login feedhook
```

Switch to the new user and move to the home directory:

```sh
sudo su feedhook
cd ~
```

Download and decompress executable:

```sh
wget https://github.com/ErikKalkoken/feedhook/releases/download/v0.1.16/feedhook-0.1.16-linux-amd64.tar.gz
tar -xvzf feedhook-0.1.16-linux-amd64.tar.gz
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

Setup feeds and webhooks by adding them to `config.toml`.

Add feedhook to supervisor:

```sh
sudo ln -s /home/feedhook/supervisor.conf /etc/supervisor/conf.d/feedhook.conf
sudo systemctl restart supervisor
```

Restart feedhook to start feedhook.

```sh
sudo supervisorctl restart feedhook
```
