package main

import (
	"bufio"
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
	Scheme                string
	ExcludeInterfaceRegex string
	IncludeInterfaceRegex string
	ExcludeInterfaces     string
	IncludeInterfaces     string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-interface",
			Short:    "Collect network interface metrics from /proc/net/dev",
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
			Path:      "exclude-interface-regex",
			Argument:  "exclude-interface-regex",
			Shorthand: "X",
			Usage:     "Regex to match interfaces to exclude",
			Value:     &plugin.ExcludeInterfaceRegex,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "include-interface-regex",
			Argument:  "include-interface-regex",
			Shorthand: "I",
			Usage:     "Regex to match interfaces to include",
			Value:     &plugin.IncludeInterfaceRegex,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "exclude-interface",
			Argument:  "exclude-interface",
			Shorthand: "x",
			Usage:     "Comma-separated list of interfaces to exclude",
			Value:     &plugin.ExcludeInterfaces,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "include-interface",
			Argument:  "include-interface",
			Shorthand: "i",
			Usage:     "Comma-separated list of interfaces to include",
			Value:     &plugin.IncludeInterfaces,
		},
	}
)

var metricFields = []string{
	"rxBytes", "rxPackets", "rxErrors", "rxDrops",
	"rxFifo", "rxFrame", "rxCompressed", "rxMulticast",
	"txBytes", "txPackets", "txErrors", "txDrops",
	"txFifo", "txColls", "txCarrier", "txCompressed",
}

var lineRe = regexp.MustCompile(`^\s*([^:]+):\s*(.*)$`)

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
		plugin.Scheme = hostname + ".interface"
	}
	return sensu.CheckStateOK, nil
}

func allZero(stats []string) bool {
	for _, s := range stats {
		if s != "0" {
			return false
		}
	}
	return true
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func executeMetric(_ *corev2.Event) (int, error) {
	timestamp := time.Now().Unix()

	var excludeRe, includeRe *regexp.Regexp
	if plugin.ExcludeInterfaceRegex != "" {
		r, err := regexp.Compile(plugin.ExcludeInterfaceRegex)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("invalid exclude-interface-regex: %v", err)
		}
		excludeRe = r
	}
	if plugin.IncludeInterfaceRegex != "" {
		r, err := regexp.Compile(plugin.IncludeInterfaceRegex)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("invalid include-interface-regex: %v", err)
		}
		includeRe = r
	}

	var excludeList, includeList []string
	if plugin.ExcludeInterfaces != "" {
		for _, s := range strings.Split(plugin.ExcludeInterfaces, ",") {
			excludeList = append(excludeList, strings.TrimSpace(s))
		}
	}
	if plugin.IncludeInterfaces != "" {
		for _, s := range strings.Split(plugin.IncludeInterfaces, ",") {
			includeList = append(includeList, strings.TrimSpace(s))
		}
	}

	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to open /proc/net/dev: %v", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := lineRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		iface := strings.TrimSpace(m[1])
		statsStr := strings.TrimSpace(m[2])

		if excludeRe != nil && excludeRe.MatchString(iface) {
			continue
		}
		if includeRe != nil && !includeRe.MatchString(iface) {
			continue
		}
		if containsStr(excludeList, iface) {
			continue
		}
		if len(includeList) > 0 && !containsStr(includeList, iface) {
			continue
		}

		stats := strings.Fields(statsStr)
		if len(stats) < len(metricFields) {
			continue
		}
		if allZero(stats[:len(metricFields)]) {
			continue
		}

		safeName := strings.ReplaceAll(iface, ".", "_")
		for i, field := range metricFields {
			fmt.Printf("%s.%s.%s %s %d\n", plugin.Scheme, safeName, field, stats[i], timestamp)
		}
	}
	if err := scanner.Err(); err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error reading /proc/net/dev: %v", err)
	}
	return sensu.CheckStateOK, nil
}
