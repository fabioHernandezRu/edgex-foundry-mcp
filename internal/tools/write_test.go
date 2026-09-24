package tools_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex/edgextest"
	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/tools"
)

func writeEnv(t *testing.T) *env {
	t.Helper()
	return newEnv(t, setup{opts: tools.Options{EnableWrites: true}})
}

func mutating(e *env) []edgextest.Request {
	var out []edgextest.Request
	for _, r := range e.fake.Requests() {
		if r.Method != http.MethodGet {
			out = append(out, r)
		}
	}
	return out
}

func TestSetDeviceCommand(t *testing.T) {
	e := writeEnv(t)
	out := e.mustCall(t, "set_device_command", map[string]any{
		"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int8": "42"},
	})
	w := mutating(e)
	if len(w) != 1 || w[0].Method != http.MethodPut || w[0].Path != "/api/v3/device/name/Random-Integer-Device/Int8" || w[0].Body != `{"Int8":"42"}` {
		t.Fatalf("writes = %+v", w)
	}
	if out["dryRun"] != false || !strings.Contains(out["result"].(string), "accepted") {
		t.Errorf("out = %v", out)
	}
	logs := e.logs.String()
	if !strings.Contains(logs, "edgex write") || !strings.Contains(logs, "tool=set_device_command") || !strings.Contains(logs, "outcome=ok") {
		t.Errorf("audit line missing: %s", logs)
	}
}

func TestSetDeviceCommandValidation(t *testing.T) {
	e := writeEnv(t)
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"not settable", map[string]any{"device": "Example-MQTT-Sensor", "command": "Temperature", "values": map[string]any{"Temperature": "1"}}, "SET commands: none"},
		{"unknown command", map[string]any{"device": "Random-Integer-Device", "command": "Nope", "values": map[string]any{"x": "1"}}, "SET commands: Int8, Int16, WriteInt8Value"},
		{"missing param", map[string]any{"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int16": "1"}}, "exactly the parameters"},
		{"extra param", map[string]any{"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int8": "1", "Int16": "2"}}, "exactly the parameters"},
		{"above profile max", map[string]any{"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int8": "101"}}, "above the maximum 100"},
		{"overflow", map[string]any{"device": "Random-Integer-Device", "command": "Int16", "values": map[string]any{"Int16": "40000"}}, "16-bit"},
		{"unknown device", map[string]any{"device": "Nope", "command": "Int8", "values": map[string]any{"Int8": "1"}}, "not found"},
		{"no values", map[string]any{"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{}}, "required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.mustFail(t, "set_device_command", tc.args, tc.want)
		})
	}
	if w := mutating(e); len(w) != 0 {
		t.Errorf("validation failures must not send writes: %+v", w)
	}
}

func TestSetDeviceCommandDryRun(t *testing.T) {
	e := writeEnv(t)
	out := e.mustCall(t, "set_device_command", map[string]any{
		"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int8": "42"}, "dryRun": true,
	})
	if w := mutating(e); len(w) != 0 {
		t.Fatalf("dry run sent %+v", w)
	}
	req := out["request"].(map[string]any)
	if out["dryRun"] != true || req["method"] != "PUT" || req["path"] != "/api/v3/device/name/Random-Integer-Device/Int8" {
		t.Errorf("out = %v", out)
	}
	if !strings.Contains(e.logs.String(), "dryRun=true") {
		t.Error("dry run not audited")
	}
}

func TestSetDeviceCommandLockedAndNoRetry(t *testing.T) {
	e := writeEnv(t)
	// Real EdgeX propagates a LOCK asynchronously, so the 423 is injected
	// directly instead of relying on a preceding set_device_admin_state.
	e.fake.Handle("core-command", "/api/v3/device/name/Random-Integer-Device/Int8", func(w http.ResponseWriter, r *http.Request) {
		edgextest.WriteError(w, http.StatusLocked, "request failed, status code: 423, err: device Random-Integer-Device locked")
	})
	e.mustFail(t, "set_device_command", map[string]any{
		"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int8": "1"},
	}, "locked or down")

	e.fake.Handle("core-command", "/api/v3/device/name/Random-Float-Device/Float64", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			edgextest.WriteError(w, 500, "driver failure")
			return
		}
		http.NotFound(w, r)
	})
	e.mustFail(t, "set_device_command", map[string]any{
		"device": "Random-Float-Device", "command": "Float64", "values": map[string]any{"Float64": "1.5"},
	}, "HTTP 500")
	puts := 0
	for _, r := range mutating(e) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.Path, "/Float64") {
			puts++
		}
	}
	if puts != 1 {
		t.Errorf("failed SET sent %d times, want exactly 1", puts)
	}
	if !strings.Contains(e.logs.String(), "outcome=\"error: HTTP 500\"") {
		t.Error("failure not audited with its status")
	}
}

func TestDeviceStateTools(t *testing.T) {
	e := writeEnv(t)
	e.mustCall(t, "set_device_admin_state", map[string]any{"device": "Random-Integer-Device", "state": "LOCKED"})
	w := mutating(e)
	want := `[{"apiVersion":"v3","device":{"name":"Random-Integer-Device","adminState":"LOCKED"}}]`
	if len(w) != 1 || w[0].Method != http.MethodPatch || w[0].Path != "/api/v3/device" || w[0].Body != want {
		t.Fatalf("writes = %+v", w)
	}
	if strings.Contains(w[0].Body, "protocols") {
		t.Error("protocols must never be sent")
	}
	out := e.mustCall(t, "get_device", map[string]any{"name": "Random-Integer-Device"})
	if out["adminState"] != "LOCKED" {
		t.Errorf("adminState = %v", out["adminState"])
	}

	e.mustCall(t, "set_device_operating_state", map[string]any{"device": "Random-Integer-Device", "state": "DOWN", "dryRun": true})
	if len(mutating(e)) != 1 {
		t.Error("dry run sent a PATCH")
	}
	e.mustFail(t, "set_device_operating_state", map[string]any{"device": "Random-Integer-Device", "state": "BROKEN"}, "UP, DOWN, UNKNOWN")
	e.mustFail(t, "set_device_admin_state", map[string]any{"device": "No-Such-Device", "state": "LOCKED"}, "not found")
	if len(mutating(e)) != 1 {
		t.Error("invalid state requests must not send a PATCH")
	}
}

func TestDeviceStatePerItemFailure(t *testing.T) {
	e := writeEnv(t)
	e.fake.Handle("core-metadata", "/api/v3/device", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`[{"apiVersion":"v3","statusCode":503,"message":"device service did not respond"}]`))
	})
	e.mustFail(t, "set_device_admin_state", map[string]any{"device": "Random-Integer-Device", "state": "UNLOCKED"}, "device service did not respond")
}

func TestWriteToolsAbsentWithoutGate(t *testing.T) {
	e := newEnv(t, setup{})
	_, err := e.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "set_device_command", Arguments: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("write tool must be unknown without --enable-writes, got %v", err)
	}
	if w := mutating(e); len(w) != 0 {
		t.Errorf("writes sent: %+v", w)
	}
}

func TestWriteAuditRedactsAndOmitsToken(t *testing.T) {
	e := newEnv(t, setup{opts: tools.Options{EnableWrites: true}, token: "test-token-value"})
	// A credential-like resource name: its value must never appear in logs,
	// neither in the values attribute nor through an error message.
	e.fake.Handle("core-command", "/api/v3/device/name/Random-Integer-Device", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v3","statusCode":200,"deviceCoreCommand":{"deviceName":"Random-Integer-Device","profileName":"Random-Integer-Device",` +
			`"coreCommands":[{"name":"DoorKeyCode","set":true,"parameters":[{"resourceName":"DoorKeyCode","valueType":"Uint32"}]}]}}`))
	})
	e.mustFail(t, "set_device_command", map[string]any{
		"device": "Random-Integer-Device", "command": "DoorKeyCode", "values": map[string]any{"DoorKeyCode": "99999999999"},
	}, "not a valid Uint32")
	logs := e.logs.String()
	if strings.Contains(logs, "99999999999") {
		t.Error("SET value leaked into the audit log")
	}
	if !strings.Contains(logs, "DoorKeyCode:***REDACTED***") {
		t.Errorf("audit line lacks the redacted value: %s", logs)
	}
	if strings.Contains(logs, "test-token-value") {
		t.Error("token leaked into logs")
	}
}

func TestWriteOutcomeUnknownOnTimeoutLikeErrors(t *testing.T) {
	e := writeEnv(t)
	e.fake.Handle("core-command", "/api/v3/device/name/Random-Integer-Device/Int8", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("request timeout"))
	})
	e.mustFail(t, "set_device_command", map[string]any{
		"device": "Random-Integer-Device", "command": "Int8", "values": map[string]any{"Int8": "1"},
	}, "OUTCOME UNKNOWN")
}
