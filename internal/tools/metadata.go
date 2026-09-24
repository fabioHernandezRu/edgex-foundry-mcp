package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// Default page sizes per tool family.
const (
	defaultListLimit   = 20
	defaultLatestLimit = 10
	defaultQueryLimit  = 50
)

// ---- list_device_services ----

// ListDeviceServicesIn is the input of list_device_services.
type ListDeviceServicesIn struct {
	Labels []string `json:"labels,omitempty" jsonschema:"only services carrying ALL of these labels"`
	Offset int      `json:"offset,omitempty" jsonschema:"number of items to skip (pagination)"`
	Limit  int      `json:"limit,omitempty" jsonschema:"maximum items to return (default 20, capped by the server's --max-results)"`
}

// DeviceServiceOut is a compact device service.
type DeviceServiceOut struct {
	Name        string         `json:"name" jsonschema:"device service name"`
	Description string         `json:"description,omitempty" jsonschema:"description"`
	AdminState  string         `json:"adminState" jsonschema:"LOCKED or UNLOCKED"`
	BaseAddress string         `json:"baseAddress,omitempty" jsonschema:"address EdgeX uses to reach the service"`
	Labels      []string       `json:"labels,omitempty" jsonschema:"labels"`
	Properties  map[string]any `json:"properties,omitempty" jsonschema:"service properties, credential-like values redacted"`
}

// ListDeviceServicesOut is the output of list_device_services.
type ListDeviceServicesOut struct {
	ListInfo
	Services []DeviceServiceOut `json:"services" jsonschema:"device services"`
}

func listDeviceServicesTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name:        "list_device_services",
		Description: "List the device services registered in EdgeX core-metadata (the protocol adapters such as device-virtual or device-mqtt), with admin state and base address. Read-only.",
		Annotations: readOnly("List device services"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListDeviceServicesIn) (*mcp.CallToolResult, ListDeviceServicesOut, error) {
		limit := c.ClampLimit(in.Limit, defaultListLimit)
		items, total, err := c.DeviceServices(ctx, in.Labels, edgex.Page{Offset: in.Offset, Limit: limit})
		if err != nil {
			return nil, ListDeviceServicesOut{}, toolError(err, "")
		}
		out := ListDeviceServicesOut{Services: make([]DeviceServiceOut, 0, len(items))}
		for _, s := range items {
			out.Services = append(out.Services, DeviceServiceOut{
				Name: s.Name, Description: s.Description, AdminState: s.AdminState,
				BaseAddress: s.BaseAddress, Labels: s.Labels, Properties: RedactMap(s.Properties),
			})
		}
		out.ListInfo = listInfo(total, in.Offset, len(out.Services))
		if len(out.Services) == 0 {
			out.Hint = "no device services found; check the labels filter or whether any device service has registered"
		}
		return nil, out, nil
	})
}

// ---- list_devices ----

// ListDevicesIn is the input of list_devices.
type ListDevicesIn struct {
	Service string   `json:"service,omitempty" jsonschema:"only devices managed by this device service (cannot be combined with profile or labels)"`
	Profile string   `json:"profile,omitempty" jsonschema:"only devices using this device profile (cannot be combined with service or labels)"`
	Labels  []string `json:"labels,omitempty" jsonschema:"only devices carrying ALL of these labels (cannot be combined with service or profile)"`
	Offset  int      `json:"offset,omitempty" jsonschema:"number of items to skip (pagination)"`
	Limit   int      `json:"limit,omitempty" jsonschema:"maximum items to return (default 20, capped by the server's --max-results)"`
}

// DeviceSummary is a compact device, without protocols or properties.
type DeviceSummary struct {
	Name           string   `json:"name" jsonschema:"device name"`
	Description    string   `json:"description,omitempty" jsonschema:"description"`
	Profile        string   `json:"profile,omitempty" jsonschema:"device profile name"`
	Service        string   `json:"service" jsonschema:"device service name"`
	AdminState     string   `json:"adminState" jsonschema:"LOCKED or UNLOCKED"`
	OperatingState string   `json:"operatingState" jsonschema:"UP, DOWN or UNKNOWN"`
	Labels         []string `json:"labels,omitempty" jsonschema:"labels"`
}

// ListDevicesOut is the output of list_devices.
type ListDevicesOut struct {
	ListInfo
	Devices []DeviceSummary `json:"devices" jsonschema:"devices"`
}

func listDevicesTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "list_devices",
		Description: "List devices registered in EdgeX core-metadata with their profile, service, admin state and operating state. " +
			"Optionally filter by ONE of: service, profile or labels. Use get_device for full details of one device. Read-only.",
		Annotations: readOnly("List devices"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListDevicesIn) (*mcp.CallToolResult, ListDevicesOut, error) {
		n := 0
		for _, set := range []bool{in.Service != "", in.Profile != "", len(in.Labels) > 0} {
			if set {
				n++
			}
		}
		if n > 1 {
			return nil, ListDevicesOut{}, errors.New("use only one filter: service, profile or labels (EdgeX cannot combine them)")
		}
		limit := c.ClampLimit(in.Limit, defaultListLimit)
		items, total, err := c.Devices(ctx, edgex.DeviceFilter{Service: in.Service, Profile: in.Profile, Labels: in.Labels}, edgex.Page{Offset: in.Offset, Limit: limit})
		if err != nil {
			return nil, ListDevicesOut{}, toolError(err, "")
		}
		out := ListDevicesOut{Devices: make([]DeviceSummary, 0, len(items))}
		for _, d := range items {
			out.Devices = append(out.Devices, DeviceSummary{
				Name: d.Name, Description: d.Description, Profile: d.ProfileName, Service: d.ServiceName,
				AdminState: d.AdminState, OperatingState: d.OperatingState, Labels: d.Labels,
			})
		}
		out.ListInfo = listInfo(total, in.Offset, len(out.Devices))
		if len(out.Devices) == 0 {
			switch {
			case in.Service != "":
				out.Hint = fmt.Sprintf("no devices for service %q; the service name may not exist (see list_device_services)", in.Service)
			case in.Profile != "":
				out.Hint = fmt.Sprintf("no devices for profile %q; the profile name may not exist (see list_device_profiles)", in.Profile)
			default:
				out.Hint = "no devices found"
			}
		}
		return nil, out, nil
	})
}

// ---- get_device ----

// NameIn is the input of tools that take one name.
type NameIn struct {
	Name string `json:"name" jsonschema:"exact name as registered in EdgeX"`
}

// AutoEventOut is a compact auto event.
type AutoEventOut struct {
	Source   string `json:"source" jsonschema:"resource or command read automatically"`
	Interval string `json:"interval" jsonschema:"reading interval, e.g. 15s"`
	OnChange bool   `json:"onChange,omitempty" jsonschema:"true when events are only sent on value change"`
}

// DeviceOut is the full, redacted form of a device.
type DeviceOut struct {
	DeviceSummary
	Parent     string         `json:"parent,omitempty" jsonschema:"parent device name"`
	Location   any            `json:"location,omitempty" jsonschema:"free-form location"`
	AutoEvents []AutoEventOut `json:"autoEvents,omitempty" jsonschema:"scheduled readings"`
	Protocols  map[string]any `json:"protocols,omitempty" jsonschema:"protocol properties per protocol, credential-like values redacted"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"device properties, credential-like values redacted"`
	Tags       map[string]any `json:"tags,omitempty" jsonschema:"device tags, credential-like values redacted"`
	Created    string         `json:"created,omitempty" jsonschema:"creation time, RFC 3339 UTC"`
	Modified   string         `json:"modified,omitempty" jsonschema:"last modification time, RFC 3339 UTC"`
}

func getDeviceTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "get_device",
		Description: "Get one EdgeX device by name: profile, service, admin/operating state, labels, location, auto events and protocol properties. " +
			"Credential-like protocol and property values are redacted. Read-only.",
		Annotations: readOnly("Get device"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in NameIn) (*mcp.CallToolResult, DeviceOut, error) {
		d, err := c.Device(ctx, in.Name)
		if err != nil {
			return nil, DeviceOut{}, toolError(err, fmt.Sprintf("device %q not found; use list_devices to see registered device names", in.Name))
		}
		out := DeviceOut{
			DeviceSummary: DeviceSummary{
				Name: d.Name, Description: d.Description, Profile: d.ProfileName, Service: d.ServiceName,
				AdminState: d.AdminState, OperatingState: d.OperatingState, Labels: d.Labels,
			},
			Parent:     d.Parent,
			Location:   d.Location,
			Protocols:  RedactProtocols(d.Protocols),
			Properties: RedactMap(d.Properties),
			Tags:       RedactMap(d.Tags),
			Created:    msTime(d.Created),
			Modified:   msTime(d.Modified),
		}
		for _, ae := range d.AutoEvents {
			out.AutoEvents = append(out.AutoEvents, AutoEventOut{Source: ae.SourceName, Interval: ae.Interval, OnChange: ae.OnChange})
		}
		return nil, out, nil
	})
}

// ---- list_device_profiles ----

// ListDeviceProfilesIn is the input of list_device_profiles.
type ListDeviceProfilesIn struct {
	Manufacturer string   `json:"manufacturer,omitempty" jsonschema:"only profiles from this manufacturer (cannot be combined with labels)"`
	Model        string   `json:"model,omitempty" jsonschema:"only profiles of this model (cannot be combined with labels)"`
	Labels       []string `json:"labels,omitempty" jsonschema:"only profiles carrying ALL of these labels"`
	Offset       int      `json:"offset,omitempty" jsonschema:"number of items to skip (pagination)"`
	Limit        int      `json:"limit,omitempty" jsonschema:"maximum items to return (default 20, capped by the server's --max-results)"`
}

// ProfileSummary is a compact device profile.
type ProfileSummary struct {
	Name              string   `json:"name" jsonschema:"device profile name"`
	Manufacturer      string   `json:"manufacturer,omitempty" jsonschema:"manufacturer"`
	Model             string   `json:"model,omitempty" jsonschema:"model"`
	Description       string   `json:"description,omitempty" jsonschema:"description"`
	Labels            []string `json:"labels,omitempty" jsonschema:"labels"`
	LinkedDeviceCount int64    `json:"linkedDeviceCount,omitempty" jsonschema:"number of devices using the profile"`
}

// ListDeviceProfilesOut is the output of list_device_profiles.
type ListDeviceProfilesOut struct {
	ListInfo
	Profiles []ProfileSummary `json:"profiles" jsonschema:"device profiles"`
}

func listDeviceProfilesTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name:        "list_device_profiles",
		Description: "List EdgeX device profiles (device type definitions) with manufacturer, model and number of linked devices. Filter by manufacturer and/or model, or by labels. Use get_device_profile for resources and commands. Read-only.",
		Annotations: readOnly("List device profiles"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListDeviceProfilesIn) (*mcp.CallToolResult, ListDeviceProfilesOut, error) {
		if len(in.Labels) > 0 && (in.Manufacturer != "" || in.Model != "") {
			return nil, ListDeviceProfilesOut{}, errors.New("labels cannot be combined with manufacturer or model")
		}
		limit := c.ClampLimit(in.Limit, defaultListLimit)
		items, total, err := c.DeviceProfiles(ctx, edgex.ProfileFilter{Manufacturer: in.Manufacturer, Model: in.Model, Labels: in.Labels}, edgex.Page{Offset: in.Offset, Limit: limit})
		if err != nil {
			return nil, ListDeviceProfilesOut{}, toolError(err, "")
		}
		out := ListDeviceProfilesOut{Profiles: make([]ProfileSummary, 0, len(items))}
		for _, p := range items {
			out.Profiles = append(out.Profiles, ProfileSummary{
				Name: p.Name, Manufacturer: p.Manufacturer, Model: p.Model, Description: p.Description,
				Labels: p.Labels, LinkedDeviceCount: p.LinkedDeviceCount,
			})
		}
		out.ListInfo = listInfo(total, in.Offset, len(out.Profiles))
		if len(out.Profiles) == 0 {
			out.Hint = "no device profiles match the filters"
		}
		return nil, out, nil
	})
}

// ---- get_device_profile ----

// GetDeviceProfileIn is the input of get_device_profile.
type GetDeviceProfileIn struct {
	Name          string `json:"name" jsonschema:"device profile name"`
	IncludeHidden bool   `json:"includeHidden,omitempty" jsonschema:"also return hidden resources and commands (default false)"`
}

// ResourceOut is a compact device resource.
type ResourceOut struct {
	Name         string         `json:"name" jsonschema:"resource name, used in readings and commands"`
	Description  string         `json:"description,omitempty" jsonschema:"description"`
	ValueType    string         `json:"valueType" jsonschema:"EdgeX value type"`
	ReadWrite    string         `json:"readWrite" jsonschema:"R, W, RW or WR"`
	Units        string         `json:"units,omitempty" jsonschema:"units"`
	Minimum      *float64       `json:"minimum,omitempty" jsonschema:"minimum value"`
	Maximum      *float64       `json:"maximum,omitempty" jsonschema:"maximum value"`
	DefaultValue string         `json:"defaultValue,omitempty" jsonschema:"default value"`
	MediaType    string         `json:"mediaType,omitempty" jsonschema:"media type for binary resources"`
	Attributes   map[string]any `json:"attributes,omitempty" jsonschema:"protocol-specific attributes, credential-like values redacted"`
	Hidden       bool           `json:"hidden,omitempty" jsonschema:"true for hidden resources"`
}

// CommandOut is a compact device command.
type CommandOut struct {
	Name      string   `json:"name" jsonschema:"device command name"`
	ReadWrite string   `json:"readWrite" jsonschema:"R, W, RW or WR"`
	Resources []string `json:"resources" jsonschema:"resources the command operates on"`
	Hidden    bool     `json:"hidden,omitempty" jsonschema:"true for hidden commands"`
}

// ProfileOut is a device profile with its resources and commands.
type ProfileOut struct {
	ProfileSummary
	Resources []ResourceOut `json:"resources" jsonschema:"device resources"`
	Commands  []CommandOut  `json:"commands,omitempty" jsonschema:"device commands"`
}

func shapeProfile(p *edgex.DeviceProfile, includeHidden bool) ProfileOut {
	out := ProfileOut{
		ProfileSummary: ProfileSummary{
			Name: p.Name, Manufacturer: p.Manufacturer, Model: p.Model, Description: p.Description,
			Labels: p.Labels, LinkedDeviceCount: p.LinkedDeviceCount,
		},
		Resources: []ResourceOut{},
	}
	for _, r := range p.DeviceResources {
		if r.IsHidden && !includeHidden {
			continue
		}
		out.Resources = append(out.Resources, ResourceOut{
			Name: r.Name, Description: r.Description, ValueType: r.Properties.ValueType, ReadWrite: r.Properties.ReadWrite,
			Units: r.Properties.Units, Minimum: r.Properties.Minimum, Maximum: r.Properties.Maximum,
			DefaultValue: r.Properties.DefaultValue, MediaType: r.Properties.MediaType,
			Attributes: RedactMap(r.Attributes), Hidden: r.IsHidden,
		})
	}
	for _, cmd := range p.DeviceCommands {
		if cmd.IsHidden && !includeHidden {
			continue
		}
		co := CommandOut{Name: cmd.Name, ReadWrite: cmd.ReadWrite, Hidden: cmd.IsHidden, Resources: []string{}}
		for _, op := range cmd.ResourceOperations {
			co.Resources = append(co.Resources, op.DeviceResource)
		}
		out.Commands = append(out.Commands, co)
	}
	return out
}

func getDeviceProfileTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "get_device_profile",
		Description: "Get one EdgeX device profile by name: its resources (value type, read/write, units, range) and device commands. " +
			"Profiles are often named after the device type; get_device shows which profile a device uses. Read-only.",
		Annotations: readOnly("Get device profile"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetDeviceProfileIn) (*mcp.CallToolResult, ProfileOut, error) {
		p, err := c.DeviceProfile(ctx, in.Name)
		if err != nil {
			return nil, ProfileOut{}, toolError(err, fmt.Sprintf("device profile %q not found; use list_device_profiles to see profile names", in.Name))
		}
		return nil, shapeProfile(p, in.IncludeHidden), nil
	})
}
