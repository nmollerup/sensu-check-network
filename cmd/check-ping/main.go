package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Host          string
	Timeout       int
	Count         int
	WarnRatio     float64
	CriticalRatio float64
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-ping",
			Short:    "ICMP ping check",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "host",
			Argument:  "host",
			Shorthand: "H",
			Usage:     "Host to ping",
			Value:     &plugin.Host,
			Default:   "localhost",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "T",
			Usage:     "Timeout in seconds per ping",
			Value:     &plugin.Timeout,
			Default:   5,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "count",
			Argument:  "count",
			Shorthand: "n",
			Usage:     "Number of ping requests",
			Value:     &plugin.Count,
			Default:   1,
		},
		&sensu.PluginConfigOption[float64]{
			Path:      "warn-ratio",
			Argument:  "warn-ratio",
			Shorthand: "W",
			Usage:     "Warn if success ratio falls at or below this value (0.0-1.0)",
			Value:     &plugin.WarnRatio,
			Default:   0.5,
		},
		&sensu.PluginConfigOption[float64]{
			Path:      "critical-ratio",
			Argument:  "critical-ratio",
			Shorthand: "C",
			Usage:     "Critical if success ratio falls at or below this value (0.0-1.0)",
			Value:     &plugin.CriticalRatio,
			Default:   0.2,
		},
	}
)

var summaryRe = regexp.MustCompile(`(\d+) packets transmitted, (\d+) received`)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	if strings.TrimSpace(plugin.Host) == "" {
		return sensu.CheckStateCritical, fmt.Errorf("host is required")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *corev2.Event) (int, error) {
	args := []string{
		fmt.Sprintf("-W%d", plugin.Timeout),
		fmt.Sprintf("-c%d", plugin.Count),
		plugin.Host,
	}
	out, _ := exec.Command("ping", args...).CombinedOutput()

	transmitted, received := plugin.Count, 0
	if m := summaryRe.FindStringSubmatch(string(out)); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			transmitted = v
		}
		if v, err := strconv.Atoi(m[2]); err == nil {
			received = v
		}
	}

	ratio := float64(received) / float64(transmitted)

	if ratio > plugin.WarnRatio {
		fmt.Printf("ICMP ping successful for host: %s\n", plugin.Host)
		return sensu.CheckStateOK, nil
	}

	failMsg := fmt.Sprintf("ICMP ping unsuccessful for host: %s (successful: %d/%d)", plugin.Host, received, transmitted)
	if ratio <= plugin.CriticalRatio {
		return sensu.CheckStateCritical, fmt.Errorf("%s", failMsg)
	}
	return sensu.CheckStateWarning, fmt.Errorf("%s", failMsg)
}
