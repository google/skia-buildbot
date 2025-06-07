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

type ToolArgumentType int

const (
	StringArgument = iota
	BooleanArgument
	NumberArgument
	ObjectArgument
	ArrayArgument
)

// ToolArgument defines a struct for each argument of the tool.
type ToolArgument struct {
	Name         string
	Description  string
	Required     bool
	ArgumentType ToolArgumentType
	// If non-empty, restricts the argument value to one of the stored values.
	EnumValues []string
	// Should have at minimum a "type" set, e.g. {"type": "string"}.
	ArraySchema map[string]any
}
