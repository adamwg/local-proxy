# local-proxy

`local-proxy` is a simple HTTP CONNECT proxy written in Go, which allows the
user to easily send selected traffic over a different network interface than
their default.

## System Setup and Proxy Usage

**This section assumes you're using Linux and NetworkManager.**

Suppose you have two network interfaces:

1. wlan0, which is your default.
2. eth0, which is connected to a network that can access certain resources not
   available over wlan0. Call this network "Work".

First, configure NetworkManager not to set up any routes for your eth0 network
other than a route to its own network. This can be done in the NetworkManager
GUI, or with the CLI:

```console
$ nmcli c modify Work ipv4.never-default yes
$ nmcli c modify Work ipv4.ignore-auto-routes yes
```

Configure your system to have a second routing table:

```console
# echo "2 rt2" >> /etc/iproute2/rt_tables
```

Now you can connect to the Work network and start the proxy on interface eth0:

```console
$ nmcli c up Work
$ local-proxy eth0
==================== CONFIG INFORMATION ====================
sudo ip route flush table rt2
sudo ip route add 192.168.1.0/24 dev eth0 proto kernel src 192.168.1.240 table main
sudo ip route add default via 192.168.1.1 dev wlan0 table rt2
sudo ip rule add from 192.168.1.240/32 table rt2
sudo ip rule add to 192.168.1.240/32 table rt2
============================================================
```

Run the commands it prints to set up your routes. The first `route add` line is
probably unnecessary, since NetworkManager should configure it for you, but the
proxy prints it just in case (doesn't hurt to run it again). The gateway in the
second `route add` line is a guess at what your gateway should be: the second IP
in the interface's network.

## Browser Setup

The proxy listens on 127.0.0.1:8080.

If you want to use a separate browser to access resources on the eth0 network,
configure that browser up to use the proxy.

If you want to use one browser for both networks you'll need an extension such
as FoxyProxy for Firefox or SwitchyOmega for Chrome. In these extensions you can
set up rules for which resources use the proxy.

## Known Limitations

* The proxy only accepts the HTTP CONNECT method, so if your browser tries to
  GET from it you'll see an error. For most browsers this probably means the
  proxy works only for HTTPS.

* The proxy logs a ton of stuff that isn't very interesting.
