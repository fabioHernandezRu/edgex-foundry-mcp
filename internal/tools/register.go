// Package tools implements the MCP tools and resources of edgex-foundry-mcp.
// Read-only tools are always registered; write/actuation tools live in a
// separate set that is registered only with --enable-writes.
package tools

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// Options controls which tools are registered.
type Options struct {
	// EnableWrites registers the write/actuation tool set.
	EnableWrites bool
	// DisableDeviceReads omits read_device_command, which reads physical devices live.
	DisableDeviceReads bool
	// Logger receives registration messages. Defaults to a discarding logger.
	Logger *slog.Logger
}

// Tool name of the live device read, which --disable-device-reads removes.
const readDeviceCommandName = "read_device_command"

// Phrases that every write tool description must contain one of.
var writeWarningPhrases = []string{"physical hardware", "modifies EdgeX metadata"}

// Instructions orient the model when it connects.
const Instructions = `This server gives read-only access to an EdgeX Foundry (LF Edge) deployment through its REST API v3.
Start with system_health to confirm core-metadata, core-data and core-command are reachable.
Discover devices with list_devices and list_device_services, inspect a device with get_device and its profile with get_device_profile.
Readings: get_latest_readings for recent values (newest first), query_readings for a time range, device_data_stats for volumes.
list_device_commands shows commands; read_device_command performs a LIVE read of a physical device, so use it only when stored readings are not enough.
Lists are paginated: pass nextOffset as offset to get more. All times are RFC 3339 UTC.
No tool changes EdgeX metadata or actuates devices unless the server reports write tools are enabled.`

// registration is one tool plus the function that adds it to a server.
type registration struct {
	tool *mcp.Tool
	add  func(*mcp.Server)
}

func reg[In, Out any](t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) registration {
	return registration{tool: t, add: func(s *mcp.Server) { mcp.AddTool(s, t, h) }}
}

// readOnly returns annotations for a read-only tool.
func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true}
}

// Register adds the tools and resources to s and returns the registered tool names.
func Register(s *mcp.Server, c *edgex.Client, o Options) ([]string, error) {
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	var regs []registration
	for _, r := range readTools(c) {
		if !r.tool.Annotations.ReadOnlyHint {
			return nil, fmt.Errorf("tool %s in the read set lacks readOnlyHint", r.tool.Name)
		}
		if o.DisableDeviceReads && r.tool.Name == readDeviceCommandName {
			continue
		}
		regs = append(regs, r)
	}
	if o.EnableWrites {
		log.Warn("write tools are ENABLED: tools in the write set can affect physical hardware or modify EdgeX metadata")
		for _, r := range writeTools(c) {
			if err := validateWriteTool(r.tool); err != nil {
				return nil, err
			}
			regs = append(regs, r)
		}
	} else {
		log.Info("read-only mode: write/actuation tools are not registered (start with --enable-writes to change this)")
	}

	names := make([]string, 0, len(regs))
	for _, r := range regs {
		r.add(s)
		names = append(names, r.tool.Name)
	}
	registerResources(s, c)
	return names, nil
}

// readTools is the read-only tool set. Every tool here only issues EdgeX GET
// requests that do not change metadata, events or readings.
func readTools(c *edgex.Client) []registration {
	return []registration{
		systemHealthTool(c),
		listDeviceServicesTool(c),
		listDevicesTool(c),
		getDeviceTool(c),
		listDeviceProfilesTool(c),
		getDeviceProfileTool(c),
		getLatestReadingsTool(c),
		queryReadingsTool(c),
		deviceDataStatsTool(c),
		listDeviceCommandsTool(c),
		readDeviceCommandTool(c),
	}
}

// writeTools is the write/actuation tool set, registered only with
// --enable-writes. It is intentionally empty until a change adds write tools.
func writeTools(_ *edgex.Client) []registration {
	return nil
}

// validateWriteTool enforces the safety model for write tools.
func validateWriteTool(t *mcp.Tool) error {
	if t.Annotations == nil || t.Annotations.ReadOnlyHint {
		return fmt.Errorf("write tool %s must have annotations with readOnlyHint=false", t.Name)
	}
	for _, p := range writeWarningPhrases {
		if strings.Contains(t.Description, p) {
			return nil
		}
	}
	return fmt.Errorf("write tool %s description must state it affects %q or %q", t.Name, writeWarningPhrases[0], writeWarningPhrases[1])
}
