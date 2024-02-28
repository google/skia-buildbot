package internal

import (
	"context"
	"errors"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/read_values"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/workflow"
)

var (
	buildWorkflowOptions = workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 4 * time.Hour,
	}

	// Two hours pending timeouts + six hours running timeouts
	// Those are defined here:
	// https://chromium.googlesource.com/chromium/src/+/3b293fe/testing/buildbot/chromium.perf.json#499
	runBenchmarkWorkflowOptions = workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 8 * time.Hour,
	}
	collectValuesActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
)

// SingleCommitRunnerParams defines the parameters for SingleCommitRunner workflow.
type SingleCommitRunnerParams struct {
	// PinpointJobID is the Job ID to associate with the run.
	PinpointJobID string

	// BotConfig is the name of the device configuration, e.g. "linux-perf".
	BotConfig string

	// Benchmark is the name of the benchmark test.
	Benchmark string

	// Chart is a story histogram in a Benchmark.
	Chart string

	// Story is a story in a Benchmark.
	Story string

	// StoryTags is for the story in a Benchmark.
	StoryTags string

	// AggregationMethod is method to aggregate sampled values.
	// If empty, then the original values are returned.
	AggregationMethod string

	// The commit with optional deps override
	// TODO(b/326352320): move CombinedCommit to a common package or rename midpoint
	CombinedCommit *midpoint.CombinedCommit

	// The number of benchmark tests to run.
	// Note the collected sampled values can be more than iterations as each iteration produce
	// more than one samples.
	Iterations int32
}

// CommitRun stores benchmark tests runs for a single commit
type CommitRun struct {
	// The commit in the chromium repo.
	Commit *midpoint.CombinedCommit

	// The Chrome build associated with the commit.
	Build *workflows.Build

	// All the benchmark runs using the build.
	Runs []*workflows.TestRun
}

func (cr *CommitRun) AllValues(chart string) []float64 {
	vs := []float64{}
	for _, r := range cr.Runs {
		if v, ok := r.Values[chart]; ok {
			vs = append(vs, v...)
		}
	}
	return vs
}

// SingleCommitRunner is a Workflow definition.
//
// SingleCommitRunner builds, runs and collects benchmark sampled values from one single commit.
func SingleCommitRunner(ctx workflow.Context, sc *SingleCommitRunnerParams) (*CommitRun, error) {
	bctx := workflow.WithChildOptions(ctx, buildWorkflowOptions)

	// TODO(b/327222369): Move this into an activity to be mocked and tracked.
	t, err := bot_configs.GetIsolateTarget(sc.BotConfig, sc.Benchmark)
	if err != nil {
		return nil, skerr.Fmt("no target found for (%s, %s)", sc.BotConfig, sc.Benchmark)
	}

	var b *workflows.Build
	if err := workflow.ExecuteChildWorkflow(bctx, workflows.BuildChrome, workflows.BuildChromeParams{
		PinpointJobID: sc.PinpointJobID,
		Device:        sc.BotConfig,
		Target:        t,
		Commit:        sc.CombinedCommit,
	}).Get(bctx, &b); err != nil {
		return nil, skerr.Wrap(err)
	}

	if b.Status != buildbucketpb.Status_SUCCESS {
		return nil, skerr.Fmt("build fails at commit %v", sc.CombinedCommit)
	}

	rc := workflow.NewBufferedChannel(ctx, int(sc.Iterations))
	ec := workflow.NewBufferedChannel(ctx, int(sc.Iterations))
	wg := workflow.NewWaitGroup(ctx)
	wg.Add(int(sc.Iterations))

	for i := 0; i < int(sc.Iterations); i++ {
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			gCtx = workflow.WithChildOptions(gCtx, runBenchmarkWorkflowOptions)
			gCtx = workflow.WithActivityOptions(gCtx, collectValuesActivityOptions)

			var tr *workflows.TestRun
			if err := workflow.ExecuteChildWorkflow(gCtx, workflows.RunBenchmark, &RunBenchmarkParams{
				JobID:     sc.PinpointJobID,
				Commit:    sc.CombinedCommit,
				BotConfig: sc.BotConfig,
				BuildCAS:  b.CAS,
				Benchmark: sc.Benchmark,
				Story:     sc.Story,
				StoryTags: sc.StoryTags,
			}).Get(gCtx, &tr); err != nil {
				ec.Send(gCtx, err)
				return
			}

			if !run_benchmark.IsTaskStateSuccess(tr.Status) {
				ec.Send(gCtx, skerr.Fmt("test run (%v) failed with status (%v)", tr.TaskID, tr.Status))
				return
			}

			// TODO(b/327224992): Should surface CAS errors here in case the test results don't exist.
			var v []float64
			if err := workflow.ExecuteActivity(gCtx, CollectValuesActivity, tr, sc.Benchmark, sc.Chart, sc.AggregationMethod).Get(gCtx, &v); err != nil {
				ec.Send(gCtx, err)
				return
			}

			tr.Values = map[string][]float64{
				sc.Chart: v,
			}
			rc.Send(gCtx, tr)
		})
	}

	wg.Wait(ctx)
	rc.Close()
	ec.Close()

	errs := make([]error, ec.Len())
	l := ec.Len()
	for i := 0; i < l; i++ {
		var err error
		ec.Receive(ctx, &err)
		errs[i] = err
	}

	// TODO(b/326480795): We can tolerate a certain number of errors but should also report
	//	test errors.
	if len(errs) != 0 {
		return nil, skerr.Wrapf(errors.Join(errs...), "not all iterations are successful")
	}

	runs := make([]*workflows.TestRun, rc.Len())
	l = rc.Len()
	for i := 0; i < l; i++ {
		var r *workflows.TestRun
		rc.Receive(ctx, &r)
		runs[i] = r
	}

	return &CommitRun{
		Commit: sc.CombinedCommit,
		Build:  b,
		Runs:   runs,
	}, nil
}

// PairwiseCommitsRunner is a Workflow definition.
//
// PairwiseCommitsRunner builds, runs and collects benchmark sampled values from several commits.
// It runs the tests in pairs to reduces sample noises.
func PairwiseCommitsRunner(ctx workflow.Context) (*[]CommitRun, error) {
	return nil, skerr.Fmt("not implemented.")
}

// CollectValuesActivity is an activity to collect sampled values from a single test run.
func CollectValuesActivity(ctx context.Context, run *workflows.TestRun, benchmark, chart, aggMethod string) ([]float64, error) {
	client, err := read_values.DialRBECAS(ctx, run.CAS.CasInstance)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to dial rbe client")
	}
	return read_values.ReadValuesByChart(ctx, client, benchmark, chart, []*swarmingV1.SwarmingRpcsCASReference{run.CAS}, aggMethod)
}
