package backends

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

// DialBigQuery returns a bigquery Service client.
func DialBigQuery(ctx context.Context) (*bigquery.Service, error) {
	creds, err := outboundGRPCCreds(ctx)
	if err != nil {
		sklog.Errorf("getting grpc creds: %v", err)
		return nil, err
	}

	svc, err := bigquery.NewService(ctx,
		option.WithGRPCDialOption(
			grpc.WithPerRPCCredentials(creds)))
	return svc, err
}
