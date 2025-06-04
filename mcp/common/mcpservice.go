package common

import "github.com/go-chi/chi/v5"

// McpService provides an interface for defining a mcp service.
type McpService interface {
	// Initialize the service with the provided arguments.
	Init(serviceArgs string) error

	// Register all api handlers
	RegisterHandlers(*chi.Mux)
}
