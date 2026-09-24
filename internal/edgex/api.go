package edgex

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// Ping calls GET /api/v3/ping on a service.
func (c *Client) Ping(ctx context.Context, s Service) (*PingResponse, error) {
	var out PingResponse
	if err := c.get(ctx, s, path("ping"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Version calls GET /api/v3/version on a service.
func (c *Client) Version(ctx context.Context, s Service) (*VersionResponse, error) {
	var out VersionResponse
	if err := c.get(ctx, s, path("version"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeviceServices lists device services (core-metadata GET /deviceservice/all).
func (c *Client) DeviceServices(ctx context.Context, labels []string, p Page) ([]DeviceService, int, error) {
	q := c.pageQuery(p)
	setLabels(q, labels)
	var out multiDeviceServicesResponse
	if err := c.get(ctx, Metadata, path("deviceservice", "all"), q, &out); err != nil {
		return nil, 0, err
	}
	return out.Services, out.TotalCount, nil
}

// DeviceFilter selects devices. At most one field may be set.
type DeviceFilter struct {
	Service string
	Profile string
	Labels  []string
}

// Devices lists devices, filtered by service, profile or labels.
func (c *Client) Devices(ctx context.Context, f DeviceFilter, p Page) ([]Device, int, error) {
	q := c.pageQuery(p)
	var pth string
	switch {
	case f.Service != "":
		pth = path("device", "service", "name", f.Service)
	case f.Profile != "":
		pth = path("device", "profile", "name", f.Profile)
	default:
		pth = path("device", "all")
		setLabels(q, f.Labels)
	}
	var out multiDevicesResponse
	if err := c.get(ctx, Metadata, pth, q, &out); err != nil {
		return nil, 0, err
	}
	return out.Devices, out.TotalCount, nil
}

// Device returns one device by name.
func (c *Client) Device(ctx context.Context, name string) (*Device, error) {
	var out deviceResponse
	if err := c.get(ctx, Metadata, path("device", "name", name), nil, &out); err != nil {
		return nil, err
	}
	return &out.Device, nil
}

// ProfileFilter selects device profiles. Labels cannot be combined with
// Manufacturer or Model.
type ProfileFilter struct {
	Manufacturer string
	Model        string
	Labels       []string
}

// DeviceProfiles lists device profile summaries.
func (c *Client) DeviceProfiles(ctx context.Context, f ProfileFilter, p Page) ([]ProfileBasicInfo, int, error) {
	q := c.pageQuery(p)
	var pth string
	switch {
	case f.Manufacturer != "" && f.Model != "":
		pth = path("deviceprofile", "manufacturer", f.Manufacturer, "model", f.Model)
	case f.Manufacturer != "":
		pth = path("deviceprofile", "manufacturer", f.Manufacturer)
	case f.Model != "":
		pth = path("deviceprofile", "model", f.Model)
	default:
		pth = path("deviceprofile", "basicinfo", "all")
		setLabels(q, f.Labels)
	}
	// Full-profile responses decode into the basic info subset.
	var out multiProfilesBasicResponse
	if err := c.get(ctx, Metadata, pth, q, &out); err != nil {
		return nil, 0, err
	}
	return out.Profiles, out.TotalCount, nil
}

// DeviceProfile returns one full device profile by name.
func (c *Client) DeviceProfile(ctx context.Context, name string) (*DeviceProfile, error) {
	var out deviceProfileResponse
	if err := c.get(ctx, Metadata, path("deviceprofile", "name", name), nil, &out); err != nil {
		return nil, err
	}
	return &out.Profile, nil
}

// ReadingQuery selects readings of one device, optionally one resource and a
// time range in Unix nanoseconds (inclusive).
type ReadingQuery struct {
	Device   string
	Resource string
	HasRange bool
	StartNs  int64
	EndNs    int64
}

// Readings returns readings newest first.
func (c *Client) Readings(ctx context.Context, rq ReadingQuery, p Page) ([]Reading, int, error) {
	if rq.Device == "" {
		return nil, 0, errors.New("edgex: device name is required")
	}
	segs := []string{"reading", "device", "name", rq.Device}
	if rq.Resource != "" {
		segs = append(segs, "resourceName", rq.Resource)
	}
	if rq.HasRange {
		if rq.StartNs < 0 || rq.EndNs < rq.StartNs {
			return nil, 0, errors.New("edgex: invalid time range")
		}
		segs = append(segs, "start", strconv.FormatInt(rq.StartNs, 10), "end", strconv.FormatInt(rq.EndNs, 10))
	}
	var out multiReadingsResponse
	if err := c.get(ctx, Data, path(segs...), c.pageQuery(p), &out); err != nil {
		return nil, 0, err
	}
	return out.Readings, out.TotalCount, nil
}

// EventCount returns the number of events stored for a device.
func (c *Client) EventCount(ctx context.Context, device string) (int64, error) {
	var out countResponse
	if err := c.get(ctx, Data, path("event", "count", "device", "name", device), nil, &out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

// ReadingCount returns the number of readings stored for a device.
func (c *Client) ReadingCount(ctx context.Context, device string) (int64, error) {
	var out countResponse
	if err := c.get(ctx, Data, path("reading", "count", "device", "name", device), nil, &out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

// DeviceCommands returns the core commands of one device (core-command).
func (c *Client) DeviceCommands(ctx context.Context, device string) (*DeviceCoreCommand, error) {
	var out deviceCoreCommandResponse
	if err := c.get(ctx, Command, path("device", "name", device), nil, &out); err != nil {
		return nil, err
	}
	return &out.DeviceCoreCommand, nil
}

// AllDeviceCommands lists devices with their core commands (core-command).
func (c *Client) AllDeviceCommands(ctx context.Context, p Page) ([]DeviceCoreCommand, int, error) {
	var out multiDeviceCoreCommandsResponse
	if err := c.get(ctx, Command, path("device", "all"), c.pageQuery(p), &out); err != nil {
		return nil, 0, err
	}
	return out.DeviceCoreCommands, out.TotalCount, nil
}

// ReadCommand issues a core-command GET, which makes the device service read
// the physical device. It always sends ds-pushevent=false (no event is
// published or persisted), ds-returnevent=true and ds-regexcmd=false (the
// command name is never treated as a regular expression).
func (c *Client) ReadCommand(ctx context.Context, device, command string) (*Event, error) {
	q := url.Values{}
	q.Set("ds-pushevent", "false")
	q.Set("ds-returnevent", "true")
	q.Set("ds-regexcmd", "false")
	var out eventResponse
	if err := c.get(ctx, Command, path("device", "name", device, command), q, &out); err != nil {
		return nil, err
	}
	if out.Event == nil {
		return &Event{DeviceName: device, SourceName: command}, nil
	}
	return out.Event, nil
}

func setLabels(q url.Values, labels []string) {
	var clean []string
	for _, l := range labels {
		if l = strings.TrimSpace(l); l != "" {
			clean = append(clean, l)
		}
	}
	if len(clean) > 0 {
		q.Set("labels", strings.Join(clean, ","))
	}
}
