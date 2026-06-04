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
	Interface string
	MTU       int
	Warn      bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-mtu",
			Short:    "Check MTU of a network interface",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "interface",
			Argument:  "interface",
			Shorthand: "i",
			Usage:     "Network interface to check",
			Value:     &plugin.Interface,
			Default:   "eth0",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "mtu",
			Argument:  "mtu",
			Shorthand: "m",
			Usage:     "Expected MTU size in bytes",
			Value:     &plugin.MTU,
			Default:   1500,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "warn",
			Argument:  "warn",
			Shorthand: "w",
			Usage:     "Alert as warning instead of critical on MTU mismatch",
			Value:     &plugin.Warn,
			Default:   false,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	if strings.TrimSpace(plugin.Interface) == "" {
		return sensu.CheckStateCritical, fmt.Errorf("interface name is required")
	}
	return sensu.CheckStateOK, nil
}

func alert(warn bool, msg string) (int, error) {
	if warn {
		return sensu.CheckStateWarning, fmt.Errorf("%s", msg)
	}
	return sensu.CheckStateCritical, fmt.Errorf("%s", msg)
}

func executeCheck(_ *corev2.Event) (int, error) {
	mtuFile := fmt.Sprintf("/sys/class/net/%s/mtu", plugin.Interface)

	data, err := os.ReadFile(mtuFile)
	if err != nil {
		return alert(plugin.Warn, fmt.Sprintf("%s does not exist or is not readable", mtuFile))
	}

	mtu, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return alert(plugin.Warn, fmt.Sprintf("failed to parse MTU from %s: %v", mtuFile, err))
	}

	if mtu != plugin.MTU {
		return alert(plugin.Warn, fmt.Sprintf("required MTU is %d but found %d on %s", plugin.MTU, mtu, plugin.Interface))
	}

	fmt.Printf("%d matches %d on %s\n", mtu, plugin.MTU, plugin.Interface)
	return sensu.CheckStateOK, nil
}
