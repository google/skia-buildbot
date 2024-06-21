package internal

import (
	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/workflow"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

func PairwiseWorkflow(ctx workflow.Context, p *workflows.PairwiseParams) (*pinpoint_proto.PairwiseExecution, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	jobID := uuid.New().String()

	pairwiseRunnerParams := PairwiseCommitsRunnerParams{
		SingleCommitRunnerParams: SingleCommitRunnerParams{
			PinpointJobID:     jobID,
			BotConfig:         p.Request.Configuration,
			Benchmark:         p.Request.Benchmark,
			Chart:             p.Request.Chart,
			Story:             p.Request.Story,
			AggregationMethod: p.Request.Statistic,
			Iterations:        p.GetInitialAttempt(),
		},
		LeftCommit:  (*midpoint.CombinedCommit)(p.Request.StartCommit),
		RightCommit: (*midpoint.CombinedCommit)(p.Request.EndCommit),
	}

	var pr *PairwiseRun
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.PairwiseCommitsRunner, pairwiseRunnerParams).Get(ctx, &pr); err != nil {
		return nil, skerr.Wrap(err)
	}

	pr.removeDataUntilBalanced(p.Request.Chart)

	lValues := pr.Left.AllValues(p.Request.Chart)
	rValues := pr.Right.AllValues(p.Request.Chart)
	var res compare.ComparePairwiseResult
	if err := workflow.ExecuteActivity(ctx, ComparePairwiseActivity, lValues, rValues, p.GetImprovementDirection()).Get(ctx, &res); err != nil {
		return nil, skerr.Wrap(err)
	}

	var significant bool
	var culprit *pinpoint_proto.CombinedCommit
	if res.Verdict == compare.Different {
		significant = true
		culprit = (*pinpoint_proto.CombinedCommit)(pairwiseRunnerParams.RightCommit)
	}

	return &pinpoint_proto.PairwiseExecution{
		Significant: significant,
		JobId:       jobID,
		Statistic: &pinpoint_proto.PairwiseExecution_WilcoxonResult{
			PValue:                   res.PValue,
			ConfidenceIntervalLower:  res.LowerCi,
			ConfidenceIntervalHigher: res.UpperCi,
			ControlMedian:            res.XMedian,
			TreatmentMedian:          res.YMedian,
		},
		Culprit: culprit,
	}, nil
}
