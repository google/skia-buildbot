// Package backends provides common logic to establish connections to backend RPC services that cabe depends on.

package backends

import (
	"context"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	grpc_oauth "google.golang.org/grpc/credentials/oauth"

	"go.skia.org/infra/perf/go/perfresults"
)

var (
	scopesForBackends = []string{
		compute.CloudPlatformScope,
	}
)

// CASResultReader is an interface for getting PerfResults for CAS instance and root digest values.
type CASResultReader func(context.Context, string, string) (map[string]perfresults.PerfResults, error)

// SwarmingTaskReader is an interface for getting Swarming task metadata associated with a pinpoint job.
type SwarmingTaskReader func(context.Context, string) ([]*apipb.TaskRequestMetadataResponse, error)

func outboundAuthTokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	ts, err := google.DefaultTokenSource(ctx, scopesForBackends...)
	if err != nil {
		sklog.Errorf("getting token source: %v", err)
		return nil, err
	}
	return ts, nil
}

func outboundGRPCCreds(ctx context.Context) (*grpc_oauth.TokenSource, error) {
	ts, err := outboundAuthTokenSource(ctx)
	if err != nil {
		return nil, err
	}
	return &grpc_oauth.TokenSource{
		TokenSource: ts,
	}, nil
}
