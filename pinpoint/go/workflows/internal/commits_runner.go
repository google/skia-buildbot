package internal

import (
	"context"
	"errors"

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

func buildChrome(ctx workflow.Context, jobID, bot, benchmark string, commit *midpoint.CombinedCommit) (*workflows.Build, error) {
	t, err := bot_configs.GetIsolateTarget(bot, benchmark)
	if err != nil {
		return nil, skerr.Wrapf(err, "no target found for (%s, %s)", bot, benchmark)
	}

	var b *workflows.Build
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.BuildChrome, workflows.BuildChromeParams{
		PinpointJobID: jobID,
		Device:        bot,
		Target:        t,
		Commit:        commit,
	}).Get(ctx, &b); err != nil {
		return nil, skerr.Wrap(err)
	}

	if b.Status != buildbucketpb.Status_SUCCESS {
		return nil, skerr.Fmt("build fails at commit %v", commit)
	}

	return b, nil
}

func fetchAllFromChannel[T any](ctx workflow.Context, rc workflow.ReceiveChannel) []T {
	ln := rc.Len()
	runs := make([]T, ln)
	for i := 0; i < ln; i++ {
		rc.Receive(ctx, &runs[i])
	}
	return runs
}

func runBenchmark(ctx workflow.Context, cc *midpoint.CombinedCommit, cas *swarmingV1.SwarmingRpcsCASReference, scrp *SingleCommitRunnerParams) (*workflows.TestRun, error) {
	var tr *workflows.TestRun
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.RunBenchmark, &RunBenchmarkParams{
		JobID:     scrp.PinpointJobID,
		Commit:    cc,
		BotConfig: scrp.BotConfig,
		BuildCAS:  cas,
		Benchmark: scrp.Benchmark,
		Story:     scrp.Story,
		StoryTags: scrp.StoryTags,
	}).Get(ctx, &tr); err != nil {
		return nil, err
	}

	if !run_benchmark.IsTaskStateSuccess(tr.Status) {
		return nil, skerr.Fmt("test run (%v) failed with status (%v)", tr.TaskID, tr.Status)
	}

	// TODO(b/327224992): Should surface CAS errors here in case the test results don't exist.
	var lv []float64
	if err := workflow.ExecuteActivity(ctx, CollectValuesActivity, tr, scrp.Benchmark, scrp.Chart, scrp.AggregationMethod).Get(ctx, &lv); err != nil {
		return nil, err
	}

	tr.Values = map[string][]float64{
		scrp.Chart: lv,
	}
	return tr, nil
}

// SingleCommitRunner is a Workflow definition.
//
// SingleCommitRunner builds, runs and collects benchmark sampled values from one single commit.
func SingleCommitRunner(ctx workflow.Context, sc *SingleCommitRunnerParams) (*CommitRun, error) {
	bctx := workflow.WithChildOptions(ctx, buildWorkflowOptions)

	b, err := buildChrome(bctx, sc.PinpointJobID, sc.BotConfig, sc.Benchmark, sc.CombinedCommit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	rc := workflow.NewBufferedChannel(ctx, int(sc.Iterations))
	ec := workflow.NewBufferedChannel(ctx, int(sc.Iterations))
	wg := workflow.NewWaitGroup(ctx)
	wg.Add(int(sc.Iterations))

	ctx = workflow.WithChildOptions(ctx, runBenchmarkWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	for i := 0; i < int(sc.Iterations); i++ {
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			tr, err := runBenchmark(gCtx, sc.CombinedCommit, b.CAS, sc)

			if err != nil {
				ec.Send(gCtx, err)
				return
			}
			rc.Send(gCtx, tr)
		})
	}

	wg.Wait(ctx)
	rc.Close()
	ec.Close()

	// TODO(b/326480795): We can tolerate a certain number of errors but should also report
	//	test errors.
	if errs := fetchAllFromChannel[error](ctx, ec); len(errs) != 0 {
		return nil, skerr.Wrapf(errors.Join(errs...), "not all iterations are successful")
	}

	runs := fetchAllFromChannel[*workflows.TestRun](ctx, rc)
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
