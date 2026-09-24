package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(nil, envFrom(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if c.GatewayMode() {
		t.Error("expected direct mode")
	}
	if c.MetadataURL != DefaultMetadataURL || c.DataURL != DefaultDataURL || c.CommandURL != DefaultCommandURL {
		t.Errorf("unexpected URLs: %s %s %s", c.MetadataURL, c.DataURL, c.CommandURL)
	}
	if c.Transport != TransportStdio || c.Timeout != 10*time.Second || c.EnableWrites || c.DisableDeviceReads {
		t.Errorf("unexpected defaults: %+v", c)
	}
	if c.MaxResults != DefaultMaxResults || c.HTTPAddr != DefaultHTTPAddr {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		env   map[string]string
		check func(*Config) error
	}{
		{
			name: "flag overrides env",
			args: []string{"--data-url", "http://192.0.2.20:59880"},
			env:  map[string]string{"EDGEX_DATA_URL": "http://192.0.2.10:59880"},
			check: func(c *Config) error {
				if c.DataURL != "http://192.0.2.20:59880" {
					return fmt.Errorf("data URL = %s", c.DataURL)
				}
				return nil
			},
		},
		{
			name: "env overrides default",
			env:  map[string]string{"EDGEX_TIMEOUT": "3s"},
			check: func(c *Config) error {
				if c.Timeout != 3*time.Second {
					return fmt.Errorf("timeout = %s", c.Timeout)
				}
				return nil
			},
		},
		{
			name: "env booleans",
			env:  map[string]string{"EDGEX_ENABLE_WRITES": "true", "EDGEX_DISABLE_DEVICE_READS": "1"},
			check: func(c *Config) error {
				if !c.EnableWrites || !c.DisableDeviceReads {
					return fmt.Errorf("booleans not applied: %+v", c)
				}
				return nil
			},
		},
		{
			name: "trailing slash trimmed",
			args: []string{"--metadata-url", "http://192.0.2.10:59881/"},
			check: func(c *Config) error {
				if c.MetadataURL != "http://192.0.2.10:59881" {
					return fmt.Errorf("metadata URL = %s", c.MetadataURL)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(tt.args, envFrom(tt.env), io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if err := tt.check(c); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestGatewayMode(t *testing.T) {
	c, err := Load([]string{"--gateway-url", "https://192.0.2.10:8443/"}, envFrom(map[string]string{"EDGEX_TOKEN": "jwt-value"}), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !c.GatewayMode() || c.GatewayURL != "https://192.0.2.10:8443" {
		t.Errorf("gateway not applied: %q", c.GatewayURL)
	}
	if c.Token != "jwt-value" {
		t.Error("token not applied")
	}
	if len(c.Warnings()) != 0 {
		t.Errorf("unexpected warnings: %v", c.Warnings())
	}
}

func TestInvalidConfigurations(t *testing.T) {
	dir := t.TempDir()
	emptyFile := filepath.Join(dir, "empty")
	if err := os.WriteFile(emptyFile, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		wantErr string
	}{
		{"gateway with data-url flag", []string{"--gateway-url", "https://192.0.2.10:8443", "--data-url", "http://192.0.2.10:59880"}, nil, "--data-url"},
		{"gateway with env service url", []string{"--gateway-url", "https://192.0.2.10:8443"}, map[string]string{"EDGEX_METADATA_URL": "http://192.0.2.10:59881"}, "--metadata-url"},
		{"relative URL", []string{"--metadata-url", "localhost:59881"}, nil, "metadata URL must be an absolute http(s) URL"},
		{"URL with credentials", []string{"--data-url", "http://user:pw@192.0.2.10:59880"}, nil, "without credentials"},
		{"max results too big", []string{"--max-results", "5000"}, nil, "1..1024"},
		{"max results zero", []string{"--max-results", "0"}, nil, "1..1024"},
		{"timeout too small", []string{"--timeout", "10ms"}, nil, "between 1s and 5m"},
		{"bad transport", []string{"--transport", "sse"}, nil, "transport must be"},
		{"bad http addr", []string{"--transport", "http", "--http-addr", "8080"}, nil, "host:port"},
		{"bad log level", []string{"--log-level", "loud"}, nil, "log-level"},
		{"bad env bool", nil, map[string]string{"EDGEX_ENABLE_WRITES": "maybe"}, "EDGEX_ENABLE_WRITES"},
		{"bad env duration", nil, map[string]string{"EDGEX_TIMEOUT": "soon"}, "EDGEX_TIMEOUT"},
		{"both token sources", []string{"--token-file", emptyFile}, map[string]string{"EDGEX_TOKEN": "secret-jwt"}, "only one token source"},
		{"empty token file", []string{"--token-file", emptyFile}, nil, "is empty"},
		{"missing token file", []string{"--token-file", filepath.Join(dir, "nope")}, nil, "reading token file"},
		{"positional args", []string{"extra"}, nil, "unexpected arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.args, envFrom(tt.env), io.Discard)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "secret-jwt") {
				t.Error("error leaks the token")
			}
		})
	}
}

func TestTokenFromFileIsTrimmed(t *testing.T) {
	f := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(f, []byte("  header.payload.sig\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load([]string{"--token-file", f}, envFrom(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if string(c.Token) != "header.payload.sig" {
		t.Errorf("token = %q", string(c.Token))
	}
}

func TestSecretNeverPrints(t *testing.T) {
	c := &Config{Token: "super-secret-token"}
	for _, s := range []string{fmt.Sprint(c.Token), fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c)} {
		if strings.Contains(s, "super-secret-token") {
			t.Errorf("secret printed: %s", s)
		}
	}
}

func TestWarnings(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"gateway without token", []string{"--gateway-url", "https://192.0.2.10:8443"}, "401"},
		{"http on all interfaces", []string{"--transport", "http", "--http-addr", "0.0.0.0:8080"}, "no client authentication"},
		{"http on empty host", []string{"--transport", "http", "--http-addr", ":8080"}, "no client authentication"},
		{"http on loopback", []string{"--transport", "http", "--http-addr", "127.0.0.1:8080"}, ""},
		{"http on localhost", []string{"--transport", "http", "--http-addr", "localhost:8080"}, ""},
		{"http on ipv6 loopback", []string{"--transport", "http", "--http-addr", "[::1]:8080"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(tt.args, envFrom(nil), io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			w := strings.Join(c.Warnings(), "\n")
			if tt.want == "" && w != "" {
				t.Errorf("unexpected warnings: %s", w)
			}
			if tt.want != "" && !strings.Contains(w, tt.want) {
				t.Errorf("warnings %q do not contain %q", w, tt.want)
			}
		})
	}
}

func TestVersionFlagSkipsValidation(t *testing.T) {
	c, err := Load([]string{"--version", "--max-results", "0"}, envFrom(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !c.ShowVersion {
		t.Error("ShowVersion not set")
	}
}
