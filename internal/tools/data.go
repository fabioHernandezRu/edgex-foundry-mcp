package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// now is replaceable in tests.
var now = time.Now

// ---- get_latest_readings ----

// LatestReadingsIn is the input of get_latest_readings.
type LatestReadingsIn struct {
	Device   string `json:"device" jsonschema:"device name"`
	Resource string `json:"resource,omitempty" jsonschema:"only readings of this device resource (see get_device_profile)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum readings to return (default 10, capped by the server's --max-results)"`
}

// ReadingsOut is the output of reading tools.
type ReadingsOut struct {
	Device   string `json:"device" jsonschema:"device name"`
	Resource string `json:"resource,omitempty" jsonschema:"resource filter, when given"`
	Start    string `json:"start,omitempty" jsonschema:"resolved range start, RFC 3339 UTC"`
	End      string `json:"end,omitempty" jsonschema:"resolved range end, RFC 3339 UTC"`
	ListInfo
	Readings []ReadingOut `json:"readings" jsonschema:"readings, newest first"`
}

func emptyReadingsHint(device string) string {
	return fmt.Sprintf("no readings found for %q; the device may not be producing data, the resource/time range may not match, or the name may be wrong (see list_devices)", device)
}

func getLatestReadingsTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "get_latest_readings",
		Description: "Get the most recent readings stored in EdgeX core-data for a device, newest first, optionally for one resource. " +
			"Binary values are summarized (media type and size), not returned. Read-only; does not contact the device.",
		Annotations: readOnly("Get latest readings"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in LatestReadingsIn) (*mcp.CallToolResult, ReadingsOut, error) {
		if in.Device == "" {
			return nil, ReadingsOut{}, errors.New("device is required")
		}
		limit := c.ClampLimit(in.Limit, defaultLatestLimit)
		rs, total, err := c.Readings(ctx, edgex.ReadingQuery{Device: in.Device, Resource: in.Resource}, edgex.Page{Limit: limit})
		if err != nil {
			return nil, ReadingsOut{}, toolError(err, "")
		}
		out := ReadingsOut{Device: in.Device, Resource: in.Resource, Readings: shapeReadings(rs)}
		out.ListInfo = listInfo(total, 0, len(out.Readings))
		if len(out.Readings) == 0 {
			out.Hint = emptyReadingsHint(in.Device)
		}
		return nil, out, nil
	})
}

// ---- query_readings ----

// QueryReadingsIn is the input of query_readings.
type QueryReadingsIn struct {
	Device   string `json:"device" jsonschema:"device name"`
	Resource string `json:"resource,omitempty" jsonschema:"only readings of this device resource"`
	Start    string `json:"start,omitempty" jsonschema:"range start, RFC 3339 (use with end; not with window)"`
	End      string `json:"end,omitempty" jsonschema:"range end, RFC 3339 (use with start; not with window)"`
	Window   string `json:"window,omitempty" jsonschema:"range ending now as a Go duration, e.g. 15m, 2h, 24h (not with start/end)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"number of readings to skip (pagination)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum readings to return (default 50, capped by the server's --max-results)"`
}

// resolveRange returns the [start, end] range of a query.
func resolveRange(in QueryReadingsIn) (time.Time, time.Time, error) {
	hasAbs := in.Start != "" || in.End != ""
	switch {
	case in.Window != "" && hasAbs:
		return time.Time{}, time.Time{}, errors.New("window and start/end are mutually exclusive; give either window or both start and end")
	case in.Window != "":
		d, err := time.ParseDuration(in.Window)
		if err != nil || d <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("window must be a positive Go duration such as 15m or 2h, got %q", in.Window)
		}
		end := now().UTC()
		return end.Add(-d), end, nil
	case in.Start != "" && in.End != "":
		start, err := time.Parse(time.RFC3339Nano, in.Start)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("start must be RFC 3339, got %q", in.Start)
		}
		end, err := time.Parse(time.RFC3339Nano, in.End)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("end must be RFC 3339, got %q", in.End)
		}
		if !start.Before(end) {
			return time.Time{}, time.Time{}, errors.New("start must be before end")
		}
		if start.Year() < 1970 || end.Year() > 2261 {
			return time.Time{}, time.Time{}, errors.New("start and end must be between 1970 and 2261 (EdgeX stores nanosecond timestamps)")
		}
		return start.UTC(), end.UTC(), nil
	default:
		return time.Time{}, time.Time{}, errors.New("give either window (e.g. 1h) or both start and end (RFC 3339)")
	}
}

func queryReadingsTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "query_readings",
		Description: "Query readings stored in EdgeX core-data for a device within a time range, newest first, optionally for one resource. " +
			"Give either window (e.g. 1h, ending now) or start and end in RFC 3339. Paginate with offset/nextOffset. Read-only; does not contact the device.",
		Annotations: readOnly("Query readings"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in QueryReadingsIn) (*mcp.CallToolResult, ReadingsOut, error) {
		if in.Device == "" {
			return nil, ReadingsOut{}, errors.New("device is required")
		}
		start, end, err := resolveRange(in)
		if err != nil {
			return nil, ReadingsOut{}, err
		}
		limit := c.ClampLimit(in.Limit, defaultQueryLimit)
		rq := edgex.ReadingQuery{Device: in.Device, Resource: in.Resource, HasRange: true, StartNs: start.UnixNano(), EndNs: end.UnixNano()}
		rs, total, err := c.Readings(ctx, rq, edgex.Page{Offset: in.Offset, Limit: limit})
		if err != nil {
			return nil, ReadingsOut{}, toolError(err, "")
		}
		out := ReadingsOut{
			Device: in.Device, Resource: in.Resource,
			Start: start.Format(time.RFC3339Nano), End: end.Format(time.RFC3339Nano),
			Readings: shapeReadings(rs),
		}
		out.ListInfo = listInfo(total, in.Offset, len(out.Readings))
		if len(out.Readings) == 0 {
			out.Hint = emptyReadingsHint(in.Device)
		}
		return nil, out, nil
	})
}

// ---- device_data_stats ----

// DeviceIn is the input of tools that take one device name.
type DeviceIn struct {
	Device string `json:"device" jsonschema:"device name"`
}

// StatsOut is the output of device_data_stats.
type StatsOut struct {
	Device              string `json:"device" jsonschema:"device name"`
	EventCount          int64  `json:"eventCount" jsonschema:"events stored in core-data for the device"`
	ReadingCount        int64  `json:"readingCount" jsonschema:"readings stored in core-data for the device"`
	LastReadingTime     string `json:"lastReadingTime,omitempty" jsonschema:"time of the newest stored reading, RFC 3339 UTC"`
	LastReadingResource string `json:"lastReadingResource,omitempty" jsonschema:"resource of the newest stored reading"`
}

func deviceDataStatsTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "device_data_stats",
		Description: "Summarize the data EdgeX core-data holds for a device: number of events and readings, and when the newest reading arrived. " +
			"Useful to tell whether a device is reporting. Read-only.",
		Annotations: readOnly("Device data stats"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DeviceIn) (*mcp.CallToolResult, StatsOut, error) {
		if in.Device == "" {
			return nil, StatsOut{}, errors.New("device is required")
		}
		out := StatsOut{Device: in.Device}
		var err error
		if out.EventCount, err = c.EventCount(ctx, in.Device); err != nil {
			return nil, StatsOut{}, toolError(err, "")
		}
		if out.ReadingCount, err = c.ReadingCount(ctx, in.Device); err != nil {
			return nil, StatsOut{}, toolError(err, "")
		}
		rs, _, err := c.Readings(ctx, edgex.ReadingQuery{Device: in.Device}, edgex.Page{Limit: 1})
		if err != nil {
			return nil, StatsOut{}, toolError(err, "")
		}
		if len(rs) > 0 {
			out.LastReadingTime = nsTime(rs[0].Origin)
			out.LastReadingResource = rs[0].ResourceName
		}
		return nil, out, nil
	})
}
