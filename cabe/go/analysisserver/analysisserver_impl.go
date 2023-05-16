package analysisserver

import (
	"context"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"go.skia.org/infra/go/swarming"

	"go.skia.org/infra/go/sklog"

	cpb "go.skia.org/infra/cabe/go/proto"
)

type analysisServerImpl struct {
	cpb.UnimplementedAnalysisServer
	rbeClients     map[string]*rbeclient.Client
	swarmingClient swarming.ApiClient
}

// New returns a new instance of AnalysisServer.
func New(rbeClients map[string]*rbeclient.Client, swarmingClient swarming.ApiClient) cpb.AnalysisServer {
	return &analysisServerImpl{
		rbeClients:     rbeClients,
		swarmingClient: swarmingClient,
	}
}

// GetAnalysis returns the results of a performance experiment analysis.
func (s *analysisServerImpl) GetAnalysis(context.Context, *cpb.GetAnalysisRequest) (*cpb.GetAnalysisResponse, error) {
	sklog.Errorf("Not yet implemented")
	ret := &cpb.GetAnalysisResponse{}

	return ret, nil
}
