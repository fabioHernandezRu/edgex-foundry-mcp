package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// ---- list_device_commands ----

// ListDeviceCommandsIn is the input of list_device_commands.
type ListDeviceCommandsIn struct {
	Device string `json:"device,omitempty" jsonschema:"device name; omit to list commands of all devices"`
	Offset int    `json:"offset,omitempty" jsonschema:"number of devices to skip when listing all devices"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum devices to return when listing all devices (default 20, capped by the server's --max-results)"`
}

// ParameterOut is one command parameter.
type ParameterOut struct {
	Resource  string `json:"resource" jsonschema:"resource name"`
	ValueType string `json:"valueType" jsonschema:"EdgeX value type"`
}

// CoreCommandOut is one core command.
type CoreCommandOut struct {
	Name       string         `json:"name" jsonschema:"command name, used with read_device_command"`
	Get        bool           `json:"get" jsonschema:"true when the command can be read (GET)"`
	Set        bool           `json:"set" jsonschema:"true when the command can be written (SET, not available in read-only mode)"`
	Parameters []ParameterOut `json:"parameters,omitempty" jsonschema:"resources and value types involved"`
}

// DeviceCommandsOut groups the commands of one device.
type DeviceCommandsOut struct {
	Device   string           `json:"device" jsonschema:"device name"`
	Profile  string           `json:"profile" jsonschema:"device profile name"`
	Commands []CoreCommandOut `json:"commands" jsonschema:"core commands"`
}

// ListDeviceCommandsOut is the output of list_device_commands.
type ListDeviceCommandsOut struct {
	ListInfo
	Devices []DeviceCommandsOut `json:"devices" jsonschema:"devices with their commands"`
}

func shapeCoreCommands(d edgex.DeviceCoreCommand) DeviceCommandsOut {
	out := DeviceCommandsOut{Device: d.DeviceName, Profile: d.ProfileName, Commands: []CoreCommandOut{}}
	for _, cc := range d.CoreCommands {
		co := CoreCommandOut{Name: cc.Name, Get: cc.Get, Set: cc.Set}
		for _, p := range cc.Parameters {
			co.Parameters = append(co.Parameters, ParameterOut{Resource: p.ResourceName, ValueType: p.ValueType})
		}
		out.Commands = append(out.Commands, co)
	}
	return out
}

func listDeviceCommandsTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "list_device_commands",
		Description: "List the commands EdgeX core-command exposes for a device (or for all devices), with whether each supports GET (read) and SET (write) and its parameters. " +
			"Read-only: listing commands does not execute them.",
		Annotations: readOnly("List device commands"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListDeviceCommandsIn) (*mcp.CallToolResult, ListDeviceCommandsOut, error) {
		if in.Device != "" {
			d, err := c.DeviceCommands(ctx, in.Device)
			if err != nil {
				return nil, ListDeviceCommandsOut{}, toolError(err, fmt.Sprintf("device %q not found in core-command; use list_devices to see device names", in.Device))
			}
			out := ListDeviceCommandsOut{Devices: []DeviceCommandsOut{shapeCoreCommands(*d)}}
			out.ListInfo = listInfo(1, 0, 1)
			return nil, out, nil
		}
		limit := c.ClampLimit(in.Limit, defaultListLimit)
		items, total, err := c.AllDeviceCommands(ctx, edgex.Page{Offset: in.Offset, Limit: limit})
		if err != nil {
			return nil, ListDeviceCommandsOut{}, toolError(err, "")
		}
		out := ListDeviceCommandsOut{Devices: make([]DeviceCommandsOut, 0, len(items))}
		for _, d := range items {
			out.Devices = append(out.Devices, shapeCoreCommands(d))
		}
		out.ListInfo = listInfo(total, in.Offset, len(out.Devices))
		return nil, out, nil
	})
}

// ---- read_device_command ----

// ReadCommandIn is the input of read_device_command.
type ReadCommandIn struct {
	Device  string `json:"device" jsonschema:"device name"`
	Command string `json:"command" jsonschema:"GET-capable command name from list_device_commands"`
}

// ReadCommandOut is the output of read_device_command.
type ReadCommandOut struct {
	Device   string       `json:"device" jsonschema:"device name"`
	Command  string       `json:"command" jsonschema:"command name"`
	Time     string       `json:"time,omitempty" jsonschema:"event time, RFC 3339 UTC"`
	Readings []ReadingOut `json:"readings" jsonschema:"values read from the device"`
}

func readDeviceCommandTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: readDeviceCommandName,
		Description: "Execute a GET command through EdgeX core-command, which asks the device service to perform a LIVE read of the physical device and returns the values. " +
			"It requests a read only and never publishes or stores an event (ds-pushevent=false); what a read does on the device is defined by its device service driver. " +
			"Repeated failed reads can cause EdgeX to mark the device operating state DOWN, so prefer get_latest_readings unless a fresh value is required. " +
			"The command must support GET (see list_device_commands).",
		Annotations: readOnly("Read device command (live)"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ReadCommandIn) (*mcp.CallToolResult, ReadCommandOut, error) {
		if in.Device == "" || in.Command == "" {
			return nil, ReadCommandOut{}, errors.New("device and command are required")
		}
		d, err := c.DeviceCommands(ctx, in.Device)
		if err != nil {
			return nil, ReadCommandOut{}, toolError(err, fmt.Sprintf("device %q not found in core-command; use list_devices to see device names", in.Device))
		}
		var getCmds []string
		found := false
		for _, cc := range d.CoreCommands {
			if cc.Get {
				getCmds = append(getCmds, cc.Name)
				if cc.Name == in.Command {
					found = true
				}
			}
		}
		if !found {
			avail := "none"
			if len(getCmds) > 0 {
				avail = strings.Join(getCmds, ", ")
			}
			return nil, ReadCommandOut{}, fmt.Errorf("command %q is not a GET command of device %q; GET commands: %s", in.Command, in.Device, avail)
		}
		ev, err := c.ReadCommand(ctx, in.Device, in.Command)
		if err != nil {
			return nil, ReadCommandOut{}, toolError(err, fmt.Sprintf("command %q or device %q not found", in.Command, in.Device))
		}
		return nil, ReadCommandOut{Device: in.Device, Command: in.Command, Time: nsTime(ev.Origin), Readings: shapeReadings(ev.Readings)}, nil
	})
}
