package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// Write tools: registered only with --enable-writes (see Register). They
// validate everything before sending, support dryRun, log one audit line per
// call and never retry.

func boolPtr(b bool) *bool { return &b }

// PlannedRequest describes the request a write tool sends (or would send in a dry run).
type PlannedRequest struct {
	Method string `json:"method" jsonschema:"HTTP method"`
	Path   string `json:"path" jsonschema:"EdgeX API path"`
	Body   any    `json:"body" jsonschema:"JSON body"`
}

// WriteOut is the output of every write tool.
type WriteOut struct {
	DryRun  bool           `json:"dryRun" jsonschema:"true when nothing was sent"`
	Device  string         `json:"device" jsonschema:"device name"`
	Request PlannedRequest `json:"request" jsonschema:"the request sent to EdgeX, or that would be sent in a dry run"`
	Result  string         `json:"result" jsonschema:"what happened"`
}

func auditWrite(log *slog.Logger, tool, device string, dryRun bool, attrs []any, err error) {
	if log == nil {
		return
	}
	// The outcome never includes error text: validation and EdgeX messages can
	// echo the submitted values, which are only logged in redacted form.
	outcome := "ok"
	if err != nil {
		outcome = "error"
		if st := edgex.StatusOf(err); st != 0 {
			outcome = fmt.Sprintf("error: HTTP %d", st)
		}
	}
	args := append([]any{"tool", tool, "device", device, "dryRun", dryRun}, attrs...)
	args = append(args, "outcome", outcome)
	log.Warn("edgex write", args...)
}

// ---- set_device_command ----

// SetCommandIn is the input of set_device_command.
type SetCommandIn struct {
	Device  string            `json:"device" jsonschema:"device name"`
	Command string            `json:"command" jsonschema:"settable command name from list_device_commands (set: true)"`
	Values  map[string]string `json:"values" jsonschema:"value for EVERY parameter of the command, keyed by resource name, as strings (e.g. {\"Int8\":\"42\"}; arrays as JSON text \"[1,2]\")"`
	DryRun  bool              `json:"dryRun,omitempty" jsonschema:"validate and show the request without sending it (default false)"`
}

func setDeviceCommandTool(c *edgex.Client, log *slog.Logger) registration {
	const name = "set_device_command"
	return reg(&mcp.Tool{
		Name: name,
		Description: "WRITE: send a SET command through EdgeX core-command to a device. This affects physical hardware: the device service writes the values to the real device. " +
			"Only available because the server runs with --enable-writes. Confirm with the user before calling it, and prefer a dryRun first. " +
			"Values are validated against the command's parameters, value types and profile min/max before anything is sent; failed writes are not retried.",
		Annotations: &mcp.ToolAnnotations{Title: "Set device command (writes to hardware)", DestructiveHint: boolPtr(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SetCommandIn) (_ *mcp.CallToolResult, _ WriteOut, err error) {
		out := WriteOut{DryRun: in.DryRun, Device: in.Device, Request: PlannedRequest{
			Method: "PUT",
			Path:   "/api/v3/device/name/" + url.PathEscape(in.Device) + "/" + url.PathEscape(in.Command),
			Body:   in.Values,
		}}
		defer func() {
			auditWrite(log, name, in.Device, in.DryRun, []any{"command", in.Command, "values", RedactMap(toAnyMap(in.Values))}, err)
		}()
		if in.Device == "" || in.Command == "" || len(in.Values) == 0 {
			return nil, WriteOut{}, errors.New("device, command and values are required")
		}
		if err := validateSetCommand(ctx, c, in); err != nil {
			return nil, WriteOut{}, err
		}
		if in.DryRun {
			out.Result = "dry run: validation passed, nothing was sent"
			return nil, out, nil
		}
		if err := c.SetCommand(ctx, in.Device, in.Command, in.Values); err != nil {
			return nil, WriteOut{}, writeError(err, fmt.Sprintf("device %q or command %q not found", in.Device, in.Command), "read_device_command")
		}
		out.Result = "SET accepted by EdgeX"
		return nil, out, nil
	})
}

func validateSetCommand(ctx context.Context, c *edgex.Client, in SetCommandIn) error {
	dc, err := c.DeviceCommands(ctx, in.Device)
	if err != nil {
		return toolError(err, fmt.Sprintf("device %q not found in core-command; use list_devices", in.Device))
	}
	var cmd *edgex.CoreCommand
	var settable []string
	for i := range dc.CoreCommands {
		cc := &dc.CoreCommands[i]
		if cc.Set {
			settable = append(settable, cc.Name)
			if cc.Name == in.Command {
				cmd = cc
			}
		}
	}
	if cmd == nil {
		avail := "none"
		if len(settable) > 0 {
			avail = strings.Join(settable, ", ")
		}
		return fmt.Errorf("command %q is not a SET command of device %q; SET commands: %s", in.Command, in.Device, avail)
	}

	params := map[string]string{}
	for _, p := range cmd.Parameters {
		params[p.ResourceName] = p.ValueType
	}
	expected := slices.Sorted(maps.Keys(params))
	given := slices.Sorted(maps.Keys(in.Values))
	if !slices.Equal(expected, given) {
		return fmt.Errorf("values must contain exactly the parameters of %q: %s (got %s)", in.Command, strings.Join(expected, ", "), strings.Join(given, ", "))
	}

	// Minimum/maximum come from the device profile.
	props := map[string]edgex.ResourceProperties{}
	if d, err := c.Device(ctx, in.Device); err != nil {
		return toolError(err, fmt.Sprintf("device %q not found in core-metadata", in.Device))
	} else if d.ProfileName != "" {
		p, err := c.DeviceProfile(ctx, d.ProfileName)
		if err != nil {
			return toolError(err, fmt.Sprintf("device profile %q not found", d.ProfileName))
		}
		for _, r := range p.DeviceResources {
			props[r.Name] = r.Properties
		}
	}

	var problems []string
	for _, res := range expected {
		pr := props[res]
		if err := validateSetValue(params[res], in.Values[res], pr.Minimum, pr.Maximum); err != nil {
			problems = append(problems, fmt.Sprintf("%s (%s): %v", res, params[res], err))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid values, nothing was sent: %s", strings.Join(problems, "; "))
	}
	return nil
}

// writeError maps a failed write. When the request may have reached EdgeX but
// no definitive answer came back (timeout, connection lost, 503, unreadable
// response), it says so explicitly so the model checks state before retrying.
func writeError(err error, notFound, checkTool string) error {
	te := toolError(err, notFound)
	var ae *edgex.APIError
	if errors.As(err, &ae) && (ae.Timeout || ae.Status == 0 || ae.Status == http.StatusServiceUnavailable || ae.Err != nil) {
		return fmt.Errorf("%w; OUTCOME UNKNOWN: the write may have been applied, check the device with %s before retrying", te, checkTool)
	}
	return te
}

func toAnyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ---- device state tools ----

// StateIn is the input of the device state tools.
type StateIn struct {
	Device string `json:"device" jsonschema:"device name"`
	State  string `json:"state" jsonschema:"new state"`
	DryRun bool   `json:"dryRun,omitempty" jsonschema:"validate and show the request without sending it (default false)"`
}

func stateTool(c *edgex.Client, log *slog.Logger, name, field string, allowed []string, desc string, ann *mcp.ToolAnnotations) registration {
	return reg(&mcp.Tool{Name: name, Description: desc, Annotations: ann},
		func(ctx context.Context, _ *mcp.CallToolRequest, in StateIn) (_ *mcp.CallToolResult, _ WriteOut, err error) {
			out := WriteOut{DryRun: in.DryRun, Device: in.Device, Request: PlannedRequest{
				Method: "PATCH", Path: "/api/v3/device",
				Body: []map[string]any{{"apiVersion": "v3", "device": map[string]any{"name": in.Device, field: in.State}}},
			}}
			defer func() {
				auditWrite(log, name, in.Device, in.DryRun, []any{"field", field, "state", in.State}, err)
			}()
			if in.Device == "" {
				return nil, WriteOut{}, errors.New("device is required")
			}
			if !slices.Contains(allowed, in.State) {
				return nil, WriteOut{}, fmt.Errorf("state must be one of %s", strings.Join(allowed, ", "))
			}
			if _, err := c.Device(ctx, in.Device); err != nil {
				return nil, WriteOut{}, toolError(err, fmt.Sprintf("device %q not found; use list_devices", in.Device))
			}
			if in.DryRun {
				out.Result = "dry run: validation passed, nothing was sent"
				return nil, out, nil
			}
			if err := c.UpdateDeviceState(ctx, in.Device, field, in.State); err != nil {
				return nil, WriteOut{}, writeError(err, fmt.Sprintf("device %q not found", in.Device), "get_device")
			}
			out.Result = fmt.Sprintf("%s set to %s; EdgeX propagates it to the device service asynchronously", field, in.State)
			return nil, out, nil
		})
}

func setDeviceAdminStateTool(c *edgex.Client, log *slog.Logger) registration {
	return stateTool(c, log, "set_device_admin_state", edgex.AdminStateField, []string{"LOCKED", "UNLOCKED"},
		"WRITE: lock or unlock a device in EdgeX core-metadata (adminState LOCKED or UNLOCKED). This modifies EdgeX metadata and affects physical hardware operation: "+
			"a LOCKED device rejects all commands (HTTP 423) and its automatic readings stop. The change reaches the device service asynchronously. "+
			"Only available with --enable-writes; confirm with the user before calling it.",
		&mcp.ToolAnnotations{Title: "Set device admin state", DestructiveHint: boolPtr(true), IdempotentHint: true})
}

func setDeviceOperatingStateTool(c *edgex.Client, log *slog.Logger) registration {
	return stateTool(c, log, "set_device_operating_state", edgex.OperatingStateField, []string{"UP", "DOWN", "UNKNOWN"},
		"WRITE: set a device's operatingState in EdgeX core-metadata (UP, DOWN or UNKNOWN). This modifies EdgeX metadata and affects physical hardware operation: "+
			"a DOWN device rejects every GET and SET command with HTTP 423 until it is set back to UP (EdgeX does not restore it automatically by default). "+
			"Only available with --enable-writes; confirm with the user before calling it.",
		&mcp.ToolAnnotations{Title: "Set device operating state", DestructiveHint: boolPtr(true), IdempotentHint: true})
}
