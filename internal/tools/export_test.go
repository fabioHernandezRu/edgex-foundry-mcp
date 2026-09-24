package tools

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ValidateWriteToolForTest exposes validateWriteTool to external tests.
var ValidateWriteToolForTest = validateWriteTool

// WriteToolsForTest returns the write tool definitions.
func WriteToolsForTest() []*mcp.Tool {
	var out []*mcp.Tool
	for _, r := range writeTools(nil, nil) {
		out = append(out, r.tool)
	}
	return out
}
