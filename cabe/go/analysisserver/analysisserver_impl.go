package analysisserver

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/cabe/go/analyzer"
	"go.skia.org/infra/cabe/go/backends"
	cpb "go.skia.org/infra/cabe/go/proto"
)

type analysisServerImpl struct {
	cpb.UnimplementedAnalysisServer
	swarmingTaskReader backends.SwarmingTaskReader
	casResultReader    backends.CASResultReader
}

// New returns a new instance of AnalysisServer.
func New(casResultReader backends.CASResultReader, swarmingTaskReader backends.SwarmingTaskReader) cpb.AnalysisServer {
	return &analysisServerImpl{
		swarmingTaskReader: swarmingTaskReader,
		casResultReader:    casResultReader,
	}
}

// GetAnalysis returns the results of a performance experiment analysis.
func (s *analysisServerImpl) GetAnalysis(ctx context.Context, req *cpb.GetAnalysisRequest) (*cpb.GetAnalysisResponse, error) {
	if req.GetPinpointJobId() == "" {
		return nil, fmt.Errorf("bad request: missing pinpoint_job_id")
	}
	a := analyzer.New(
		req.GetPinpointJobId(),
		analyzer.WithSwarmingTaskReader(s.swarmingTaskReader),
		analyzer.WithCASResultReader(s.casResultReader),
	)

	if _, err := a.Run(ctx); err != nil {
		sklog.Errorf("running analyzer: %#v", err)
		return nil, err
	}

	res := a.AnalysisResults()

	ret := &cpb.GetAnalysisResponse{
		Results: res,
	}

	return ret, nil
}
