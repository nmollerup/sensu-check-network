[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/nmollerup/sensu-check-network)
[![goreleaser](https://github.com/nmollerup/sensu-check-network/actions/workflows/release.yml/badge.svg)](https://github.com/nmollerup/sensu-check-network/actions/workflows/release.yml) [![Go Test](https://github.com/nmollerup/sensu-check-network/actions/workflows/test.yml/badge.svg)](https://github.com/nmollerup/sensu-check-network/actions/workflows/test.yml)

# sensu-check-network

A Go reimplementation of [sensu-plugins-network-checks](https://github.com/sensu-plugins/sensu-plugins-network-checks) providing native network checks and metrics for [Sensu Go](https://sensu.io).

## Plugins

### Checks

- **check-banner** — Connect to a TCP port, read the banner, and match against a pattern
- **check-mtu** — Check MTU of a network interface
- **check-netfilter-conntrack** — Check netfilter connection tracking table usage
- **check-netstat-tcp** — Alert on TCP socket state thresholds from /proc/net/tcp
- **check-ping** — ICMP ping check
- **check-ports** — Check TCP/UDP ports are open on one or more hosts
- **check-ports-bind** — Check TCP/UDP ports by bound address:port/protocol

### Metrics

- **metrics-interface** — Collect network interface metrics from /proc/net/dev
- **metrics-net** — Collect network interface metrics from /sys/class/net
- **metrics-netstat-tcp** — Collect TCP state counts from /proc/net/tcp
- **metrics-ping** — Collect ICMP ping statistics as metrics
- **metrics-sockstat** — Collect socket statistics from /proc/net/sockstat

All metrics are output in Graphite plaintext format: `<scheme>.<category>.<name> <value> <timestamp>`

## Usage

Every plugin supports `--help` for full option details.

### Examples

```bash
# Check if a banner matches a pattern
check-banner --hosts myhost --port 22 --pattern "OpenSSH"

# Check MTU on an interface
check-mtu --interface eth0 --mtu 1500

# Alert when conntrack table exceeds thresholds
check-netfilter-conntrack --warning 80 --critical 90

# Alert on TCP connections in a specific state
check-netstat-tcp --state ESTABLISHED --warning 1000 --critical 2000

# ICMP ping check
check-ping --host 8.8.8.8

# Check that ports are open
check-ports --hosts myhost --ports 80,443

# Check ports by bound address
check-ports-bind --port-binds "0.0.0.0:80/tcp,0.0.0.0:443/tcp"

# Collect interface metrics
metrics-interface --scheme myhost.network

# Collect net metrics
metrics-net --scheme myhost.network

# Collect TCP state metrics
metrics-netstat-tcp --scheme myhost.network

# Collect ping metrics
metrics-ping --host 8.8.8.8 --scheme myhost.network

# Collect socket stats
metrics-sockstat --scheme myhost.network
```

## Installation

Download the latest release from the [releases page](https://github.com/nmollerup/sensu-check-network/releases) or register as a Sensu Bonsai asset.

## Configuration

Checks return standard Sensu exit codes (0=OK, 1=WARNING, 2=CRITICAL). Metric plugins output Graphite-formatted data to stdout for Sensu metric collection.
