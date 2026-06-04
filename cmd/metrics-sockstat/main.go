package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Scheme string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-sockstat",
			Short:    "Collect socket statistics from /proc/net/sockstat",
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
	}
)

var kvRe = regexp.MustCompile(`([A-Za-z_]+) (\d+)`)

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
		plugin.Scheme = hostname + ".network.sockets"
	}
	return sensu.CheckStateOK, nil
}

func executeMetric(_ *corev2.Event) (int, error) {
	timestamp := time.Now().Unix()

	data, err := os.ReadFile("/proc/net/sockstat")
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to read /proc/net/sockstat: %v", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		if fields[0] == "sockets:" {
			if len(fields) >= 3 {
				fmt.Printf("%s.total_used %s %d\n", plugin.Scheme, fields[2], timestamp)
			}
			continue
		}

		proto := strings.TrimSuffix(fields[0], ":")
		rest := strings.Join(fields[1:], " ")
		for _, m := range kvRe.FindAllStringSubmatch(rest, -1) {
			fmt.Printf("%s.%s.%s %s %d\n", plugin.Scheme, proto, m[1], m[2], timestamp)
		}
	}
	return sensu.CheckStateOK, nil
}
