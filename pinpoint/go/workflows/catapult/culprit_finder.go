package catapult

import (
	"errors"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
)

// CulpritFinderWorkflow confirms if an anomaly is a real regression, finds culprits for the
// regression and then verifies the culprit is real.
// This workflow is also known as the sandwich verification workflow
// TODO(b/322202740): Move this workflow out of the catapult folder and into the internal folder
// prior to deprecating the catapult directory.
func CulpritFinderWorkflow(ctx workflow.Context, cfp *workflows.CulpritFinderParams) (*pinpoint_proto.CulpritFinderExecution, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	pp := workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			StartCommit: &pinpoint_proto.CombinedCommit{
				Main: midpoint.NewChromiumCommit(cfp.Request.StartGitHash),
			},
			EndCommit: &pinpoint_proto.CombinedCommit{
				Main: midpoint.NewChromiumCommit(cfp.Request.EndGitHash),
			},
			Configuration:        cfp.Request.Configuration,
			Benchmark:            cfp.Request.Benchmark,
			Story:                cfp.Request.Story,
			Chart:                cfp.Request.Chart,
			Statistic:            cfp.Request.Statistic,
			InitialAttemptCount:  "30",
			ImprovementDirection: cfp.Request.ImprovementDirection,
		},
	}

	var pe *pinpoint_proto.PairwiseExecution
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.PairwiseWorkflow, pp).Get(ctx, &pe); err != nil {
		return nil, skerr.Wrap(err)
	}

	// no regression found, no bug creation necessary
	if !pe.Significant {
		return &pinpoint_proto.CulpritFinderExecution{
			RegressionVerified: false,
		}, nil
	}

	bp := &workflows.BisectParams{
		Request: &pinpoint_proto.ScheduleBisectRequest{
			ComparisonMode:       "performance",
			StartGitHash:         cfp.Request.StartGitHash,
			EndGitHash:           cfp.Request.EndGitHash,
			Configuration:        cfp.Request.Configuration,
			Benchmark:            cfp.Request.Benchmark,
			Story:                cfp.Request.Story,
			Chart:                cfp.Request.Chart,
			AggregationMethod:    cfp.Request.Statistic,
			ComparisonMagnitude:  cfp.Request.ComparisonMagnitude,
			InitialAttemptCount:  "20",
			ImprovementDirection: cfp.Request.ImprovementDirection,
		},
		Production: cfp.Production,
	}
	var be *pinpoint_proto.BisectExecution
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.CatapultBisect, bp).Get(ctx, &be); err != nil {
		return nil, skerr.Wrap(err)
	}

	// no culprits found, is there a bug?
	// TODO(b/340235131): call culprit processing (if necessary)
	if be.Culprits == nil || len(be.Culprits) == 0 {
		return &pinpoint_proto.CulpritFinderExecution{
			RegressionVerified: true,
		}, nil
	}

	verifiedCulprits, err := verifyCulprits(ctx, be, cfp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(b/340235131): call culprit processing
	return &pinpoint_proto.CulpritFinderExecution{
		RegressionVerified: true,
		Culprits:           verifiedCulprits,
	}, nil
}

func verifyCulprits(ctx workflow.Context, be *pinpoint_proto.BisectExecution, cfp *workflows.CulpritFinderParams) ([]*pinpoint_proto.CombinedCommit, error) {
	rc := workflow.NewBufferedChannel(ctx, len(be.Culprits))
	ec := workflow.NewBufferedChannel(ctx, len(be.Culprits))
	wg := workflow.NewWaitGroup(ctx)
	wg.Add(len(be.Culprits))
	for _, culpritPair := range be.DetailedCulprits {
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			exec, err := runCulpritVerification(gCtx, culpritPair, cfp)

			if err != nil {
				ec.Send(gCtx, err)
				return
			}
			rc.Send(gCtx, exec)
		})

	}

	wg.Wait(ctx)
	rc.Close()
	ec.Close()

	if errs := fetchAllFromChannel[error](ctx, ec); len(errs) != 0 {
		return nil, skerr.Wrapf(errors.Join(errs...), "terminal errors found")
	}

	culpritVerifyExecs := fetchAllFromChannel[*pinpoint_proto.PairwiseExecution](ctx, rc)

	verifiedCulprits := []*pinpoint_proto.CombinedCommit{}
	for _, exec := range culpritVerifyExecs {
		if exec.Culprit != nil {
			verifiedCulprits = append(verifiedCulprits, exec.Culprit)
		}
	}

	return verifiedCulprits, nil
}

// runCulpritVerification triggers a culprit verification workflow and returns the result
func runCulpritVerification(ctx workflow.Context, culpritPair *pinpoint_proto.Culprit, cfp *workflows.CulpritFinderParams) (*pinpoint_proto.PairwiseExecution, error) {
	pp := workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			StartCommit:          culpritPair.Prior,
			EndCommit:            culpritPair.Culprit,
			Configuration:        cfp.Request.Configuration,
			Benchmark:            cfp.Request.Benchmark,
			Story:                cfp.Request.Story,
			Chart:                cfp.Request.Chart,
			Statistic:            cfp.Request.Statistic,
			InitialAttemptCount:  "30",
			ImprovementDirection: cfp.Request.ImprovementDirection,
		},
		CulpritVerify: true,
	}

	var pe *pinpoint_proto.PairwiseExecution
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.PairwiseWorkflow, pp).Get(ctx, &pe); err != nil {
		return nil, err
	}
	return pe, nil
}

func fetchAllFromChannel[T any](ctx workflow.Context, rc workflow.ReceiveChannel) []T {
	ln := rc.Len()
	runs := make([]T, ln)
	for i := 0; i < ln; i++ {
		rc.Receive(ctx, &runs[i])
	}
	return runs
}
