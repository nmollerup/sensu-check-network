package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Hosts          string
	Port           int
	Write          string
	ExcludeNewline bool
	Pattern        string
	Timeout        int
	CountMatch     int
	ReadTill       string
	OKMessage      string
	CritMessage    string
	SSL            bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-banner",
			Short:    "Connect to a TCP port, read the banner, and match against a pattern",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "hosts",
			Argument:  "hosts",
			Shorthand: "H",
			Usage:     "Host(s) to connect to, comma-separated",
			Value:     &plugin.Hosts,
			Default:   "localhost",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "port",
			Argument:  "port",
			Shorthand: "p",
			Usage:     "Port to connect to",
			Value:     &plugin.Port,
			Default:   22,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "write",
			Argument:  "write",
			Shorthand: "w",
			Usage:     "String to write to the socket after connecting",
			Value:     &plugin.Write,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "exclude-newline",
			Argument:  "exclude-newline",
			Shorthand: "e",
			Usage:     "Exclude trailing newline from write string",
			Value:     &plugin.ExcludeNewline,
			Default:   false,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "pattern",
			Argument:  "pattern",
			Shorthand: "q",
			Usage:     "Regex pattern to match in the banner response",
			Value:     &plugin.Pattern,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Argument:  "timeout",
			Shorthand: "t",
			Usage:     "Connection timeout in seconds",
			Value:     &plugin.Timeout,
			Default:   30,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "count",
			Argument:  "count",
			Shorthand: "c",
			Usage:     "Number of successful host matches required",
			Value:     &plugin.CountMatch,
			Default:   1,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "read-till",
			Argument:  "read-till",
			Shorthand: "r",
			Usage:     `Read until this character; use "EOF" to read the full response`,
			Value:     &plugin.ReadTill,
			Default:   "\n",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "ok-message",
			Argument:  "ok-message",
			Shorthand: "O",
			Usage:     "Custom OK message",
			Value:     &plugin.OKMessage,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "crit-message",
			Argument:  "crit-message",
			Shorthand: "C",
			Usage:     "Custom critical message",
			Value:     &plugin.CritMessage,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "ssl",
			Argument:  "ssl",
			Shorthand: "S",
			Usage:     "Use SSL/TLS for the connection",
			Value:     &plugin.SSL,
			Default:   false,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	if plugin.Port <= 0 || plugin.Port > 65535 {
		return sensu.CheckStateCritical, fmt.Errorf("invalid port: %d", plugin.Port)
	}
	return sensu.CheckStateOK, nil
}

func openConn(host string) (net.Conn, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", plugin.Port))
	timeout := time.Duration(plugin.Timeout) * time.Second

	tcpConn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	if err := tcpConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = tcpConn.Close()
		return nil, err
	}

	if plugin.SSL {
		tlsConn := tls.Client(tcpConn, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
		if err := tlsConn.Handshake(); err != nil {
			_ = tcpConn.Close()
			return nil, fmt.Errorf("TLS handshake failed: %v", err)
		}
		return tlsConn, nil
	}
	return tcpConn, nil
}

func acquireBanner(host string) (string, error) {
	conn, err := openConn(host)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	if plugin.Write != "" {
		msg := plugin.Write
		if !plugin.ExcludeNewline {
			msg += "\n"
		}
		if _, err := fmt.Fprint(conn, msg); err != nil {
			return "", fmt.Errorf("write failed: %v", err)
		}
	}

	if plugin.ReadTill == "EOF" {
		data, err := io.ReadAll(conn)
		if err != nil && err != io.EOF {
			return "", err
		}
		return string(data), nil
	}

	delim := byte('\n')
	if len(plugin.ReadTill) > 0 {
		delim = plugin.ReadTill[0]
	}
	line, err := bufio.NewReader(conn).ReadString(delim)
	return line, err
}

func critMsg(defaultMsg string) error {
	if plugin.CritMessage != "" {
		return fmt.Errorf("%s", plugin.CritMessage)
	}
	return fmt.Errorf("%s", defaultMsg)
}

func okMsg(defaultMsg string) string {
	if plugin.OKMessage != "" {
		return plugin.OKMessage
	}
	return defaultMsg
}

func executeCheck(_ *corev2.Event) (int, error) {
	hosts := strings.Split(plugin.Hosts, ",")
	successCount := 0

	for _, host := range hosts {
		host = strings.TrimSpace(host)

		if plugin.Pattern == "" {
			conn, err := openConn(host)
			if err != nil {
				return sensu.CheckStateCritical, critMsg(fmt.Sprintf("connection failed for %s:%d: %v", host, plugin.Port, err))
			}
			_ = conn.Close()
			successCount++
		} else {
			banner, err := acquireBanner(host)
			if err != nil {
				return sensu.CheckStateCritical, critMsg(fmt.Sprintf("failed to read banner from %s:%d: %v", host, plugin.Port, err))
			}
			matched, err := regexp.MatchString(plugin.Pattern, banner)
			if err != nil {
				return sensu.CheckStateCritical, fmt.Errorf("invalid pattern %q: %v", plugin.Pattern, err)
			}
			if matched {
				successCount++
			}
		}

		if successCount == plugin.CountMatch {
			if plugin.Pattern != "" {
				fmt.Println(okMsg(fmt.Sprintf("pattern %s matched", plugin.Pattern)))
			} else {
				fmt.Println(okMsg(fmt.Sprintf("port %d open", plugin.Port)))
			}
			return sensu.CheckStateOK, nil
		}
	}

	return sensu.CheckStateCritical, critMsg(fmt.Sprintf("port count or pattern %s does not match", plugin.Pattern))
}
