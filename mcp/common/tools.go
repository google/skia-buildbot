package common

import "github.com/invopop/jsonschema"

// Tool defines a struct for specifying a MCP tool.
type Tool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	InputSchema jsonschema.Schema `json:"input_schema"`
}

// CallToolRequest defines a struct for invoking a tool.
type CallToolRequest struct {
	ToolName string `json:"tool_name"`

	// TODO(ashwinpv): Not sure if the map would suffice.
	Arguments map[string]string `json:"arguments"`
}

// CallToolResponse defines a struct that contains the response
// from a tool invocation.
type CallToolResponse struct {
	Result string `json:"result"`
}
