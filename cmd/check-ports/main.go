package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Hosts    string
	Ports    string
	Protocol string
	Timeout  int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-ports",
			Short:    "Check TCP/UDP ports are open on one or more hosts",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "hosts",
			Argument:  "hosts",
			Shorthand: "H",
			Usage:     "Hosts to connect to, comma-separated",
			Value:     &plugin.Hosts,
			Default:   "0.0.0.0",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "ports",
			Argument:  "ports",
			Shorthand: "p",
			Usage:     "Ports to check, comma-separated with optional ranges (22,25,8100-8131,3030)",
			Value:     &plugin.Ports,
			Default:   "22",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "protocol",
			Argument:  "protocol",
			Shorthand: "P",
			Usage:     "Protocol to check: tcp (default) or udp",
			Value:     &plugin.Protocol,
			Default:   "tcp",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "t",
			Usage:     "Connection timeout in seconds",
			Value:     &plugin.Timeout,
			Default:   30,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	proto := strings.ToLower(plugin.Protocol)
	if proto != "tcp" && proto != "udp" {
		return sensu.CheckStateCritical, fmt.Errorf("protocol must be tcp or udp, got: %s", plugin.Protocol)
	}
	return sensu.CheckStateOK, nil
}

func expandPorts(raw string) ([]int, error) {
	var ports []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q: %v", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q: %v", part, err)
			}
			for p := start; p <= end; p++ {
				ports = append(ports, p)
			}
		} else {
			p, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port %q: %v", part, err)
			}
			ports = append(ports, p)
		}
	}
	return ports, nil
}

func checkPort(host string, port int, proto string, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout(proto, addr, timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func executeCheck(_ *corev2.Event) (int, error) {
	ports, err := expandPorts(plugin.Ports)
	if err != nil {
		return sensu.CheckStateCritical, err
	}

	hosts := strings.Split(plugin.Hosts, ",")
	proto := strings.ToLower(plugin.Protocol)
	timeout := time.Duration(plugin.Timeout) * time.Second
	expected := len(ports) * len(hosts)
	ok := 0

	for _, host := range hosts {
		host = strings.TrimSpace(host)
		for _, port := range ports {
			if err := checkPort(host, port, proto, timeout); err != nil {
				return sensu.CheckStateCritical, fmt.Errorf("connection failed %s:%d/%s: %v", host, port, proto, err)
			}
			ok++
		}
	}

	if ok == expected {
		fmt.Printf("All ports (%s) are accessible for hosts %s\n", plugin.Ports, plugin.Hosts)
		return sensu.CheckStateOK, nil
	}
	return sensu.CheckStateCritical, fmt.Errorf("only %d of %d port checks succeeded", ok, expected)
}
