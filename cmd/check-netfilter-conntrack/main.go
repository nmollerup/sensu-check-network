package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Warning  int
	Critical int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-netfilter-conntrack",
			Short:    "Check netfilter connection tracking table usage",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[int]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warn when conntrack table is filled above this percentage",
			Value:     &plugin.Warning,
			Default:   80,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical when conntrack table is filled above this percentage",
			Value:     &plugin.Critical,
			Default:   90,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	if plugin.Warning >= plugin.Critical {
		return sensu.CheckStateCritical, fmt.Errorf("warning threshold (%d) must be less than critical (%d)", plugin.Warning, plugin.Critical)
	}
	return sensu.CheckStateOK, nil
}

func readProcInt(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func executeCheck(_ *corev2.Event) (int, error) {
	max, err := readProcInt("/proc/sys/net/netfilter/nf_conntrack_max")
	if err != nil {
		return sensu.CheckStateWarning, fmt.Errorf("can't read conntrack information: %v", err)
	}

	count, err := readProcInt("/proc/sys/net/netfilter/nf_conntrack_count")
	if err != nil {
		return sensu.CheckStateWarning, fmt.Errorf("can't read conntrack information: %v", err)
	}

	pct := (float64(count) / float64(max)) * 100.0
	msg := fmt.Sprintf("table is at %.1f%% (%d/%d)", pct, count, max)

	switch {
	case pct >= float64(plugin.Critical):
		return sensu.CheckStateCritical, fmt.Errorf("%s", msg)
	case pct >= float64(plugin.Warning):
		return sensu.CheckStateWarning, fmt.Errorf("%s", msg)
	}

	fmt.Println(msg)
	return sensu.CheckStateOK, nil
}
