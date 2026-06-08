package internal

import (
	"context"

	pb "go.skia.org/infra/perf/go/autobisection/proto/v1"
	"go.skia.org/infra/perf/go/backend/client"
)

type AutobisectionServiceActivity struct {
	insecureConn bool
}

func NewAutobisectionServiceActivity(insecure bool) *AutobisectionServiceActivity {
	return &AutobisectionServiceActivity{
		insecureConn: insecure,
	}
}

func (bsa *AutobisectionServiceActivity) SaveAutobisection(ctx context.Context, autobisectionServiceUrl string, req *pb.SaveAutobisectionRequest) (*pb.SaveAutobisectionResponse, error) {
	apiClient, err := client.NewAutobisectionServiceClient(autobisectionServiceUrl, bsa.insecureConn)
	if err != nil {
		return nil, err
	}
	resp, err := apiClient.SaveAutobisection(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
