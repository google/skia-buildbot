package internal

import (
	"context"

	pb "go.skia.org/infra/perf/go/autobisection/proto/v1"
	"go.skia.org/infra/perf/go/backend/client"
)

type AutobisectionServiceActivity struct {
	insecure_conn bool
}

func NewAutobisectionServiceActivity() *AutobisectionServiceActivity {
	return &AutobisectionServiceActivity{
		insecure_conn: false,
	}
}

func (bsa *AutobisectionServiceActivity) SaveAutobisection(ctx context.Context, autobisectionServiceUrl string, req *pb.SaveAutobisectionRequest) (*pb.SaveAutobisectionResponse, error) {
	apiClient, err := client.NewAutobisectionServiceClient(autobisectionServiceUrl, bsa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := apiClient.SaveAutobisection(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
