package common

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// Tool defines a struct for specifying a MCP tool.
type Tool struct {
	Name        string
	Description string
	Arguments   []ToolArgument
	Handler     func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// ToolArgument defines a struct for each argument of the tool.
type ToolArgument struct {
	Name        string
	Description string
	Required    bool
}
