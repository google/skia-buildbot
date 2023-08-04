package analysisserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
		return nil, status.Errorf(codes.InvalidArgument, "bad request: missing pinpoint_job_id")
	}

	a := analyzer.New(
		req.GetPinpointJobId(),
		analyzer.WithSwarmingTaskReader(s.swarmingTaskReader),
		analyzer.WithCASResultReader(s.casResultReader),
		analyzer.WithExperimentSpec(req.ExperimentSpec),
	)
	sklog.Infof("Pinpoint job %v GetAnalysis", req.GetPinpointJobId())

	c := analyzer.NewChecker(analyzer.DefaultCheckerOpts...)
	if err := a.RunChecker(ctx, c); err != nil {
		sklog.Errorf("run checker error: %v", err)
	}
	sklog.Infof("checker findings: %v", c.Findings())

	if _, err := a.Run(ctx); err != nil {
		sklog.Errorf("running analyzer: %#v", err)
		return nil, status.Errorf(codes.Internal, "analyzer error: %v", err)
	}

	res := a.AnalysisResults()

	ret := &cpb.GetAnalysisResponse{
		Results: res,
	}

	if req.ExperimentSpec == nil {
		ret.InferredExperimentSpec = a.ExperimentSpec()
	}
	return ret, nil
}
