package common

// McpService provides an interface for defining a mcp service.
type McpService interface {
	// Initialize the service with the provided arguments.
	Init(serviceArgs string) error

	// GetTools returns all the tools supported by the McpService.
	GetTools() []Tool

	// Shutdown implements shutdown procedure for the service.
	Shutdown() error
}
