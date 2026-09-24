package tools

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

// ProfileURIPrefix prefixes device profile resource URIs.
const ProfileURIPrefix = "edgex://deviceprofile/"

func registerResources(s *mcp.Server, c *edgex.Client) {
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "deviceprofile",
		Title:       "EdgeX device profile",
		URITemplate: ProfileURIPrefix + "{name}",
		MIMEType:    "application/json",
		Description: "An EdgeX device profile by name, including hidden resources and commands, as compact JSON. Read-only.",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		name, err := url.PathUnescape(strings.TrimPrefix(uri, ProfileURIPrefix))
		if err != nil || name == "" || !strings.HasPrefix(uri, ProfileURIPrefix) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		p, err := c.DeviceProfile(ctx, name)
		if err != nil {
			if edgex.IsNotFound(err) {
				return nil, mcp.ResourceNotFoundError(uri)
			}
			return nil, toolError(err, "")
		}
		b, err := json.Marshal(shapeProfile(p, true))
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(b)}}}, nil
	})
}
