package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Scheme        string
	IgnoreDevices string
	IncludeDevices string
	OnlyUp        bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "metrics-net",
			Short:    "Collect network interface metrics from /sys/class/net",
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
			Path:      "ignore-device",
			Argument:  "ignore-device",
			Shorthand: "i",
			Usage:     "Comma-separated device name patterns to ignore",
			Value:     &plugin.IgnoreDevices,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "include-device",
			Argument:  "include-device",
			Shorthand: "I",
			Usage:     "Comma-separated device name patterns to include (overrides ignore)",
			Value:     &plugin.IncludeDevices,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "only-up",
			Argument:  "only-up",
			Shorthand: "u",
			Usage:     "Only include interfaces that are operationally up",
			Value:     &plugin.OnlyUp,
			Default:   false,
		},
	}
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
		plugin.Scheme = hostname + ".net"
	}
	return sensu.CheckStateOK, nil
}

func readStat(ifacePath, stat string) string {
	data, err := os.ReadFile(filepath.Join(ifacePath, "statistics", stat))
	if err != nil {
		return "0"
	}
	return strings.TrimSpace(string(data))
}

func matchesAny(name string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(name, strings.TrimSpace(p)) {
			return true
		}
	}
	return false
}

func executeMetric(_ *corev2.Event) (int, error) {
	timestamp := time.Now().Unix()

	var ignorePatterns, includePatterns []string
	if plugin.IgnoreDevices != "" {
		ignorePatterns = strings.Split(plugin.IgnoreDevices, ",")
	}
	if plugin.IncludeDevices != "" {
		includePatterns = strings.Split(plugin.IncludeDevices, ",")
	}

	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("failed to read /sys/class/net: %v", err)
	}

	for _, entry := range entries {
		iface := entry.Name()
		if iface == "lo" {
			continue
		}

		ifacePath := filepath.Join("/sys/class/net", iface)
		info, err := os.Lstat(ifacePath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			// /sys/class/net entries are symlinks; skip plain dirs (e.g. bonding_masters)
			realInfo, err2 := os.Stat(ifacePath)
			if err2 != nil || !realInfo.IsDir() {
				continue
			}
		}

		if len(ignorePatterns) > 0 && matchesAny(iface, ignorePatterns) {
			continue
		}
		if len(includePatterns) > 0 && !matchesAny(iface, includePatterns) {
			continue
		}

		if plugin.OnlyUp {
			operstate, err := os.ReadFile(filepath.Join(ifacePath, "operstate"))
			if err != nil || strings.TrimSpace(string(operstate)) != "up" {
				continue
			}
		}

		scheme := fmt.Sprintf("%s.%s", plugin.Scheme, iface)

		ifSpeed := "0"
		if data, err := os.ReadFile(filepath.Join(ifacePath, "speed")); err == nil {
			ifSpeed = strings.TrimSpace(string(data))
		}

		stats := map[string]string{
			"tx_packets": readStat(ifacePath, "tx_packets"),
			"rx_packets": readStat(ifacePath, "rx_packets"),
			"tx_bytes":   readStat(ifacePath, "tx_bytes"),
			"rx_bytes":   readStat(ifacePath, "rx_bytes"),
			"tx_errors":  readStat(ifacePath, "tx_errors"),
			"rx_errors":  readStat(ifacePath, "rx_errors"),
		}

		for _, key := range []string{"tx_packets", "rx_packets", "tx_bytes", "rx_bytes", "tx_errors", "rx_errors"} {
			fmt.Printf("%s.%s %s %d\n", scheme, key, stats[key], timestamp)
		}
		fmt.Printf("%s.if_speed %s %d\n", scheme, ifSpeed, timestamp)
	}
	return sensu.CheckStateOK, nil
}
