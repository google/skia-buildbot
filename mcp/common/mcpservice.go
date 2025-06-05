package common

// McpService provides an interface for defining a mcp service.
type McpService interface {
	// Initialize the service with the provided arguments.
	Init(serviceArgs string) error

	// GetTools returns all the tools supported by the McpService.
	GetTools() []Tool

	// CallTool invokes a tool based on the request.
	CallTool(request CallToolRequest) CallToolResponse
}
