package tools

import (
	"context"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// NoInput is the input of tools that take no arguments.
type NoInput struct{}

// ServiceHealth is the health of one EdgeX core service.
type ServiceHealth struct {
	Service    string `json:"service" jsonschema:"EdgeX service name"`
	Reachable  bool   `json:"reachable" jsonschema:"true when /api/v3/ping answered successfully"`
	Version    string `json:"version,omitempty" jsonschema:"EdgeX service version from /api/v3/version"`
	APIVersion string `json:"apiVersion,omitempty" jsonschema:"REST API version, expected v3"`
	LatencyMs  int64  `json:"latencyMs" jsonschema:"ping round-trip time in milliseconds"`
	Error      string `json:"error,omitempty" jsonschema:"why the service is not healthy"`
}

// HealthOut is the output of system_health.
type HealthOut struct {
	Healthy  bool            `json:"healthy" jsonschema:"true when all core services answered ping"`
	Services []ServiceHealth `json:"services" jsonschema:"per-service health"`
}

func systemHealthTool(c *edgex.Client) registration {
	return reg(&mcp.Tool{
		Name: "system_health",
		Description: "Check the EdgeX core services (core-metadata, core-data, core-command): reachability via /api/v3/ping, " +
			"service version via /api/v3/version and latency. Use this first to confirm the deployment is up and which EdgeX version it runs. Read-only.",
		Annotations: readOnly("EdgeX system health"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ NoInput) (*mcp.CallToolResult, HealthOut, error) {
		out := HealthOut{Healthy: true, Services: make([]ServiceHealth, len(edgex.Services))}
		var wg sync.WaitGroup
		for i, svc := range edgex.Services {
			wg.Add(1)
			go func() {
				defer wg.Done()
				h := ServiceHealth{Service: string(svc)}
				start := time.Now()
				if _, err := c.Ping(ctx, svc); err != nil {
					h.Error = toolError(err, "ping endpoint not found; is this an EdgeX REST API v3 service?").Error()
				} else {
					h.Reachable = true
					h.LatencyMs = time.Since(start).Milliseconds()
					if v, err := c.Version(ctx, svc); err != nil {
						h.Error = "version unavailable: " + toolError(err, "").Error()
					} else {
						h.Version, h.APIVersion = v.Version, v.APIVersion
					}
				}
				out.Services[i] = h
			}()
		}
		wg.Wait()
		for _, h := range out.Services {
			if !h.Reachable {
				out.Healthy = false
			}
		}
		return nil, out, nil
	})
}
