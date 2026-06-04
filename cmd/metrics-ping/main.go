package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Scheme  string
	Host    string
	Count   int
	Timeout int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-ping",
			Short:    "Collect ICMP ping statistics as metrics",
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
		&sensu.PluginConfigOption[string]{
			Path:      "host",
			Argument:  "host",
			Shorthand: "H",
			Usage:     "Host to ping",
			Value:     &plugin.Host,
			Default:   "localhost",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "count",
			Argument:  "count",
			Shorthand: "c",
			Usage:     "Number of ping requests",
			Value:     &plugin.Count,
			Default:   5,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "t",
			Usage:     "Timeout in seconds per ping",
			Value:     &plugin.Timeout,
			Default:   5,
		},
	}
)

var (
	// iputils-ping summary: "5 packets transmitted, 5 received, 0% packet loss, time 4003ms"
	overviewRe = regexp.MustCompile(`(\d+) packets transmitted, (\d+) received, (\d+)% packet loss, time (\d+)ms`)
	// iputils-ping stats: "rtt min/avg/max/mdev = 0.016/0.019/0.024/0.004 ms"
	statsRe = regexp.MustCompile(`rtt min/avg/max/mdev = (\d+\.\d+)/(\d+\.\d+)/(\d+\.\d+)/(\d+\.\d+) ms`)
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
		plugin.Scheme = hostname + ".ping"
	}
	return sensu.CheckStateOK, nil
}

func executeMetric(_ *corev2.Event) (int, error) {
	timestamp := time.Now().Unix()

	args := []string{
		fmt.Sprintf("-W%d", plugin.Timeout),
		fmt.Sprintf("-c%d", plugin.Count),
		plugin.Host,
	}
	out, err := exec.Command("ping", args...).CombinedOutput()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("ping error: unable to ping %s: %v", plugin.Host, err)
	}

	output := string(out)

	om := overviewRe.FindStringSubmatch(output)
	if om == nil {
		return sensu.CheckStateCritical, fmt.Errorf("could not parse ping summary output for %s", plugin.Host)
	}

	overviewKeys := []string{"packets_transmitted", "packets_received", "packet_loss", "time"}
	for i, key := range overviewKeys {
		fmt.Printf("%s.%s %s %d\n", plugin.Scheme, key, om[i+1], timestamp)
	}

	sm := statsRe.FindStringSubmatch(output)
	if sm != nil {
		statsKeys := []string{"min", "avg", "max", "mdev"}
		for i, key := range statsKeys {
			// Normalise: remove trailing zeros for cleaner output, but keep at least one decimal
			val := strings.TrimRight(sm[i+1], "0")
			if strings.HasSuffix(val, ".") {
				val += "0"
			}
			fmt.Printf("%s.%s %s %d\n", plugin.Scheme, key, val, timestamp)
		}
	}

	return sensu.CheckStateOK, nil
}
