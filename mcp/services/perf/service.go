package perf

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/perf/anomalies"
	lcp "go.skia.org/infra/mcp/services/perf/chromeperf"
	pc "go.skia.org/infra/mcp/services/perf/common"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
	"go.skia.org/infra/perf/go/chromeperf"
)

type PerfService struct {
	chromePerfClient chromeperf.ChromePerfClient
	httpClient       *http.Client
}

// Initialize the service with the provided arguments.
func (s *PerfService) Init(serviceArgs string) error {
	ctx := context.Background()
	var err error
	s.chromePerfClient, err = chromeperf.NewChromePerfClient(ctx, "", true)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	s.httpClient, err = pc.DefaultHttpClient(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to create http client")
	}
	return nil
}

// GetTools returns the supported tools by the service.
func (s *PerfService) GetTools() []common.Tool {
	return append(anomalies.GetTools(&s.chromePerfClient),
		append(pinpoint.GetTools(s.httpClient), lcp.GetTools(s.httpClient)...)...)
}

func (s *PerfService) Shutdown() error {
	// TODO(jeffyoon): Perform any necessary shutdown.
	sklog.Infof("Shutting down perf service")
	return nil
}

func (s *PerfService) GetResources() []common.Resource {
	return []common.Resource{}
}
