package common

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

type Resource struct {
	// Unique identifier for the resource. Eg: perf://static/pinpoint/readme
	Uri string

	// Name of the resource.
	Name string

	// Description of the resource.
	Description string

	// Mime type for the resource.
	MimeType string

	// Handler that is invoked when reading the resource.
	Handler func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)
}
