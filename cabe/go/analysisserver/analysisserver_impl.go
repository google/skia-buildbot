package analysisserver

import (
	"context"

	"go.skia.org/infra/go/sklog"

	cpb "go.skia.org/infra/cabe/go/proto"
)

type analysisServerImpl struct {
	cpb.UnimplementedAnalysisServer
}

// New returns a new instance of AnalysisServer.
func New() *analysisServerImpl {
	return &analysisServerImpl{}
}

// GetAnalysis returns the results of a performance experiment analysis.
func (s *analysisServerImpl) GetAnalysis(context.Context, *cpb.GetAnalysisRequest) (*cpb.GetAnalysisResponse, error) {
	sklog.Errorf("Not yet implemented")
	ret := &cpb.GetAnalysisResponse{}

	return ret, nil
}
