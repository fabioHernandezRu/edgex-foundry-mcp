// Package config loads and validates the edgex-foundry-mcp configuration from
// command-line flags and environment variables (flag > env > default).
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Default values.
const (
	DefaultMetadataURL = "http://localhost:59881"
	DefaultDataURL     = "http://localhost:59880"
	DefaultCommandURL  = "http://localhost:59882"
	DefaultTimeout     = 10 * time.Second
	DefaultMaxResults  = 100
	DefaultHTTPAddr    = "127.0.0.1:8080"

	// MaxResultsLimit is the EdgeX default Service.MaxResultCount. EdgeX answers
	// 400 to any larger limit.
	MaxResultsLimit = 1024

	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// Secret is a string that never prints its value.
type Secret string

// String implements fmt.Stringer without revealing the value.
func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return "***"
}

// GoString implements fmt.GoStringer without revealing the value.
func (s Secret) GoString() string { return s.String() }

// Config is the validated server configuration.
type Config struct {
	MetadataURL   string
	DataURL       string
	CommandURL    string
	GatewayURL    string
	Token         Secret
	GatewayCAFile string

	Timeout    time.Duration
	MaxResults int

	Transport string
	HTTPAddr  string

	EnableWrites       bool
	DisableDeviceReads bool

	LogLevel    slog.Level
	ShowVersion bool
}

// GatewayMode reports whether services are reached through the API gateway.
func (c *Config) GatewayMode() bool { return c.GatewayURL != "" }

// Warnings returns non-fatal configuration issues that should be logged at startup.
func (c *Config) Warnings() []string {
	var w []string
	if c.GatewayMode() && c.Token == "" {
		w = append(w, "gateway mode without a token: the EdgeX API gateway will most likely answer 401; set EDGEX_TOKEN or --token-file")
	}
	if c.GatewayMode() && c.Token != "" {
		if u, err := url.Parse(c.GatewayURL); err == nil && u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
			w = append(w, "gateway URL uses http:// to a non-loopback host: the bearer token is sent in cleartext; use https://")
		}
	}
	if c.Transport == TransportHTTP && !IsLoopbackAddr(c.HTTPAddr) {
		w = append(w, fmt.Sprintf("HTTP transport listening on %s: the MCP endpoint has no client authentication and is reachable from the network", c.HTTPAddr))
	}
	return w
}

// IsLoopbackAddr reports whether a host:port listen address binds only to loopback.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	return isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Load parses args (without the program name) and environment variables
// obtained through getenv. Help and usage output is written to out.
func Load(args []string, getenv func(string) string, out io.Writer) (*Config, error) {
	fs := flag.NewFlagSet("edgex-foundry-mcp", flag.ContinueOnError)
	fs.SetOutput(out)

	envStr := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	var envErrs []error
	envBool := func(key string) bool {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			return false
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			envErrs = append(envErrs, fmt.Errorf("%s: invalid boolean %q", key, v))
		}
		return b
	}
	envInt := func(key string, def int) int {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			envErrs = append(envErrs, fmt.Errorf("%s: invalid integer %q", key, v))
			return def
		}
		return n
	}
	envDur := func(key string, def time.Duration) time.Duration {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			return def
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			envErrs = append(envErrs, fmt.Errorf("%s: invalid duration %q", key, v))
			return def
		}
		return d
	}

	c := &Config{}
	var tokenFile, logLevel string
	fs.StringVar(&c.MetadataURL, "metadata-url", envStr("EDGEX_METADATA_URL", DefaultMetadataURL), "core-metadata base URL (env EDGEX_METADATA_URL)")
	fs.StringVar(&c.DataURL, "data-url", envStr("EDGEX_DATA_URL", DefaultDataURL), "core-data base URL (env EDGEX_DATA_URL)")
	fs.StringVar(&c.CommandURL, "command-url", envStr("EDGEX_COMMAND_URL", DefaultCommandURL), "core-command base URL (env EDGEX_COMMAND_URL)")
	fs.StringVar(&c.GatewayURL, "gateway-url", envStr("EDGEX_GATEWAY_URL", ""), "secure-mode API gateway base URL, e.g. https://host:8443 (env EDGEX_GATEWAY_URL)")
	fs.StringVar(&tokenFile, "token-file", envStr("EDGEX_TOKEN_FILE", ""), "file containing the JWT bearer token (env EDGEX_TOKEN_FILE; or set EDGEX_TOKEN)")
	fs.StringVar(&c.GatewayCAFile, "gateway-ca-file", envStr("EDGEX_GATEWAY_CA_FILE", ""), "PEM CA bundle to trust for the gateway, in addition to system roots (env EDGEX_GATEWAY_CA_FILE)")
	fs.DurationVar(&c.Timeout, "timeout", envDur("EDGEX_TIMEOUT", DefaultTimeout), "timeout for each EdgeX HTTP request (env EDGEX_TIMEOUT)")
	fs.IntVar(&c.MaxResults, "max-results", envInt("EDGEX_MAX_RESULTS", DefaultMaxResults), "maximum items returned by any list/query tool, 1..1024 (env EDGEX_MAX_RESULTS)")
	fs.StringVar(&c.Transport, "transport", envStr("EDGEX_MCP_TRANSPORT", TransportStdio), "MCP transport: stdio or http (env EDGEX_MCP_TRANSPORT)")
	fs.StringVar(&c.HTTPAddr, "http-addr", envStr("EDGEX_MCP_HTTP_ADDR", DefaultHTTPAddr), "listen address for --transport http (env EDGEX_MCP_HTTP_ADDR)")
	fs.BoolVar(&c.EnableWrites, "enable-writes", envBool("EDGEX_ENABLE_WRITES"), "register write/actuation tools that can affect physical hardware (env EDGEX_ENABLE_WRITES)")
	fs.BoolVar(&c.DisableDeviceReads, "disable-device-reads", envBool("EDGEX_DISABLE_DEVICE_READS"), "do not register read_device_command, which reads physical devices live (env EDGEX_DISABLE_DEVICE_READS)")
	fs.StringVar(&logLevel, "log-level", envStr("EDGEX_LOG_LEVEL", "info"), "log level: debug, info, warn, error (env EDGEX_LOG_LEVEL)")
	fs.BoolVar(&c.ShowVersion, "version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if c.ShowVersion {
		return c, nil
	}
	if len(envErrs) > 0 {
		return nil, errors.Join(envErrs...)
	}

	// A service URL counts as explicitly set when given as a flag or env var.
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	for flagName, env := range map[string]string{"metadata-url": "EDGEX_METADATA_URL", "data-url": "EDGEX_DATA_URL", "command-url": "EDGEX_COMMAND_URL"} {
		if strings.TrimSpace(getenv(env)) != "" {
			explicit[flagName] = true
		}
	}
	if c.GatewayURL != "" {
		var conflicts []string
		for _, name := range []string{"metadata-url", "data-url", "command-url"} {
			if explicit[name] {
				conflicts = append(conflicts, "--"+name)
			}
		}
		if len(conflicts) > 0 {
			return nil, fmt.Errorf("--gateway-url cannot be combined with %s: in gateway mode all services are reached through the gateway", strings.Join(conflicts, ", "))
		}
	}

	envToken := strings.TrimSpace(getenv("EDGEX_TOKEN"))
	switch {
	case envToken != "" && tokenFile != "":
		return nil, errors.New("both EDGEX_TOKEN and --token-file/EDGEX_TOKEN_FILE are set; use only one token source")
	case envToken != "":
		c.Token = Secret(envToken)
	case tokenFile != "":
		b, err := os.ReadFile(tokenFile) // #nosec G304 -- path is operator-supplied configuration
		if err != nil {
			return nil, fmt.Errorf("reading token file: %w", err)
		}
		t := strings.TrimSpace(string(b))
		if t == "" {
			return nil, fmt.Errorf("token file %s is empty", tokenFile)
		}
		c.Token = Secret(t)
	}

	lvl, err := parseLevel(logLevel)
	if err != nil {
		return nil, err
	}
	c.LogLevel = lvl

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks value ranges and normalizes URLs (trailing slashes removed).
func (c *Config) Validate() error {
	var errs []error
	check := func(name string, v *string, required bool) {
		if *v == "" && !required {
			return
		}
		u, err := url.Parse(*v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.User != nil {
			errs = append(errs, fmt.Errorf("%s must be an absolute http(s) URL without credentials or query, got %q", name, *v))
			return
		}
		*v = strings.TrimRight(*v, "/")
	}
	if c.GatewayMode() {
		check("gateway URL", &c.GatewayURL, true)
	} else {
		check("metadata URL", &c.MetadataURL, true)
		check("data URL", &c.DataURL, true)
		check("command URL", &c.CommandURL, true)
	}
	if c.Timeout < time.Second || c.Timeout > 5*time.Minute {
		errs = append(errs, fmt.Errorf("timeout must be between 1s and 5m, got %s", c.Timeout))
	}
	if c.MaxResults < 1 || c.MaxResults > MaxResultsLimit {
		errs = append(errs, fmt.Errorf("max-results must be in the range 1..%d, got %d", MaxResultsLimit, c.MaxResults))
	}
	switch c.Transport {
	case TransportStdio:
	case TransportHTTP:
		if _, _, err := net.SplitHostPort(c.HTTPAddr); err != nil {
			errs = append(errs, fmt.Errorf("http-addr must be host:port, got %q", c.HTTPAddr))
		}
	default:
		errs = append(errs, fmt.Errorf("transport must be %q or %q, got %q", TransportStdio, TransportHTTP, c.Transport))
	}
	return errors.Join(errs...)
}

func parseLevel(s string) (slog.Level, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(strings.TrimSpace(s)))); err != nil {
		return 0, fmt.Errorf("log-level must be debug, info, warn or error, got %q", s)
	}
	return l, nil
}
