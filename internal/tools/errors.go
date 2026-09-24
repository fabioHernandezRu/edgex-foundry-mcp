package tools

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// toolError converts an EdgeX client error into a short, actionable message
// returned to the model as an MCP tool error (isError: true). notFound is used
// for 404 responses when non-empty. Messages never contain headers or tokens.
// mappedError carries a model-facing message while keeping the original
// error chain (e.g. for edgex.StatusOf in audit logs).
type mappedError struct {
	msg   string
	cause error
}

func (e *mappedError) Error() string { return e.msg }
func (e *mappedError) Unwrap() error { return e.cause }

func toolError(err error, notFound string) error {
	var ae *edgex.APIError
	if !errors.As(err, &ae) {
		return err
	}
	return &mappedError{msg: mapError(err, notFound).Error(), cause: err}
}

func mapError(err error, notFound string) error {
	var ae *edgex.APIError
	if !errors.As(err, &ae) {
		return err
	}
	switch {
	case ae.Timeout:
		return fmt.Errorf("%s at %s did not answer within the configured timeout; the service may be overloaded or unreachable", ae.Service, ae.BaseURL)
	case ae.Status == 0:
		return fmt.Errorf("%s is unreachable at %s (%w); check that EdgeX is running and the URL is correct", ae.Service, ae.BaseURL, ae.Err)
	case ae.Status == http.StatusNotFound && notFound != "":
		return errors.New(notFound)
	case ae.Status == http.StatusUnauthorized || ae.Status == http.StatusForbidden:
		return fmt.Errorf("%s rejected the request (HTTP %d): EdgeX is probably in secure mode; configure --gateway-url and a valid JWT via EDGEX_TOKEN or --token-file", ae.Service, ae.Status)
	case ae.Status == http.StatusLocked:
		return fmt.Errorf("device or service is locked or down (HTTP 423): %s", ae.Message)
	case ae.Status == http.StatusRequestedRangeNotSatisfiable:
		return errors.New("offset is past the end of the result set; use a smaller offset")
	case ae.Status == http.StatusServiceUnavailable:
		return fmt.Errorf("%s is unavailable or timed out (HTTP 503): %s", ae.Service, ae.Message)
	default:
		return fmt.Errorf("%s returned HTTP %d: %s", ae.Service, ae.Status, ae.Message)
	}
}
