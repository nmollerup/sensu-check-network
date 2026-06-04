package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

var tcpStates = map[string]string{
	"00": "UNKNOWN",
	"FF": "UNKNOWN",
	"01": "ESTABLISHED",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

type Config struct {
	sensu.PluginConfig
	Scheme      string
	Port        int
	PortType    string
	DisableTCP6 bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-netstat-tcp",
			Short:    "Collect TCP state counts from /proc/net/tcp",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "scheme",
			Argument:  "scheme",
			Shorthand: "s",
			Usage:     "Metric naming scheme prefix",
			Value:     &plugin.Scheme,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "port",
			Argument:  "port",
			Shorthand: "p",
			Usage:     "Filter metrics to this port (default: all)",
			Value:     &plugin.Port,
			Default:   0,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "type",
			Argument:  "type",
			Shorthand: "t",
			Usage:     "Port type to filter on: local (default) or remote",
			Value:     &plugin.PortType,
			Default:   "local",
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "disable-tcp6",
			Argument:  "disable-tcp6",
			Shorthand: "d",
			Usage:     "Disable TCP6 metrics",
			Value:     &plugin.DisableTCP6,
			Default:   false,
		},
	}
)

var (
	tcp4Re = regexp.MustCompile(`^\s*\d+:\s+(.{8}):(.{4})\s+(.{8}):(.{4})\s+(.{2})`)
	tcp6Re = regexp.MustCompile(`^\s*\d+:\s+(.{32}):(.{4})\s+(.{32}):(.{4})\s+(.{2})`)
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeMetric, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	if plugin.Scheme == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "localhost"
		}
		plugin.Scheme = hostname + ".tcp"
	}
	if plugin.PortType != "local" && plugin.PortType != "remote" {
		return sensu.CheckStateCritical, fmt.Errorf("type must be 'local' or 'remote', got: %s", plugin.PortType)
	}
	return sensu.CheckStateOK, nil
}

func parseStates(proto string, pattern *regexp.Regexp, counts map[string]int) {
	f, err := os.Open("/proc/net/" + proto)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := pattern.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if m == nil {
			continue
		}

		var portHex string
		if plugin.PortType == "remote" {
			portHex = m[4]
		} else {
			portHex = m[2]
		}

		stateName, ok := tcpStates[strings.ToUpper(m[5])]
		if !ok {
			continue
		}

		if plugin.Port != 0 {
			p, err := strconv.ParseInt(portHex, 16, 32)
			if err != nil || int(p) != plugin.Port {
				continue
			}
		}
		counts[stateName]++
	}
}

func executeMetric(_ *corev2.Event) (int, error) {
	timestamp := time.Now().Unix()

	counts := make(map[string]int)
	for _, name := range tcpStates {
		counts[name] = 0
	}

	parseStates("tcp", tcp4Re, counts)
	if !plugin.DisableTCP6 {
		parseStates("tcp6", tcp6Re, counts)
	}

	stateOrder := []string{
		"UNKNOWN", "ESTABLISHED", "SYN_SENT", "SYN_RECV",
		"FIN_WAIT1", "FIN_WAIT2", "TIME_WAIT", "CLOSE",
		"CLOSE_WAIT", "LAST_ACK", "LISTEN", "CLOSING",
	}

	for _, state := range stateOrder {
		var name string
		if plugin.Port != 0 {
			name = fmt.Sprintf("%s.%d.%s.%s", plugin.Scheme, plugin.Port, plugin.PortType, state)
		} else {
			name = fmt.Sprintf("%s.%s", plugin.Scheme, state)
		}
		fmt.Printf("%s %d %d\n", name, counts[state], timestamp)
	}
	return sensu.CheckStateOK, nil
}
