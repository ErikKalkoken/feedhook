# feedforward

A service for forwarding RSS and Atom feeds to Discord webhooks.

- Forward RSS and Atom feeds to webhooks on Discord
- Build for high throughput
- Easy configuration

## Installation (WIP)

This section explains how to install **feedforward** as a service on a Unix-like server.

> !NOTE
> This guide uses [supervisor](http://supervisord.org/index.html) for running feedforward as service. Please install it first.

Create a "service" user with disabled login:

```sh
sudo adduser --disabled-login feedforward
```

Switch to the new user and move to the home directory:

```sh
sudo su feedforward
cd ~
```

Download files:

```sh
wget executable
wget supervisor.conf
wget config.toml
```

Add feedforward to supervisor:

```sh
sudo ln -s /home/feedforward/supervisor.conf /etc/supervisor/conf.d/feedforward.conf
sudo systemctl restart supervisor
```

Setup feeds and webhooks by adding them to config.toml.

Restart feedforward to enable the latest configuration.

```sh
sudo supervisorctl restart feedforward
```
