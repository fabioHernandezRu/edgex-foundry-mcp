package edgex_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex/edgextest"
)

func newClient(t *testing.T, f *edgextest.Fake, mut func(*edgex.Options)) *edgex.Client {
	t.Helper()
	o := edgex.Options{
		MetadataURL: f.Metadata.URL,
		DataURL:     f.Data.URL,
		CommandURL:  f.Command.URL,
		Timeout:     2 * time.Second,
		MaxResults:  100,
	}
	if mut != nil {
		mut(&o)
	}
	c, err := edgex.New(o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// exerciseAll calls every client method once.
func exerciseAll(t *testing.T, c *edgex.Client) {
	t.Helper()
	ctx := context.Background()
	calls := map[string]func() error{
		"ping":     func() error { _, err := c.Ping(ctx, edgex.Metadata); return err },
		"version":  func() error { _, err := c.Version(ctx, edgex.Data); return err },
		"services": func() error { _, _, err := c.DeviceServices(ctx, nil, edgex.Page{}); return err },
		"devices":  func() error { _, _, err := c.Devices(ctx, edgex.DeviceFilter{}, edgex.Page{}); return err },
		"devicesBySvc": func() error {
			_, _, err := c.Devices(ctx, edgex.DeviceFilter{Service: "device-virtual"}, edgex.Page{})
			return err
		},
		"devicesByProf": func() error {
			_, _, err := c.Devices(ctx, edgex.DeviceFilter{Profile: "Random-Integer-Device"}, edgex.Page{})
			return err
		},
		"device":   func() error { _, err := c.Device(ctx, "Random-Integer-Device"); return err },
		"profiles": func() error { _, _, err := c.DeviceProfiles(ctx, edgex.ProfileFilter{}, edgex.Page{}); return err },
		"profilesByMfr": func() error {
			_, _, err := c.DeviceProfiles(ctx, edgex.ProfileFilter{Manufacturer: "Example Corp"}, edgex.Page{})
			return err
		},
		"profilesByModel": func() error {
			_, _, err := c.DeviceProfiles(ctx, edgex.ProfileFilter{Model: "EX-1"}, edgex.Page{})
			return err
		},
		"profilesByBoth": func() error {
			_, _, err := c.DeviceProfiles(ctx, edgex.ProfileFilter{Manufacturer: "Example Corp", Model: "EX-1"}, edgex.Page{})
			return err
		},
		"profile": func() error { _, err := c.DeviceProfile(ctx, "Random-Integer-Device"); return err },
		"readings": func() error {
			_, _, err := c.Readings(ctx, edgex.ReadingQuery{Device: "Random-Integer-Device"}, edgex.Page{})
			return err
		},
		"readingsRange": func() error {
			_, _, err := c.Readings(ctx, edgex.ReadingQuery{Device: "Random-Integer-Device", Resource: "Int8", HasRange: true, StartNs: 1, EndNs: 2}, edgex.Page{})
			return err
		},
		"eventCount":   func() error { _, err := c.EventCount(ctx, "Random-Integer-Device"); return err },
		"readingCount": func() error { _, err := c.ReadingCount(ctx, "Random-Integer-Device"); return err },
		"commands":     func() error { _, err := c.DeviceCommands(ctx, "Random-Integer-Device"); return err },
		"allCommands":  func() error { _, _, err := c.AllDeviceCommands(ctx, edgex.Page{}); return err },
		"readCommand":  func() error { _, err := c.ReadCommand(ctx, "Random-Integer-Device", "Int8"); return err },
	}
	for name, call := range calls {
		if err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestReadMethodsOnlyIssueGET(t *testing.T) {
	f := edgextest.New(t)
	exerciseAll(t, newClient(t, f, nil))
	reqs := f.Requests()
	if len(reqs) < 19 {
		t.Fatalf("expected at least 19 requests, got %d", len(reqs))
	}
	for _, r := range reqs {
		if r.Method != http.MethodGet {
			t.Errorf("%s %s %s: only GET is allowed", r.Service, r.Method, r.Path)
		}
		if l := r.Query["limit"]; len(l) > 0 && l[0] == "-1" {
			t.Errorf("%s %s: limit=-1 sent", r.Service, r.Path)
		}
	}
}

func TestGatewayModeRoutesByPrefixAndSendsToken(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, func(o *edgex.Options) {
		o.MetadataURL, o.DataURL, o.CommandURL = "", "", ""
		o.GatewayURL = f.Gateway.URL + "/"
		o.Token = "test-jwt"
	})
	if got := c.BaseURL(edgex.Data); got != f.Gateway.URL+"/core-data" {
		t.Errorf("data base URL = %s", got)
	}
	exerciseAll(t, c)
	for _, r := range f.Requests() {
		if r.Auth != "Bearer test-jwt" {
			t.Errorf("%s %s: Authorization = %q", r.Service, r.Path, r.Auth)
		}
	}
}

func TestNoAuthorizationWithoutToken(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	if _, err := c.Ping(context.Background(), edgex.Command); err != nil {
		t.Fatal(err)
	}
	r, _ := f.LastRequest("core-command")
	if r.Auth != "" {
		t.Errorf("unexpected Authorization header %q", r.Auth)
	}
}

func TestPathEscaping(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	_, err := c.Device(context.Background(), "Line 1/Sensor A")
	if !edgex.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
	r, _ := f.LastRequest("core-metadata")
	if r.Path != "/api/v3/device/name/Line%201%2FSensor%20A" {
		t.Errorf("path = %s", r.Path)
	}
}

func TestPaginationAndLabels(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	if _, _, err := c.Devices(context.Background(), edgex.DeviceFilter{Labels: []string{"site-a", " floor-2 ", ""}}, edgex.Page{Offset: 0, Limit: 10}); err != nil {
		t.Fatal(err)
	}
	r, _ := f.LastRequest("core-metadata")
	if got := r.Query["labels"]; len(got) != 1 || got[0] != "site-a,floor-2" {
		t.Errorf("labels = %v", got)
	}
	if r.Query["limit"][0] != "10" || r.Query["offset"][0] != "0" {
		t.Errorf("query = %v", r.Query)
	}
}

func TestClampLimit(t *testing.T) {
	c, err := edgex.New(edgex.Options{MetadataURL: "http://192.0.2.1", DataURL: "http://192.0.2.1", CommandURL: "http://192.0.2.1", Timeout: time.Second, MaxResults: 100})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ limit, def, want int }{
		{5000, 20, 100},
		{0, 20, 20},
		{-1, 10, 10},
		{50, 20, 50},
		{0, 0, edgex.DefaultLimit},
	}
	for _, tt := range tests {
		if got := c.ClampLimit(tt.limit, tt.def); got != tt.want {
			t.Errorf("ClampLimit(%d,%d) = %d, want %d", tt.limit, tt.def, got, tt.want)
		}
	}
}

func TestOversizedLimitIsClampedOnTheWire(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	if _, _, err := c.Readings(context.Background(), edgex.ReadingQuery{Device: "Random-Integer-Device"}, edgex.Page{Limit: 5000, Offset: -3}); err != nil {
		t.Fatal(err)
	}
	r, _ := f.LastRequest("core-data")
	if r.Query["limit"][0] != "100" || r.Query["offset"][0] != "0" {
		t.Errorf("query = %v", r.Query)
	}
}

func TestReadCommandSafeFlags(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	ev, err := c.ReadCommand(context.Background(), "Random-Integer-Device", "Int8")
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Readings) != 1 || ev.Readings[0].ResourceName != "Int8" {
		t.Errorf("unexpected event: %+v", ev)
	}
	r, _ := f.LastRequest("core-command")
	if r.Path != "/api/v3/device/name/Random-Integer-Device/Int8" {
		t.Errorf("path = %s", r.Path)
	}
	want := map[string]string{"ds-pushevent": "false", "ds-returnevent": "true", "ds-regexcmd": "false"}
	for k, v := range want {
		if got := r.Query[k]; len(got) != 1 || got[0] != v {
			t.Errorf("%s = %v, want %s", k, got, v)
		}
	}
}

func TestCountDecoding(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	n, err := c.ReadingCount(context.Background(), "Random-Integer-Device")
	if err != nil || n != 48 {
		t.Errorf("count = %d, err = %v", n, err)
	}
}

func TestErrorDecoding(t *testing.T) {
	f := edgextest.New(t)
	f.Handle("core-data", "/api/v3/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("request timeout"))
	})
	f.Handle("core-command", "/api/v3/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})
	f.Handle("core-metadata", "/api/v3/version", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 1000)))
	})
	c := newClient(t, f, func(o *edgex.Options) { o.Token = "secret-token-value" })
	ctx := context.Background()

	_, err := c.Device(ctx, "missing")
	var ae *edgex.APIError
	if !errors.As(err, &ae) || ae.Status != 404 || ae.Service != edgex.Metadata || !strings.Contains(ae.Message, "no device with name 'missing' found") {
		t.Errorf("404: %#v", err)
	}
	if !edgex.IsNotFound(err) {
		t.Error("IsNotFound false")
	}

	_, err = c.Ping(ctx, edgex.Data)
	if edgex.StatusOf(err) != 503 || !strings.Contains(err.Error(), "request timeout") {
		t.Errorf("503: %v", err)
	}

	_, err = c.Ping(ctx, edgex.Command)
	if !edgex.IsUnauthorized(err) {
		t.Errorf("401: %v", err)
	}

	_, err = c.Version(ctx, edgex.Metadata)
	if !errors.As(err, &ae) || len(ae.Message) > 210 {
		t.Errorf("long body not truncated: %d bytes", len(ae.Message))
	}

	for _, e := range []error{err} {
		if strings.Contains(e.Error(), "secret-token-value") {
			t.Error("error leaks the token")
		}
	}
}

func TestTimeout(t *testing.T) {
	f := edgextest.New(t)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	f.Handle("core-data", "/api/v3/ping", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	c := newClient(t, f, func(o *edgex.Options) { o.Timeout = 200 * time.Millisecond })
	start := time.Now()
	_, err := c.Ping(context.Background(), edgex.Data)
	var ae *edgex.APIError
	if !errors.As(err, &ae) || !ae.Timeout {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Errorf("timeout took %s", el)
	}
}

func TestUnreachableNamesServiceAndURL(t *testing.T) {
	c, err := edgex.New(edgex.Options{MetadataURL: "http://127.0.0.1:1", DataURL: "http://127.0.0.1:1", CommandURL: "http://127.0.0.1:1", Timeout: time.Second, MaxResults: 10})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Ping(context.Background(), edgex.Data)
	if err == nil || !strings.Contains(err.Error(), "core-data") || !strings.Contains(err.Error(), "http://127.0.0.1:1") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvalidOptions(t *testing.T) {
	if _, err := edgex.New(edgex.Options{MaxResults: 10}); err == nil {
		t.Error("expected error for zero timeout")
	}
	if _, err := edgex.New(edgex.Options{Timeout: time.Second}); err == nil {
		t.Error("expected error for zero max results")
	}
	if _, err := edgex.New(edgex.Options{Timeout: time.Second, MaxResults: 1, CAFile: "/nonexistent/ca.pem"}); err == nil {
		t.Error("expected error for missing CA file")
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	f := edgextest.New(t)
	f.Handle("core-data", "/api/v3/ping", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://192.0.2.99/steal", http.StatusMovedPermanently)
	})
	c, err := edgex.New(edgex.Options{MetadataURL: f.Metadata.URL, DataURL: f.Data.URL, CommandURL: f.Command.URL, Timeout: 2 * time.Second, MaxResults: 10, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Ping(context.Background(), edgex.Data)
	if edgex.StatusOf(err) != http.StatusMovedPermanently {
		t.Errorf("expected the 301 to be returned, not followed: %v", err)
	}
}

func TestDotSegmentsRejected(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	before := len(f.Requests())
	for _, name := range []string{"..", ".", ""} {
		if _, err := c.Device(context.Background(), name); !errors.Is(err, edgex.ErrInvalidName) {
			t.Errorf("%q: err = %v", name, err)
		}
	}
	if _, _, err := c.Readings(context.Background(), edgex.ReadingQuery{Device: "d", Resource: ".."}, edgex.Page{}); !errors.Is(err, edgex.ErrInvalidName) {
		t.Errorf("resource '..': err = %v", err)
	}
	if len(f.Requests()) != before {
		t.Error("invalid names reached EdgeX")
	}
}

func TestErrorMessagesScrubURLCredentials(t *testing.T) {
	f := edgextest.New(t)
	f.Handle("core-command", "/api/v3/ping", func(w http.ResponseWriter, _ *http.Request) {
		edgextest.WriteError(w, http.StatusLocked, "dial opc.tcp://operator:example-pw@192.0.2.10:4840 failed")
	})
	c := newClient(t, f, nil)
	_, err := c.Ping(context.Background(), edgex.Command)
	if err == nil || strings.Contains(err.Error(), "example-pw") || !strings.Contains(err.Error(), "opc.tcp://***REDACTED***@192.0.2.10") {
		t.Errorf("err = %v", err)
	}
}

func TestScrubURLUserinfo(t *testing.T) {
	tests := map[string]string{
		"tcp://u:p@192.0.2.1:1883":          "tcp://***REDACTED***@192.0.2.1:1883",
		"https://192.0.2.1/path?a=b":        "https://192.0.2.1/path?a=b",
		"see mqtt://reader@host and x":      "see mqtt://***REDACTED***@host and x",
		"plain text with user@host.example": "plain text with user@host.example",
	}
	for in, want := range tests {
		if got := edgex.ScrubURLUserinfo(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestSetCommandRequest(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	if err := c.SetCommand(context.Background(), "Random-Integer-Device", "Int8", map[string]string{"Int8": "42"}); err != nil {
		t.Fatal(err)
	}
	r, _ := f.LastRequest("core-command")
	if r.Method != http.MethodPut || r.Path != "/api/v3/device/name/Random-Integer-Device/Int8" {
		t.Errorf("request = %s %s", r.Method, r.Path)
	}
	if r.Body != `{"Int8":"42"}` || r.ContentType != "application/json" {
		t.Errorf("body = %s, content type = %s", r.Body, r.ContentType)
	}
	if err := c.SetCommand(context.Background(), "Random-Integer-Device", "Int8", nil); err == nil {
		t.Error("empty values must be rejected client-side")
	}
}

func TestSetCommandErrors(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	ctx := context.Background()
	tests := []struct {
		device, command string
		status          int
	}{
		{"Random-Integer-Device", "NoSuchCommand", 404},
		{"Example-MQTT-Sensor", "Temperature", 423},
		{"Random-Float-Device", "Float64", 0},
	}
	for _, tt := range tests {
		err := c.SetCommand(ctx, tt.device, tt.command, map[string]string{"x": "1"})
		if got := edgex.StatusOf(err); got != tt.status {
			t.Errorf("%s/%s: status %d, want %d (err %v)", tt.device, tt.command, got, tt.status, err)
		}
	}
}

func TestUpdateDeviceState(t *testing.T) {
	f := edgextest.New(t)
	c := newClient(t, f, nil)
	ctx := context.Background()
	if err := c.UpdateDeviceState(ctx, "Random-Integer-Device", edgex.AdminStateField, "LOCKED"); err != nil {
		t.Fatal(err)
	}
	r, _ := f.LastRequest("core-metadata")
	want := `[{"apiVersion":"v3","device":{"name":"Random-Integer-Device","adminState":"LOCKED"}}]`
	if r.Method != http.MethodPatch || r.Path != "/api/v3/device" || r.Body != want || r.ContentType != "application/json" {
		t.Errorf("request = %s %s %s (%s)", r.Method, r.Path, r.Body, r.ContentType)
	}
	d, err := c.Device(ctx, "Random-Integer-Device")
	if err != nil || d.AdminState != "LOCKED" {
		t.Errorf("state not applied: %+v %v", d, err)
	}

	err = c.UpdateDeviceState(ctx, "No-Such-Device", edgex.OperatingStateField, "UP")
	if !edgex.IsNotFound(err) || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("207 item failure: %v", err)
	}
	if err := c.UpdateDeviceState(ctx, "Random-Integer-Device", edgex.OperatingStateField, "BROKEN"); edgex.StatusOf(err) != 400 {
		t.Errorf("invalid state: %v", err)
	}
	if err := c.UpdateDeviceState(ctx, "Random-Integer-Device", "protocols", "x"); err == nil {
		t.Error("unknown field must be rejected")
	}
}
