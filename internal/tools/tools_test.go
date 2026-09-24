package tools_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex/edgextest"
	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/tools"
)

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type env struct {
	fake    *edgextest.Fake
	session *mcp.ClientSession
	names   []string
	logs    *syncBuffer
}

type setup struct {
	opts       tools.Options
	token      string
	maxResults int
}

func newEnv(t *testing.T, s setup) *env {
	t.Helper()
	f := edgextest.New(t)
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if s.maxResults == 0 {
		s.maxResults = 100
	}
	c, err := edgex.New(edgex.Options{
		MetadataURL: f.Metadata.URL, DataURL: f.Data.URL, CommandURL: f.Command.URL,
		Token: s.token, Timeout: 2 * time.Second, MaxResults: s.maxResults, Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "edgex-foundry-mcp", Version: "test"},
		&mcp.ServerOptions{Instructions: tools.Instructions, Logger: logger})
	s.opts.Logger = logger
	names, err := tools.Register(server, c, s.opts)
	if err != nil {
		t.Fatal(err)
	}
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); _ = ss.Close() })
	return &env{fake: f, session: cs, names: names, logs: logs}
}

// call invokes a tool and returns the result and its structured content as JSON.
func (e *env) call(t *testing.T, name string, args map[string]any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	res, err := e.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	var out map[string]any
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(b, &out)
	}
	return res, out
}

func (e *env) mustCall(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	res, out := e.call(t, name, args)
	if res.IsError {
		t.Fatalf("%s returned tool error: %s", name, text(res))
	}
	return out
}

func (e *env) mustFail(t *testing.T, name string, args map[string]any, want string) {
	t.Helper()
	res, _ := e.call(t, name, args)
	if !res.IsError {
		t.Fatalf("%s: expected tool error containing %q", name, want)
	}
	if !strings.Contains(text(res), want) {
		t.Errorf("%s: error %q does not contain %q", name, text(res), want)
	}
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func list(out map[string]any, key string) []map[string]any {
	raw, _ := out[key].([]any)
	res := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, _ := r.(map[string]any)
		res = append(res, m)
	}
	return res
}

var allWriteTools = []string{"set_device_command", "set_device_admin_state", "set_device_operating_state"}

var allReadTools = []string{
	"system_health", "list_device_services", "list_devices", "get_device",
	"list_device_profiles", "get_device_profile", "get_latest_readings",
	"query_readings", "device_data_stats", "list_device_commands", "read_device_command",
}

func TestDefaultToolSetIsReadOnly(t *testing.T) {
	e := newEnv(t, setup{})
	res, err := e.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s lacks readOnlyHint", tool.Name)
		}
		if tool.Description == "" || tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Errorf("%s: missing description or schemas", tool.Name)
		}
	}
	slices.Sort(got)
	want := slices.Clone(allReadTools)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("tools = %v, want %v", got, want)
	}
	if !strings.Contains(e.logs.String(), "read-only mode") {
		t.Error("startup log does not state read-only mode")
	}
}

func TestEnableWritesGate(t *testing.T) {
	e := newEnv(t, setup{opts: tools.Options{EnableWrites: true}})
	if !strings.Contains(e.logs.String(), "write tools are ENABLED") {
		t.Error("missing write warning")
	}
	want := append(slices.Clone(allReadTools), allWriteTools...)
	got := slices.Clone(e.names)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("registered %v, want %v", got, want)
	}
	res, err := e.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if slices.Contains(allWriteTools, tool.Name) {
			if tool.Annotations == nil || tool.Annotations.ReadOnlyHint {
				t.Errorf("%s must not be read-only", tool.Name)
			}
			if err := tools.ValidateWriteToolForTest(tool); err != nil {
				t.Error(err)
			}
		}
	}
}

func TestDisableDeviceReads(t *testing.T) {
	e := newEnv(t, setup{opts: tools.Options{DisableDeviceReads: true}})
	if slices.Contains(e.names, "read_device_command") {
		t.Error("read_device_command registered despite --disable-device-reads")
	}
	if len(e.names) != len(allReadTools)-1 {
		t.Errorf("registered %v", e.names)
	}
}

func TestValidateWriteTool(t *testing.T) {
	tests := []struct {
		name string
		tool *mcp.Tool
		ok   bool
	}{
		{"ok hardware", &mcp.Tool{Name: "set_x", Description: "This affects physical hardware.", Annotations: &mcp.ToolAnnotations{}}, true},
		{"ok metadata", &mcp.Tool{Name: "lock_x", Description: "This modifies EdgeX metadata.", Annotations: &mcp.ToolAnnotations{}}, true},
		{"missing wording", &mcp.Tool{Name: "set_x", Description: "Sets a value.", Annotations: &mcp.ToolAnnotations{}}, false},
		{"read-only hint", &mcp.Tool{Name: "set_x", Description: "This affects physical hardware.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, false},
		{"no annotations", &mcp.Tool{Name: "set_x", Description: "This affects physical hardware."}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.ValidateWriteToolForTest(tt.tool)
			if (err == nil) != tt.ok {
				t.Errorf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
	for _, tool := range tools.WriteToolsForTest() {
		if err := tools.ValidateWriteToolForTest(tool); err != nil {
			t.Error(err)
		}
	}
}

func TestSystemHealth(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "system_health", nil)
	if out["healthy"] != true {
		t.Errorf("healthy = %v", out["healthy"])
	}
	for _, s := range list(out, "services") {
		if s["reachable"] != true || s["version"] != edgextest.Version || s["apiVersion"] != "v3" {
			t.Errorf("service %v", s)
		}
	}
}

func TestSystemHealthPartialFailure(t *testing.T) {
	e := newEnv(t, setup{})
	e.fake.Command.Close()
	out := e.mustCall(t, "system_health", nil)
	if out["healthy"] != false {
		t.Fatal("expected healthy=false")
	}
	for _, s := range list(out, "services") {
		isCmd := s["service"] == "core-command"
		if isCmd && (s["reachable"] != false || s["error"] == nil) {
			t.Errorf("core-command: %v", s)
		}
		if !isCmd && s["reachable"] != true {
			t.Errorf("%v should be reachable", s["service"])
		}
	}
}

func TestListDeviceServices(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "list_device_services", nil)
	svcs := list(out, "services")
	if len(svcs) != 2 || svcs[0]["name"] != "device-virtual" || svcs[0]["adminState"] != "UNLOCKED" || svcs[0]["baseAddress"] == "" {
		t.Fatalf("services = %v", svcs)
	}
	props, _ := svcs[1]["properties"].(map[string]any)
	if props["BrokerPassword"] != tools.Redacted {
		t.Errorf("service properties not redacted: %v", props)
	}
}

func TestListDevices(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "list_devices", map[string]any{"service": "device-virtual"})
	if r, _ := e.fake.LastRequest("core-metadata"); r.Path != "/api/v3/device/service/name/device-virtual" {
		t.Errorf("path = %s", r.Path)
	}
	devs := list(out, "devices")
	if len(devs) != 3 {
		t.Fatalf("devices = %v", devs)
	}
	for _, d := range devs {
		if _, ok := d["protocols"]; ok {
			t.Error("list_devices must not return protocols")
		}
	}

	out = e.mustCall(t, "list_devices", map[string]any{"labels": []string{"site-a", "floor-2"}})
	if devs := list(out, "devices"); len(devs) != 1 || devs[0]["name"] != "Example-MQTT-Sensor" {
		t.Errorf("labels filter: %v", devs)
	}

	before := len(e.fake.Requests())
	e.mustFail(t, "list_devices", map[string]any{"service": "device-virtual", "profile": "x"}, "only one filter")
	if len(e.fake.Requests()) != before {
		t.Error("conflicting filters must not reach EdgeX")
	}

	out = e.mustCall(t, "list_devices", map[string]any{"service": "no-such-service"})
	if out["count"] != float64(0) || !strings.Contains(out["hint"].(string), "may not exist") {
		t.Errorf("empty result: %v", out)
	}
}

func TestPaginationCapAndNextOffset(t *testing.T) {
	e := newEnv(t, setup{maxResults: 2})
	out := e.mustCall(t, "list_devices", map[string]any{"limit": 500})
	if r, _ := e.fake.LastRequest("core-metadata"); r.Query["limit"][0] != "2" {
		t.Errorf("limit sent = %v", r.Query["limit"])
	}
	if out["totalCount"] != float64(4) || out["count"] != float64(2) || out["nextOffset"] != float64(2) {
		t.Errorf("envelope = %v", out)
	}
	out = e.mustCall(t, "list_devices", map[string]any{"offset": 2, "limit": 2})
	if _, ok := out["nextOffset"]; ok {
		t.Errorf("last page must not have nextOffset: %v", out)
	}
}

func TestGetDeviceRedactsCredentials(t *testing.T) {
	e := newEnv(t, setup{})
	res, out := e.call(t, "get_device", map[string]any{"name": "Example-MQTT-Sensor"})
	if res.IsError {
		t.Fatal(text(res))
	}
	mqtt := out["protocols"].(map[string]any)["mqtt"].(map[string]any)
	if mqtt["Password"] != tools.Redacted || mqtt["Host"] != "192.0.2.10" || mqtt["Username"] != "reader" {
		t.Errorf("protocols = %v", mqtt)
	}
	sec := out["properties"].(map[string]any)["opcua"].(map[string]any)["security"].(map[string]any)
	if sec["PrivateKeyPath"] != tools.Redacted {
		t.Errorf("nested property not redacted: %v", sec)
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), "example-pass") || strings.Contains(string(raw), "/keys/example.pem") {
		t.Error("credential leaked in tool result")
	}
	if out["operatingState"] != "DOWN" || out["profile"] != "Example-Sensor-Profile" {
		t.Errorf("device = %v", out)
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	e := newEnv(t, setup{})
	e.mustFail(t, "get_device", map[string]any{"name": "nope"}, "list_devices")
}

func TestGetDeviceAutoEvents(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "get_device", map[string]any{"name": "Random-Integer-Device"})
	aes := list(out, "autoEvents")
	if len(aes) != 2 || aes[0]["source"] != "Int8" || aes[0]["interval"] != "15s" {
		t.Errorf("autoEvents = %v", aes)
	}
	if out["created"] != "2024-09-22T10:13:20Z" {
		t.Errorf("created = %v", out["created"])
	}
}

func TestListDeviceProfiles(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "list_device_profiles", nil)
	if r, _ := e.fake.LastRequest("core-metadata"); r.Path != "/api/v3/deviceprofile/basicinfo/all" {
		t.Errorf("path = %s", r.Path)
	}
	if ps := list(out, "profiles"); len(ps) != 3 || ps[0]["model"] != "Device-Virtual-01" {
		t.Errorf("profiles = %v", ps)
	}
	out = e.mustCall(t, "list_device_profiles", map[string]any{"manufacturer": "Example Corp", "model": "EX-1"})
	if r, _ := e.fake.LastRequest("core-metadata"); r.Path != "/api/v3/deviceprofile/manufacturer/Example%20Corp/model/EX-1" {
		t.Errorf("path = %s", r.Path)
	}
	if ps := list(out, "profiles"); len(ps) != 1 || ps[0]["name"] != "Example-Sensor-Profile" {
		t.Errorf("profiles = %v", ps)
	}
	e.mustFail(t, "list_device_profiles", map[string]any{"labels": []string{"a"}, "model": "EX-1"}, "cannot be combined")
}

func TestGetDeviceProfile(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "get_device_profile", map[string]any{"name": "Random-Integer-Device"})
	res := list(out, "resources")
	if len(res) != 2 || res[0]["name"] != "Int8" || res[0]["valueType"] != "Int8" || res[0]["readWrite"] != "RW" || res[0]["minimum"] != float64(-100) {
		t.Errorf("resources = %v", res)
	}
	cmds := list(out, "commands")
	if len(cmds) != 1 || cmds[0]["name"] != "WriteInt8Value" {
		t.Errorf("commands = %v", cmds)
	}

	out = e.mustCall(t, "get_device_profile", map[string]any{"name": "Random-Integer-Device", "includeHidden": true})
	hidden := false
	for _, r := range list(out, "resources") {
		if r["name"] == "EnableRandomization_Int8" && r["hidden"] == true {
			hidden = true
		}
	}
	if !hidden {
		t.Error("hidden resource missing with includeHidden")
	}

	out = e.mustCall(t, "get_device_profile", map[string]any{"name": "Example-Sensor-Profile"})
	attrs := list(out, "resources")[0]["attributes"].(map[string]any)
	if attrs["authToken"] != tools.Redacted || attrs["topic"] != "sensors/temp" {
		t.Errorf("attributes = %v", attrs)
	}
	e.mustFail(t, "get_device_profile", map[string]any{"name": "missing"}, "list_device_profiles")
}

func TestGetLatestReadings(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "get_latest_readings", map[string]any{"device": "Random-Integer-Device", "resource": "Int8", "limit": 2})
	r, _ := e.fake.LastRequest("core-data")
	if r.Path != "/api/v3/reading/device/name/Random-Integer-Device/resourceName/Int8" || r.Query["limit"][0] != "2" {
		t.Errorf("request = %+v", r)
	}
	rs := list(out, "readings")
	if len(rs) != 2 || rs[0]["value"] != "75" || rs[0]["time"] != "2024-09-24T10:00:03Z" || rs[0]["resource"] != "Int8" {
		t.Errorf("readings = %v", rs)
	}
	if out["nextOffset"] != float64(2) {
		t.Errorf("nextOffset = %v", out["nextOffset"])
	}

	out = e.mustCall(t, "get_latest_readings", map[string]any{"device": "Unknown-Device"})
	if out["count"] != float64(0) || out["hint"] == nil {
		t.Errorf("empty: %v", out)
	}
}

func TestBinaryReadingsAreSummarized(t *testing.T) {
	e := newEnv(t, setup{})
	res, out := e.call(t, "get_latest_readings", map[string]any{"device": "Random-Binary-Device"})
	if res.IsError {
		t.Fatal(text(res))
	}
	rs := list(out, "readings")
	if len(rs) != 1 || rs[0]["binarySize"] != float64(40*1024) || rs[0]["mediaType"] != "image/jpeg" {
		t.Errorf("binary = %v", rs)
	}
	if raw, _ := json.Marshal(res); len(raw) > 4096 {
		t.Errorf("binary payload leaked into result (%d bytes)", len(raw))
	}
}

func TestQueryReadings(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "query_readings", map[string]any{"device": "Random-Float-Device", "resource": "Float64", "window": "1h"})
	r, _ := e.fake.LastRequest("core-data")
	parts := strings.Split(r.Path, "/")
	// /api/v3/reading/device/name/<d>/resourceName/<r>/start/<s>/end/<e>
	if len(parts) != 13 || parts[7] != "resourceName" || parts[8] != "Float64" || parts[9] != "start" || parts[11] != "end" {
		t.Fatalf("path = %s", r.Path)
	}
	start, _ := strconv.ParseInt(parts[10], 10, 64)
	end, _ := strconv.ParseInt(parts[12], 10, 64)
	if end-start != int64(time.Hour) {
		t.Errorf("range = %d ns", end-start)
	}
	if out["start"] == nil || out["end"] == nil || r.Query["limit"][0] != "50" {
		t.Errorf("out = %v, query = %v", out, r.Query)
	}

	out = e.mustCall(t, "query_readings", map[string]any{
		"device": "Random-Integer-Device", "start": "2024-09-24T10:00:00Z", "end": "2024-09-24T10:00:02.5Z",
	})
	if rs := list(out, "readings"); len(rs) != 2 {
		t.Errorf("absolute range readings = %v", rs)
	}

	before := len(e.fake.Requests())
	e.mustFail(t, "query_readings", map[string]any{"device": "d", "start": "2024-09-24T11:00:00Z", "end": "2024-09-24T10:00:00Z"}, "before end")
	e.mustFail(t, "query_readings", map[string]any{"device": "d", "window": "1h", "start": "2024-09-24T10:00:00Z"}, "mutually exclusive")
	e.mustFail(t, "query_readings", map[string]any{"device": "d"}, "window")
	e.mustFail(t, "query_readings", map[string]any{"device": "d", "window": "-5m"}, "positive")
	e.mustFail(t, "query_readings", map[string]any{"device": "d", "start": "yesterday", "end": "2024-09-24T10:00:00Z"}, "RFC 3339")
	if len(e.fake.Requests()) != before {
		t.Error("invalid ranges must not reach EdgeX")
	}
}

func TestDeviceDataStats(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "device_data_stats", map[string]any{"device": "Random-Integer-Device"})
	if out["eventCount"] != float64(12) || out["readingCount"] != float64(48) || out["lastReadingTime"] != "2024-09-24T10:00:03Z" || out["lastReadingResource"] != "Int8" {
		t.Errorf("stats = %v", out)
	}
	out = e.mustCall(t, "device_data_stats", map[string]any{"device": "Quiet-Device"})
	if out["eventCount"] != float64(0) || out["readingCount"] != float64(0) {
		t.Errorf("stats = %v", out)
	}
	if _, ok := out["lastReadingTime"]; ok {
		t.Error("lastReadingTime must be absent without data")
	}
}

func TestListDeviceCommands(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "list_device_commands", map[string]any{"device": "Random-Integer-Device"})
	devs := list(out, "devices")
	if len(devs) != 1 {
		t.Fatalf("devices = %v", devs)
	}
	cmds := list(devs[0], "commands")
	if len(cmds) != 3 || cmds[0]["name"] != "Int8" || cmds[0]["get"] != true || cmds[0]["set"] != true {
		t.Errorf("commands = %v", cmds)
	}
	params := list(cmds[0], "parameters")
	if len(params) != 1 || params[0]["resource"] != "Int8" || params[0]["valueType"] != "Int8" {
		t.Errorf("parameters = %v", params)
	}
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), "edgex-core-command:59882") {
		t.Error("internal command url must be omitted")
	}
	out = e.mustCall(t, "list_device_commands", nil)
	if len(list(out, "devices")) != 3 {
		t.Errorf("all = %v", out)
	}
}

func TestReadDeviceCommand(t *testing.T) {
	e := newEnv(t, setup{})
	out := e.mustCall(t, "read_device_command", map[string]any{"device": "Random-Integer-Device", "command": "Int8"})
	r, _ := e.fake.LastRequest("core-command")
	if r.Path != "/api/v3/device/name/Random-Integer-Device/Int8" {
		t.Errorf("path = %s", r.Path)
	}
	for k, v := range map[string]string{"ds-pushevent": "false", "ds-returnevent": "true", "ds-regexcmd": "false"} {
		if got := r.Query[k]; len(got) != 1 || got[0] != v {
			t.Errorf("%s = %v", k, got)
		}
	}
	if rs := list(out, "readings"); len(rs) != 1 || rs[0]["value"] != "75" {
		t.Errorf("readings = %v", rs)
	}
}

func TestReadDeviceCommandRejectsNonGet(t *testing.T) {
	e := newEnv(t, setup{})
	e.mustFail(t, "read_device_command", map[string]any{"device": "Random-Integer-Device", "command": "WriteInt8Value"}, "GET commands: Int8, Int16")
	for _, r := range e.fake.Requests() {
		if strings.HasSuffix(r.Path, "/WriteInt8Value") {
			t.Error("non-GET command was issued")
		}
	}
}

func TestReadDeviceCommandLocked(t *testing.T) {
	e := newEnv(t, setup{})
	e.mustFail(t, "read_device_command", map[string]any{"device": "Example-MQTT-Sensor", "command": "Temperature"}, "locked or down")
}

func TestErrorMapping(t *testing.T) {
	e := newEnv(t, setup{token: "test-token-value"})
	e.fake.Handle("core-metadata", "/api/v3/device/all", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})
	e.mustFail(t, "list_devices", nil, "EDGEX_TOKEN")
	e.fake.Handle("core-data", "/api/v3/reading/device/name/Random-Integer-Device", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("request timeout"))
	})
	e.mustFail(t, "get_latest_readings", map[string]any{"device": "Random-Integer-Device"}, "HTTP 503")
	e.fake.Handle("core-metadata", "/api/v3/deviceservice/all", func(w http.ResponseWriter, _ *http.Request) {
		edgextest.WriteError(w, http.StatusRequestedRangeNotSatisfiable, "out of range")
	})
	e.mustFail(t, "list_device_services", map[string]any{"offset": 999}, "offset is past the end")
}

func TestUnreachableService(t *testing.T) {
	e := newEnv(t, setup{})
	e.fake.Data.Close()
	e.mustFail(t, "get_latest_readings", map[string]any{"device": "Random-Integer-Device"}, "core-data is unreachable at "+e.fake.Data.URL)
}

func TestInvalidArgumentsAreToolErrors(t *testing.T) {
	e := newEnv(t, setup{})
	e.mustFail(t, "get_device", map[string]any{}, "name")
	e.mustFail(t, "get_device", map[string]any{"name": "x", "extra": true}, "extra")
}

// TestAllToolsNeverMutateOrLeakToken calls every tool through the protocol and
// checks the fake EdgeX saw only GETs and that the token never leaks.
func TestAllToolsNeverMutateOrLeakToken(t *testing.T) {
	e := newEnv(t, setup{token: "test-token-value"})
	calls := map[string]map[string]any{
		"system_health":        nil,
		"list_device_services": nil,
		"list_devices":         nil,
		"get_device":           {"name": "Example-MQTT-Sensor"},
		"list_device_profiles": nil,
		"get_device_profile":   {"name": "Random-Integer-Device"},
		"get_latest_readings":  {"device": "Random-Integer-Device"},
		"query_readings":       {"device": "Random-Integer-Device", "window": "24h"},
		"device_data_stats":    {"device": "Random-Integer-Device"},
		"list_device_commands": nil,
		"read_device_command":  {"device": "Random-Integer-Device", "command": "Int8"},
	}
	if len(calls) != len(allReadTools) {
		t.Fatal("update calls when adding tools")
	}
	var results []string
	for name, args := range calls {
		res, _ := e.call(t, name, args)
		raw, _ := json.Marshal(res)
		results = append(results, string(raw))
	}
	e.fake.Handle("core-metadata", "/api/v3/device/name/Denied", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	res, _ := e.call(t, "get_device", map[string]any{"name": "Denied"})
	results = append(results, text(res))

	for _, r := range e.fake.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("mutating request %s %s", r.Method, r.Path)
		}
		if r.Auth != "Bearer test-token-value" {
			t.Errorf("%s: missing bearer token", r.Path)
		}
	}
	all := strings.Join(results, "\n") + e.logs.String()
	if strings.Contains(all, "test-token-value") {
		t.Error("token leaked into results or logs")
	}
	if strings.Contains(all, "example-pass") {
		t.Error("protocol credential leaked into results or logs")
	}
}

func TestProfileResource(t *testing.T) {
	e := newEnv(t, setup{})
	ctx := context.Background()
	rr, err := e.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "edgex://deviceprofile/Random-Integer-Device"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Contents) != 1 || rr.Contents[0].MIMEType != "application/json" {
		t.Fatalf("contents = %+v", rr.Contents)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(rr.Contents[0].Text), &p); err != nil {
		t.Fatal(err)
	}
	if p["name"] != "Random-Integer-Device" || len(list(p, "resources")) != 3 {
		t.Errorf("profile = %v", p)
	}

	if _, err := e.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "edgex://deviceprofile/Example%20Sensor"}); err == nil {
		t.Error("expected not found")
	}
	if r, _ := e.fake.LastRequest("core-metadata"); r.Path != "/api/v3/deviceprofile/name/Example%20Sensor" {
		t.Errorf("percent-decoding: path = %s", r.Path)
	}

	_, err = e.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "edgex://deviceprofile/does-not-exist"})
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("expected resource-not-found error, got %v", err)
	}
}

func TestReadmeListsEveryTool(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	for _, name := range append(slices.Clone(allReadTools), allWriteTools...) {
		if !strings.Contains(readme, "| `"+name+"` |") {
			t.Errorf("README tool table lacks %s", name)
		}
	}
	if !strings.Contains(readme, "edgex://deviceprofile/{name}") {
		t.Error("README lacks the device profile resource")
	}
}

func TestDotSegmentNameIsRejected(t *testing.T) {
	e := newEnv(t, setup{})
	before := len(e.fake.Requests())
	e.mustFail(t, "get_device", map[string]any{"name": ".."}, "invalid name")
	if len(e.fake.Requests()) != before {
		t.Error("request reached EdgeX")
	}
}
