// Package backends provides common logic to establish connections to backend RPC services that cabe depends on.

package backends

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	grpc_oauth "google.golang.org/grpc/credentials/oauth"
)

var (
	scopesForBackends = []string{
		compute.CloudPlatformScope,
	}
)

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
