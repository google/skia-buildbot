package perf

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/perf/anomalies"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
	"go.skia.org/infra/perf/go/chromeperf"
)

type PerfService struct {
	chromePerfClient chromeperf.ChromePerfClient
}

// Initialize the service with the provided arguments.
func (s *PerfService) Init(serviceArgs string) error {
	ctx := context.Background()
	var err error
	s.chromePerfClient, err = chromeperf.NewChromePerfClient(ctx, "", true)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	return nil
}

// GetTools returns the supported tools by the service.
func (s PerfService) GetTools() []common.Tool {
	return append(pinpoint.GetTools(), anomalies.GetTools(&s.chromePerfClient)...)
}
