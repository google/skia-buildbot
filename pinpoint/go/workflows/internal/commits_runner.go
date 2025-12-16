package internal

import (
	"context"
	"errors"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/read_values"
	"go.skia.org/infra/pinpoint/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
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
	// Note: This field is only used in bisect
	// TODO(b/326352320): move CombinedCommit to a common package or rename midpoint
	CombinedCommit *common.CombinedCommit

	// The number of benchmark tests to run.
	// Note the collected sampled values can be more than iterations as each iteration produce
	// more than one samples.
	Iterations int32

	// Finished number of iterations of benchmark test
	// In the bisect jobs, one commit run will start with an initial number of iteration
	// If the compare result is not significant enough, then extra number of iteration will be added to that commit
	// We need this attribute to record the finished number of iterations
	// Note: This field is only used in bisect
	FinishedIteration int32

	// Extra arguments to be passed to the benchmark runner.
	ExtraArgs []string

	// Available bot list
	BotIds []string
}

// CommitRun stores benchmark tests runs for a single commit
type CommitRun struct {
	// The Chrome build associated with the commit.
	Build *workflows.Build

	// All the benchmark runs using the build.
	Runs []*workflows.TestRun
}

func (cr *CommitRun) AllValues(chart string) []float64 {
	vs := []float64{}
	for _, r := range cr.Runs {
		// Task succeeded but benchmark run failed
		if r.Values == nil {
			continue
		}
		if v, ok := r.Values[chart]; ok {
			vs = append(vs, v...)
		}
	}
	return vs
}

func (cr *CommitRun) AllErrorValues(chart string) []float64 {
	vs := []float64{}
	for _, r := range cr.Runs {
		var v float64
		// Task succeeded but benchmark run failed
		if r.Values == nil {
			v = 1.0
		}
		// Benchmark run succeeded but failed to produce CAS could
		// also be attributed to code changes
		if _, ok := r.Values[chart]; !ok {
			v = 1.0
		}
		vs = append(vs, v) // default append 0.0
	}
	return vs
}

func (cr *CommitRun) GetSwarmingStatus() []*pinpoint_proto.SwarmingTaskStatus {
	status := []*pinpoint_proto.SwarmingTaskStatus{}
	for _, tr := range cr.Runs {
		s := &pinpoint_proto.SwarmingTaskStatus{
			TaskId: tr.TaskID,
			Status: tr.Status.ConvertToProto(),
		}
		status = append(status, s)
	}
	return status
}

func buildChrome(ctx workflow.Context, jobID, bot, benchmark string, commit *common.CombinedCommit) (*workflows.Build, error) {
	t, err := bot_configs.GetIsolateTarget(bot, benchmark)
	if err != nil {
		return nil, skerr.Wrapf(err, "no target found for (%s, %s)", bot, benchmark)
	}

	var patch []*buildbucketpb.GerritChange
	if commit.Patch != nil {
		// Our inputs put patch info inside CombinedCommit, but BuildChrome
		// workflow requires it in a separate field. Also, the objects used to
		// represent a patch are similar but slightly different. So some data
		// movement is needed here.
		patch = append(patch, &buildbucketpb.GerritChange{
			Host:     commit.Patch.Host,
			Project:  commit.Patch.Project,
			Change:   commit.Patch.Change,
			Patchset: commit.Patch.Patchset,
		})
	}

	var b *workflows.Build
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.BuildChrome, workflows.BuildParams{
		WorkflowID: jobID,
		Device:     bot,
		Target:     t,
		Commit:     commit,
		Project:    "chromium",
		Patch:      patch,
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

func runBenchmark(ctx workflow.Context, cc *common.CombinedCommit, cas *apipb.CASReference, scrp *SingleCommitRunnerParams, dimensions map[string]string, iteration int32) (*workflows.TestRun, error) {
	var tr *workflows.TestRun
	rbp := &RunBenchmarkParams{
		JobID:        scrp.PinpointJobID,
		Commit:       cc,
		BotConfig:    scrp.BotConfig,
		BuildCAS:     cas,
		Benchmark:    scrp.Benchmark,
		Story:        scrp.Story,
		StoryTags:    scrp.StoryTags,
		Dimensions:   dimensions,
		IterationIdx: iteration,
		ExtraArgs:    scrp.ExtraArgs,
	}

	if err := workflow.ExecuteChildWorkflow(ctx, workflows.RunBenchmark, rbp).Get(ctx, &tr); err != nil {
		return nil, err
	}

	switch s := tr.Status; {
	case s.IsTaskTerminalFailure():
		// TODO(b/327224992): Handle retry logic for non-terminal benchmark failures
		// For now, assume all retryable errors are terminal
		return nil, skerr.Fmt("test run (%v) terminally failed with status (%v)", tr.TaskID, tr.Status)
	case s.IsTaskBenchmarkFailure(), s.IsTaskTimedOut():
		return tr, nil
	}

	// TODO(b/327224992): Should surface CAS errors here in case the test results don't exist.
	var results *workflows.TestResults
	if scrp.Chart != "" {
		if err := workflow.ExecuteActivity(ctx, CollectValuesActivity, tr, scrp.Benchmark, scrp.Chart, scrp.AggregationMethod).Get(ctx, &results); err != nil {
			return nil, err
		}
	} else {
		if err := workflow.ExecuteActivity(ctx, CollectAllValuesActivity, tr, scrp.Benchmark, scrp.AggregationMethod).Get(ctx, &results); err != nil {
			return nil, err
		}
	}

	tr.Architecture = results.Architecture
	tr.OSName = results.OSName
	tr.Values = results.Values
	tr.Units = results.Units
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
		// We need to make a copy of i since the following is a closure. By making a
		// copy every closure will point to it's own copy of i rather than pointing to
		// the same variable.
		iteration := int32(i)
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()

			botDimensions := getBotDimension(sc.FinishedIteration, iteration, sc.BotIds)
			tr, err := runBenchmark(gCtx, sc.CombinedCommit, b.CAS, sc, botDimensions, iteration)

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

	if errs := fetchAllFromChannel[error](ctx, ec); len(errs) != 0 {
		return nil, skerr.Wrapf(errors.Join(errs...), "terminal errors found")
	}

	runs := fetchAllFromChannel[*workflows.TestRun](ctx, rc)
	return &CommitRun{
		Build: b,
		Runs:  runs,
	}, nil
}

func getBotDimension(finishedIteration int32, iteration int32, botIds []string) map[string]string {
	if len(botIds) == 0 {
		return nil
	}

	botDimension := map[string]string{
		"key":   "id",
		"value": (botIds)[(finishedIteration+iteration)%int32(len(botIds))],
	}
	return botDimension
}

// CollectValuesActivity is an activity to collect sampled values from a single test run.
func CollectValuesActivity(ctx context.Context, run *workflows.TestRun, benchmark, chart, aggMethod string) (*workflows.TestResults, error) {
	client, err := read_values.DialRBECAS(ctx, run.CAS.CasInstance)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to dial rbe client")
	}
	return client.ReadValuesByChart(ctx, benchmark, chart, []*apipb.CASReference{run.CAS}, aggMethod)
}

// CollectAllValuesActivity is an activity to collect all sampled values from a single test run.
func CollectAllValuesActivity(ctx context.Context, run *workflows.TestRun, benchmark, aggMethod string) (*workflows.TestResults, error) {
	client, err := read_values.DialRBECAS(ctx, run.CAS.CasInstance)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to dial rbe client")
	}
	return client.ReadValuesForAllCharts(ctx, benchmark, []*apipb.CASReference{run.CAS}, aggMethod)
}
