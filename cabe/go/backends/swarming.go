package backends

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"golang.org/x/oauth2/google"
)

const (
	swarmingServiceAddress = "chrome-swarming.appspot.com:443"
)

func DialSwarming(ctx context.Context) (swarming.ApiClient, error) {
	// Create authenticated HTTP client.
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	ret, err := swarming.NewApiClient(client, swarmingServiceAddress)

	return ret, err
}
