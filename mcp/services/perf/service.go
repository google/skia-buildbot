package perf

import (
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
)

type PerfService struct {
}

// Initialize the service with the provided arguments.
func (s PerfService) Init(serviceArgs string) error {
	return nil
}

// GetTools returns the supported tools by the service.
func (s PerfService) GetTools() []common.Tool {
	return pinpoint.GetTools()
}
