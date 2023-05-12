package backends

import (
	"context"

	"go.skia.org/infra/go/sklog"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

const (
	bqBillingProjectID = "chromeperf"
)

// DialBigQuery returns a bigquery Service client.
func DialBigQuery(ctx context.Context) (*bigquery.Client, error) {
	creds, err := outboundGRPCCreds(ctx)
	if err != nil {
		sklog.Errorf("getting grpc creds: %v", err)
		return nil, err
	}

	svc, err := bigquery.NewClient(ctx, bqBillingProjectID,
		option.WithGRPCDialOption(grpc.WithPerRPCCredentials(creds)))
	return svc, err
}
