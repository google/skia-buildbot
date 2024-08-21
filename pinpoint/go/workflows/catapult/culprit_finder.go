package catapult

import (
	"errors"
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	perf_wf "go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/api/enums/v1"
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
				Main: common.NewChromiumCommit(cfp.Request.StartGitHash),
			},
			EndCommit: &pinpoint_proto.CombinedCommit{
				Main: common.NewChromiumCommit(cfp.Request.EndGitHash),
			},
			Configuration:        cfp.Request.Configuration,
			Benchmark:            cfp.Request.Benchmark,
			Story:                cfp.Request.Story,
			Chart:                cfp.Request.Chart,
			AggregationMethod:    cfp.Request.AggregationMethod,
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

	magnitude := fmt.Sprintf("%f", pe.Statistic.TreatmentMedian-pe.Statistic.ControlMedian)

	bp := &workflows.BisectParams{
		Request: &pinpoint_proto.ScheduleBisectRequest{
			ComparisonMode:       "performance",
			StartGitHash:         cfp.Request.StartGitHash,
			EndGitHash:           cfp.Request.EndGitHash,
			Configuration:        cfp.Request.Configuration,
			Benchmark:            cfp.Request.Benchmark,
			Story:                cfp.Request.Story,
			Chart:                cfp.Request.Chart,
			AggregationMethod:    cfp.Request.AggregationMethod,
			ComparisonMagnitude:  magnitude,
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

	// call culprit processing
	if err = InvokeCulpritProcessingWorkflow(ctx, cfp, verifiedCulprits); err != nil {
		return nil, skerr.Wrap(err)
	}

	return &pinpoint_proto.CulpritFinderExecution{
		RegressionVerified: true,
		Culprits:           verifiedCulprits,
	}, nil
}

func InvokeCulpritProcessingWorkflow(ctx workflow.Context, cfp *workflows.CulpritFinderParams,
	verified_combined_culprits []*pinpoint_proto.CombinedCommit) error {
	if verified_combined_culprits == nil || len(verified_combined_culprits) == 0 {
		sklog.Debug("No verified culprit is found. No need to invoke culprit processing.")
		return nil
	}

	verified_culprits := make([]*pinpoint_proto.Commit, len(verified_combined_culprits))
	for i, verified_combined_culprit := range verified_combined_culprits {
		verified_culprits[i] = findLastDepCommit(verified_combined_culprit)
	}

	if cfp.CallbackParams == nil || cfp.CallbackParams.CulpritServiceUrl == "" || cfp.CallbackParams.TemporalTaskQueueName == "" {
		sklog.Warningf("Not enough info to invoke culprit processing workflow: %s", cfp.CallbackParams)
		return nil
	}

	child_wf_options := workflow.ChildWorkflowOptions{
		// Assign to the perf task queue.
		TaskQueue:         cfp.CallbackParams.TemporalTaskQueueName,
		ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
	}
	c_ctx := workflow.WithChildOptions(ctx, child_wf_options)

	wf := workflow.ExecuteChildWorkflow(c_ctx, perf_wf.ProcessCulprit, perf_wf.ProcessCulpritParam{
		CulpritServiceUrl: cfp.CallbackParams.CulpritServiceUrl,
		Commits:           verified_culprits,
		AnomalyGroupId:    cfp.CallbackParams.AnomalyGroupId,
	})
	if err := wf.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
		return skerr.Wrapf(err, "Culprit processing workflow failed to start.")
	}
	return nil
}

func findLastDepCommit(combined_commit *pinpoint_proto.CombinedCommit) *pinpoint_proto.Commit {
	var last_dep_commit *pinpoint_proto.Commit
	if combined_commit.ModifiedDeps != nil && len(combined_commit.ModifiedDeps) > 0 {
		last_dep_commit = combined_commit.ModifiedDeps[len(combined_commit.ModifiedDeps)-1]
	} else {
		last_dep_commit = combined_commit.Main
	}
	return last_dep_commit
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
			AggregationMethod:    cfp.Request.AggregationMethod,
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
