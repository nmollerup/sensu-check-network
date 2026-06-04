# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `check-banner`: Connect to a TCP port, optionally write a string, read the response, and match against a regex pattern
- `check-mtu`: Check the MTU of a network interface via `/sys/class/net/<iface>/mtu`
- `check-netfilter-conntrack`: Check netfilter connection tracking table usage percentage
- `check-netstat-tcp`: Alert on TCP socket state counts from `/proc/net/tcp`
- `check-ping`: ICMP ping check with configurable success ratio thresholds
- `check-ports`: Check that TCP/UDP ports are open on one or more hosts
- `check-ports-bind`: Check ports by bound address:port/protocol specification
- `metrics-interface`: Collect per-interface network statistics from `/proc/net/dev`
- `metrics-net`: Collect per-interface packet/byte/error stats from `/sys/class/net`
- `metrics-netstat-tcp`: Collect TCP state counts as Graphite metrics from `/proc/net/tcp`
- `metrics-ping`: Collect ICMP ping statistics (latency, packet loss) as Graphite metrics
- `metrics-sockstat`: Collect socket statistics from `/proc/net/sockstat`
