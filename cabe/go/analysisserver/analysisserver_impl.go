package analysisserver

import (
	"context"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"google.golang.org/api/bigquery/v2"

	"go.skia.org/infra/go/sklog"

	cpb "go.skia.org/infra/cabe/go/proto"
)

type analysisServerImpl struct {
	cpb.UnimplementedAnalysisServer
	rbeClients map[string]*rbeclient.Client
	bqClient   *bigquery.Service
}

// New returns a new instance of AnalysisServer.
func New(rbeClients map[string]*rbeclient.Client, bqClient *bigquery.Service) *analysisServerImpl {
	return &analysisServerImpl{
		rbeClients: rbeClients,
		bqClient:   bqClient,
	}
}

// GetAnalysis returns the results of a performance experiment analysis.
func (s *analysisServerImpl) GetAnalysis(context.Context, *cpb.GetAnalysisRequest) (*cpb.GetAnalysisResponse, error) {
	sklog.Errorf("Not yet implemented")
	ret := &cpb.GetAnalysisResponse{}

	return ret, nil
}
