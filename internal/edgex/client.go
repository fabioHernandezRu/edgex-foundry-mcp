// Package edgex is a small, read-only HTTP client for the EdgeX Foundry
// REST API v3 (core-metadata, core-data, core-command). It knows base URLs,
// the gateway route prefixes, the bearer token, timeouts and error decoding.
// It has no knowledge of MCP.
package edgex

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Service identifies an EdgeX core service. Its value is also the API gateway
// route prefix and the service name EdgeX reports.
type Service string

// EdgeX core services used by this client.
const (
	Metadata Service = "core-metadata"
	Data     Service = "core-data"
	Command  Service = "core-command"
)

// Services lists the core services in a stable order.
var Services = []Service{Metadata, Data, Command}

const (
	apiBase = "/api/v3"
	// maxBodyBytes bounds how much of any EdgeX response is read.
	maxBodyBytes = 16 << 20
	// maxErrorExcerpt bounds error text taken from response bodies.
	maxErrorExcerpt = 200
	// DefaultLimit is the page size used when a caller passes a non-positive limit.
	DefaultLimit = 20
)

// Options configures a Client.
type Options struct {
	MetadataURL string
	DataURL     string
	CommandURL  string
	// GatewayURL, when set, routes every service through the secure-mode API
	// gateway at <GatewayURL>/<service>/api/v3/...
	GatewayURL string
	// Token is sent as "Authorization: Bearer <Token>" when non-empty.
	Token string
	// CAFile is an optional PEM bundle trusted in addition to system roots.
	CAFile string
	// Timeout bounds every request. Required.
	Timeout time.Duration
	// MaxResults caps every limit sent to EdgeX (1..1024).
	MaxResults int
	// Logger receives one debug line per request (never headers or bodies).
	Logger *slog.Logger
	// HTTPClient overrides the HTTP client (tests). Its Timeout is not relied on.
	HTTPClient *http.Client
}

// Client is a read-only EdgeX REST API v3 client. It is safe for concurrent use.
type Client struct {
	base       map[Service]string
	token      string
	timeout    time.Duration
	maxResults int
	httpc      *http.Client
	log        *slog.Logger
}

// New builds a Client.
func New(o Options) (*Client, error) {
	if o.Timeout <= 0 {
		return nil, errors.New("edgex: timeout must be positive")
	}
	if o.MaxResults <= 0 {
		return nil, errors.New("edgex: max results must be positive")
	}
	c := &Client{
		base:       map[Service]string{},
		token:      o.Token,
		timeout:    o.Timeout,
		maxResults: o.MaxResults,
		httpc:      o.HTTPClient,
		log:        o.Logger,
	}
	if c.log == nil {
		c.log = slog.New(slog.DiscardHandler)
	}
	if o.GatewayURL != "" {
		gw := strings.TrimRight(o.GatewayURL, "/")
		for _, s := range Services {
			c.base[s] = gw + "/" + string(s)
		}
	} else {
		c.base[Metadata] = strings.TrimRight(o.MetadataURL, "/")
		c.base[Data] = strings.TrimRight(o.DataURL, "/")
		c.base[Command] = strings.TrimRight(o.CommandURL, "/")
	}
	if c.httpc == nil {
		tr, err := newTransport(o.CAFile)
		if err != nil {
			return nil, err
		}
		c.httpc = &http.Client{Transport: tr, Timeout: o.Timeout}
	}
	return c, nil
}

func newTransport(caFile string) (*http.Transport, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		pem, err := os.ReadFile(caFile) // #nosec G304 -- path is operator-supplied configuration
		if err != nil {
			return nil, fmt.Errorf("edgex: reading CA file: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("edgex: no PEM certificates found in %s", caFile)
		}
		tr.TLSClientConfig.RootCAs = pool
	}
	return tr, nil
}

// BaseURL returns the base URL used for a service (without /api/v3).
func (c *Client) BaseURL(s Service) string { return c.base[s] }

// Timeout returns the per-request timeout.
func (c *Client) Timeout() time.Duration { return c.timeout }

// MaxResults returns the configured result cap.
func (c *Client) MaxResults() int { return c.maxResults }

// Page selects a window of a list result.
type Page struct {
	Offset int
	Limit  int
}

// ClampLimit returns the limit actually sent to EdgeX: def when limit is
// non-positive, never above the configured cap, and never -1.
func (c *Client) ClampLimit(limit, def int) int {
	if def <= 0 {
		def = DefaultLimit
	}
	if limit <= 0 {
		limit = def
	}
	if limit > c.maxResults {
		limit = c.maxResults
	}
	return limit
}

func (c *Client) pageQuery(p Page) url.Values {
	q := url.Values{}
	off := p.Offset
	if off < 0 {
		off = 0
	}
	q.Set("offset", strconv.Itoa(off))
	q.Set("limit", strconv.Itoa(c.ClampLimit(p.Limit, DefaultLimit)))
	return q
}

// path joins escaped path segments under /api/v3.
func path(segments ...string) string {
	var b strings.Builder
	b.WriteString(apiBase)
	for _, s := range segments {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(s))
	}
	return b.String()
}

// APIError describes a failed EdgeX call. It never contains headers or tokens.
type APIError struct {
	Service Service
	// BaseURL is the configured base URL of the service.
	BaseURL string
	// Status is the HTTP status code, or 0 when no response was received.
	Status int
	// Message is the EdgeX error message or a short plain-text excerpt.
	Message string
	// Timeout is true when the request exceeded the configured timeout.
	Timeout bool
	// Err is the underlying transport error, if any.
	Err error
}

func (e *APIError) Error() string {
	switch {
	case e.Timeout:
		return fmt.Sprintf("%s did not answer within the timeout (%s)", e.Service, e.BaseURL)
	case e.Status == 0:
		return fmt.Sprintf("%s unreachable at %s: %v", e.Service, e.BaseURL, e.Err)
	case e.Message != "":
		return fmt.Sprintf("%s: HTTP %d: %s", e.Service, e.Status, e.Message)
	default:
		return fmt.Sprintf("%s: HTTP %d", e.Service, e.Status)
	}
}

func (e *APIError) Unwrap() error { return e.Err }

// StatusOf returns the HTTP status of an *APIError in err's chain, or 0.
func StatusOf(err error) int {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status
	}
	return 0
}

// IsNotFound reports whether err is an EdgeX 404.
func IsNotFound(err error) bool { return StatusOf(err) == http.StatusNotFound }

// IsUnauthorized reports whether err is an EdgeX 401 or 403.
func IsUnauthorized(err error) bool {
	s := StatusOf(err)
	return s == http.StatusUnauthorized || s == http.StatusForbidden
}

// IsLocked reports whether err is an EdgeX 423 (locked, or device DOWN).
func IsLocked(err error) bool { return StatusOf(err) == http.StatusLocked }

// get performs a GET on svc and decodes the JSON body into out (if non-nil).
func (c *Client) get(ctx context.Context, svc Service, p string, q url.Values, out any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	u := c.base[svc] + p
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return &APIError{Service: svc, BaseURL: c.base[svc], Err: err}
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	start := time.Now()
	resp, err := c.httpc.Do(req)
	if err != nil {
		c.log.Debug("edgex request failed", "service", svc, "path", p, "latency", time.Since(start))
		ae := &APIError{Service: svc, BaseURL: c.base[svc], Err: stripURL(err)}
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			ae.Timeout = true
		}
		return ae
	}
	defer func() { _ = resp.Body.Close() }()
	c.log.Debug("edgex request", "service", svc, "path", p, "status", resp.StatusCode, "latency", time.Since(start))

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		ae := &APIError{Service: svc, BaseURL: c.base[svc], Status: resp.StatusCode, Err: err}
		if errors.Is(err, context.DeadlineExceeded) {
			ae.Timeout = true
		}
		return ae
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Service: svc, BaseURL: c.base[svc], Status: resp.StatusCode, Message: errorMessage(body)}
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &APIError{Service: svc, BaseURL: c.base[svc], Status: resp.StatusCode, Message: "invalid JSON response", Err: err}
	}
	return nil
}

// stripURL removes the request URL from *url.Error so error strings stay short;
// the service and base URL are reported by APIError itself.
func stripURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// errorMessage extracts the EdgeX BaseResponse message, or a short text excerpt
// for non-JSON bodies such as the plain-text 503 "request timeout".
func errorMessage(body []byte) string {
	var br BaseResponse
	if json.Unmarshal(body, &br) == nil && br.Message != "" {
		return truncate(br.Message, maxErrorExcerpt)
	}
	return truncate(strings.TrimSpace(string(body)), maxErrorExcerpt)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
