package perf

import (
	"context"
	"net/http"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/mcp/common"
	sc "go.skia.org/infra/mcp/services/common"
	"go.skia.org/infra/mcp/services/perf/anomalies"
	lcp "go.skia.org/infra/mcp/services/perf/chromeperf"
	"go.skia.org/infra/mcp/services/perf/data"
	"go.skia.org/infra/mcp/services/perf/perfgit"
	"go.skia.org/infra/mcp/services/perf/pinpoint"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/pinpoint/go/backends"
)

type PerfService struct {
	chromePerfClient chromeperf.ChromePerfClient
	httpClient       *http.Client
	crrevClient      *backends.CrrevClientImpl
	serviceArgs      map[string]string
}

// Initialize the service with the provided arguments.
func (s *PerfService) Init(serviceArgs string) error {
	ctx := context.Background()
	var err error
	s.chromePerfClient, err = chromeperf.NewChromePerfClient(ctx, "", true)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	s.httpClient, err = sc.DefaultHttpClient(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to create http client")
	}
	s.crrevClient = backends.NewCrrevClientWithHttpClient(s.httpClient)

	if serviceArgs != "" {
		splits := strings.Split(serviceArgs, ",")
		s.serviceArgs = map[string]string{}
		for _, split := range splits {
			kv := strings.Split(split, "=")
			s.serviceArgs[kv[0]] = kv[1]
		}
	}

	return nil
}

// GetTools returns the supported tools by the service.
func (s *PerfService) GetTools() []common.Tool {
	return append(data.GetTools(s.serviceArgs["perf_url"], s.httpClient),
		append(anomalies.GetTools(&s.chromePerfClient),
			append(pinpoint.GetTools(s.httpClient, s.crrevClient),
				append(lcp.GetTools(s.httpClient),
					perfgit.GetTools(s.httpClient, s.crrevClient)...)...)...)...)
}

func (s *PerfService) Shutdown() error {
	// TODO(jeffyoon): Perform any necessary shutdown.
	sklog.Infof("Shutting down perf service")
	return nil
}

func (s *PerfService) GetResources() []common.Resource {
	return []common.Resource{}
}
