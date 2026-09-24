package edgex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Write methods. They are called only from tools in the --enable-writes set.

// SetCommand issues a core-command SET (PUT /api/v3/device/name/{device}/{command})
// with a JSON object mapping resource names to string values. String values
// are the one encoding the device SDK parses correctly for every scalar and
// array type (it applies fmt.Sprint to each value).
func (c *Client) SetCommand(ctx context.Context, device, command string, values map[string]string) error {
	if len(values) == 0 {
		return errors.New("edgex: SET requires at least one value")
	}
	return c.do(ctx, http.MethodPut, Command, path("device", "name", device, command), nil, values, nil)
}

// Device state field names accepted by UpdateDeviceState.
const (
	AdminStateField     = "adminState"
	OperatingStateField = "operatingState"
)

type updateDevice struct {
	Name           string `json:"name"`
	AdminState     string `json:"adminState,omitempty"`
	OperatingState string `json:"operatingState,omitempty"`
}

type updateDeviceRequest struct {
	APIVersion string       `json:"apiVersion"`
	Device     updateDevice `json:"device"`
}

// UpdateDeviceState changes exactly one state field of a device through
// core-metadata PATCH /api/v3/device (a single-item batch). No other device
// field (in particular protocols) is sent. The 207 per-item status is checked.
func (c *Client) UpdateDeviceState(ctx context.Context, device, field, value string) error {
	d := updateDevice{Name: device}
	switch field {
	case AdminStateField:
		d.AdminState = value
	case OperatingStateField:
		d.OperatingState = value
	default:
		return fmt.Errorf("edgex: unknown device state field %q", field)
	}
	if device == "" || value == "" {
		return errors.New("edgex: device and state are required")
	}
	body := []updateDeviceRequest{{APIVersion: "v3", Device: d}}
	var items []BaseResponse
	if err := c.do(ctx, http.MethodPatch, Metadata, path("device"), nil, body, &items); err != nil {
		return err
	}
	if len(items) != 1 {
		return &APIError{Service: Metadata, BaseURL: c.base[Metadata], Status: http.StatusMultiStatus, Message: fmt.Sprintf("expected 1 item in the 207 response, got %d", len(items))}
	}
	if st := items[0].StatusCode; st < 200 || st > 299 {
		return &APIError{Service: Metadata, BaseURL: c.base[Metadata], Status: st, Message: truncate(ScrubURLUserinfo(items[0].Message), maxErrorExcerpt)}
	}
	return nil
}
