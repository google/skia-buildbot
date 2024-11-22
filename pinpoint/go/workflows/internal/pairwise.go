package internal

import (
	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/workflow"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

func PairwiseWorkflow(ctx workflow.Context, p *workflows.PairwiseParams) (*pinpoint_proto.PairwiseExecution, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	jobID := uuid.New().String()

	// Benchmark runs can sometimes generate an inconsistent number of data points.
	// So even if all benchmark runs were successful, the number of data values
	// generated by commit A vs B will be inconsistent. This can fail the balancing
	// requirement of the statistical test. It can pair up data incorrectly i.e.
	// commit A: [1], [2, 3]
	// commit B: [4, 5], [6]
	// So the analysis will pair up [2,5] together, which are from different runs,
	// violating pairwise analysis
	if p.Request.AggregationMethod == "" {
		p.Request.AggregationMethod = "mean"
	}

	pairwiseRunnerParams := PairwiseCommitsRunnerParams{
		SingleCommitRunnerParams: SingleCommitRunnerParams{
			PinpointJobID:     jobID,
			BotConfig:         p.Request.Configuration,
			Benchmark:         p.Request.Benchmark,
			Chart:             p.Request.Chart,
			Story:             p.Request.Story,
			AggregationMethod: p.Request.AggregationMethod,
			Iterations:        p.GetInitialAttempt(),
		},
		LeftCommit:  (*common.CombinedCommit)(p.Request.StartCommit),
		RightCommit: (*common.CombinedCommit)(p.Request.EndCommit),
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

	// significant variable is explciitly set to false.
	// This value is used in CulpritFinder to determine whether to bisect.
	// Significant = true means that there's indeed a regression and it should
	// be investigated. If significant is not explicitly set to False, we see
	// Temporal workflows with Significant and Culprit omitted because the resolve
	// to nil.
	significant := false
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
