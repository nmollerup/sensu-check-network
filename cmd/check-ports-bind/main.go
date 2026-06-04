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
	Hard       bool
	DefaultHost string
	PortBinds  string
	Timeout    int
	Warn       bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-ports-bind",
			Short:    "Check TCP/UDP ports by bound address:port/protocol",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[bool]{
			Path:      "hard",
			Argument:  "hard",
			Shorthand: "d",
			Usage:     "Check ports on both TCP and UDP when no explicit protocol is set",
			Value:     &plugin.Hard,
			Default:   false,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "host",
			Argument:  "host",
			Shorthand: "H",
			Usage:     "Default host address when none is specified in port bind",
			Value:     &plugin.DefaultHost,
			Default:   "0.0.0.0",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "portbinds",
			Argument:  "portbinds",
			Shorthand: "p",
			Usage:     "Port bindings to check: address:port/protocol, comma-separated (e.g. 127.0.0.1:22,0.0.0.0:80/tcp,127.0.0.1:8100-8131/udp,10.0.0.1:443/both)",
			Value:     &plugin.PortBinds,
			Default:   "0.0.0.0:22",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "t",
			Usage:     "Connection timeout in seconds",
			Value:     &plugin.Timeout,
			Default:   10,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "warn",
			Argument:  "warn",
			Shorthand: "w",
			Usage:     "Alert as warning instead of critical on failure",
			Value:     &plugin.Warn,
			Default:   false,
		},
	}
)

type portBind struct {
	address  string
	port     int
	protocol string
}

func (pb portBind) String() string {
	return fmt.Sprintf("%s:%d/%s", pb.address, pb.port, pb.protocol)
}

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	return sensu.CheckStateOK, nil
}

func parsePortBinds() ([]portBind, error) {
	defaultProto := "tcp"
	if plugin.Hard {
		defaultProto = "both"
	}

	var binds []portBind
	for _, spec := range strings.Split(plugin.PortBinds, ",") {
		spec = strings.TrimSpace(spec)

		// Inject default host if missing
		if !strings.Contains(spec, ":") {
			spec = plugin.DefaultHost + ":" + spec
		}

		// Inject default protocol if missing
		if !strings.Contains(spec, "/") {
			spec = spec + "/" + defaultProto
		}

		slashIdx := strings.LastIndex(spec, "/")
		proto := spec[slashIdx+1:]
		addrPort := spec[:slashIdx]

		colonIdx := strings.LastIndex(addrPort, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("invalid portbind %q: missing colon", spec)
		}
		address := addrPort[:colonIdx]
		portStr := addrPort[colonIdx+1:]

		portList, err := expandPortRange(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port in %q: %v", spec, err)
		}

		for _, port := range portList {
			switch strings.ToLower(proto) {
			case "both":
				binds = append(binds, portBind{address, port, "tcp"}, portBind{address, port, "udp"})
			case "tcp", "udp":
				binds = append(binds, portBind{address, port, strings.ToLower(proto)})
			default:
				return nil, fmt.Errorf("unsupported protocol %q in %q", proto, spec)
			}
		}
	}
	return binds, nil
}

func expandPortRange(raw string) ([]int, error) {
	if strings.Contains(raw, "-") {
		parts := strings.SplitN(raw, "-", 2)
		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, err
		}
		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, err
		}
		ports := make([]int, 0, end-start+1)
		for p := start; p <= end; p++ {
			ports = append(ports, p)
		}
		return ports, nil
	}
	p, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return []int{p}, nil
}

func dialPort(pb portBind, timeout time.Duration) error {
	addr := fmt.Sprintf("%s:%d", pb.address, pb.port)
	conn, err := net.DialTimeout(pb.protocol, addr, timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func alert(warn bool, msg string) (int, error) {
	if warn {
		return sensu.CheckStateWarning, fmt.Errorf("%s", msg)
	}
	return sensu.CheckStateCritical, fmt.Errorf("%s", msg)
}

func executeCheck(_ *corev2.Event) (int, error) {
	binds, err := parsePortBinds()
	if err != nil {
		return sensu.CheckStateCritical, err
	}

	timeout := time.Duration(plugin.Timeout) * time.Second
	var okList []string

	for _, pb := range binds {
		if err := dialPort(pb, timeout); err != nil {
			return alert(plugin.Warn, fmt.Sprintf("connection failed %s: %v", pb, err))
		}
		okList = append(okList, pb.String())
	}

	fmt.Printf("All ports (%s) are reachable: %s\n", plugin.PortBinds, strings.Join(okList, ", "))
	return sensu.CheckStateOK, nil
}
