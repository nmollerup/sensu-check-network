package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

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
	States   string
	Warning  string
	Critical string
	Port     int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-netstat-tcp",
			Short:    "Alert on TCP socket state thresholds from /proc/net/tcp",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "states",
			Argument:  "states",
			Shorthand: "s",
			Usage:     "Comma-separated list of TCP states to check (e.g. ESTABLISHED,CLOSE_WAIT)",
			Value:     &plugin.States,
			Default:   "ESTABLISHED",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Comma-separated warning thresholds, one per state",
			Value:     &plugin.Warning,
			Default:   "500",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Comma-separated critical thresholds, one per state",
			Value:     &plugin.Critical,
			Default:   "1000",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "port",
			Argument:  "port",
			Shorthand: "p",
			Usage:     "Limit check to this local port (default: all ports)",
			Value:     &plugin.Port,
			Default:   0,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	return sensu.CheckStateOK, nil
}

var procLineRe = regexp.MustCompile(`^\s*\d+:\s+(.{8}|.{32}):(.{4})\s+(.{8}|.{32}):(.{4})\s+(.{2})`)

func countTCPStates(protocols []string, wantStates []string, port int) map[string]int {
	counts := make(map[string]int)
	for _, s := range wantStates {
		counts[s] = 0
	}

	stateSet := make(map[string]bool, len(wantStates))
	for _, s := range wantStates {
		stateSet[s] = true
	}

	for _, proto := range protocols {
		f, err := os.Open("/proc/net/" + proto)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			m := procLineRe.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
			if m == nil {
				continue
			}
			stateName, ok := tcpStates[strings.ToUpper(m[5])]
			if !ok || !stateSet[stateName] {
				continue
			}
			if port != 0 {
				localPort, err := strconv.ParseInt(m[2], 16, 32)
				if err != nil || int(localPort) != port {
					continue
				}
			}
			counts[stateName]++
		}
		_ = f.Close()
	}
	return counts
}

func parseThresholds(raw string, count int, fallback int) []int {
	parts := strings.Split(raw, ",")
	out := make([]int, count)
	for i := 0; i < count; i++ {
		out[i] = fallback
		if i < len(parts) {
			if v, err := strconv.Atoi(strings.TrimSpace(parts[i])); err == nil {
				out[i] = v
			}
		}
	}
	return out
}

func executeCheck(_ *corev2.Event) (int, error) {
	states := strings.Split(plugin.States, ",")
	for i, s := range states {
		states[i] = strings.TrimSpace(s)
	}
	warnings := parseThresholds(plugin.Warning, len(states), 500)
	criticals := parseThresholds(plugin.Critical, len(states), 1000)

	counts := countTCPStates([]string{"tcp", "tcp6"}, states, plugin.Port)

	isCritical := false
	isWarning := false
	var parts []string

	for i, state := range states {
		count := counts[state]
		switch {
		case count >= criticals[i]:
			isCritical = true
			parts = append(parts, fmt.Sprintf("CRITICAL:%s=%d", state, count))
		case count >= warnings[i]:
			isWarning = true
			parts = append(parts, fmt.Sprintf("WARNING:%s=%d", state, count))
		default:
			parts = append(parts, fmt.Sprintf("OK:%s=%d", state, count))
		}
	}

	msg := strings.Join(parts, " ")
	if isCritical {
		return sensu.CheckStateCritical, fmt.Errorf("%s", msg)
	}
	if isWarning {
		return sensu.CheckStateWarning, fmt.Errorf("%s", msg)
	}
	fmt.Println(msg)
	return sensu.CheckStateOK, nil
}
