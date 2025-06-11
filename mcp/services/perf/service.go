package perf

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/perf/anomalies"
	lcp "go.skia.org/infra/mcp/services/perf/chromeperf"
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
	return append(anomalies.GetTools(&s.chromePerfClient),
		append(pinpoint.GetTools(), lcp.GetTools()...)...)
}

func (s *PerfService) Shutdown() error {
	// TODO(jeffyoon): Perform any necessary shutdown.
	sklog.Infof("Shutting down perf service")
	return nil
}
