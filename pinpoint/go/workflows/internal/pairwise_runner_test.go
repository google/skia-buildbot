package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// generatePairwiseTestRuns generates mock test runs data for PairwiseRunner
//
// It returns the expected runs, and a channel that was buffered to send to mocked workflow.
func generatePairwiseTestRuns(chart string, chartExpectedValues []float64, pairOrder []int) ([]*workflows.PairwiseTestRun, chan *workflows.PairwiseTestRun) {
	iterations := len(pairOrder)
	rc := make(chan *workflows.PairwiseTestRun, iterations)
	ptrs := make([]*workflows.PairwiseTestRun, iterations)
	ptrs[0] = &workflows.PairwiseTestRun{
		FirstTestRun: &workflows.TestRun{
			Status: run_benchmark.State(backends.RunBenchmarkFailure),
		},
		SecondTestRun: &workflows.TestRun{
			Status: run_benchmark.State(backends.RunBenchmarkFailure),
		},
		Permutation: workflows.PairwiseOrder(pairOrder[0]),
	}
	rc <- &workflows.PairwiseTestRun{
		FirstTestRun: &workflows.TestRun{
			Status: run_benchmark.State(backends.RunBenchmarkFailure),
		},
		SecondTestRun: &workflows.TestRun{
			Status: run_benchmark.State(backends.RunBenchmarkFailure),
		},
		Permutation: workflows.PairwiseOrder(pairOrder[0]),
	}
	// The cas references used here are an example of the type of return
	// one can get. They are arbitrary and independent of the inputs.
	runs := []*workflows.TestRun{
		{
			Status: run_benchmark.State(swarming.TASK_STATE_COMPLETED),
			CAS:    &apipb.CASReference{CasInstance: "projects/chrome-swarming/instances/default_instance", Digest: &apipb.Digest{Hash: "3f2f2f849ece00d5df0d03871c8d1a14df2c1b75edd3888d7c34db12e7461c76", SizeBytes: 180}},
			Values: map[string][]float64{
				chart: chartExpectedValues,
			},
		},
		{
			Status: run_benchmark.State(swarming.TASK_STATE_COMPLETED),
			CAS:    &apipb.CASReference{CasInstance: "projects/chrome-swarming/instances/default_instance", Digest: &apipb.Digest{Hash: "6e1b133c5400c3e429e822252cb8e2cbe54c072ee75a2f732a1ec9bf0671b61a", SizeBytes: 810}},
			Values: map[string][]float64{
				chart: chartExpectedValues,
			},
		},
	}
	for i := 1; i < iterations; i++ {
		first := pairOrder[i]
		second := 1 - first // 1-0 = 1; 1-1 = 0;
		ptrs[i] = &workflows.PairwiseTestRun{
			FirstTestRun:  runs[first],
			SecondTestRun: runs[second],
			Permutation:   workflows.PairwiseOrder(first),
		}
		rc <- &workflows.PairwiseTestRun{
			FirstTestRun:  runs[first],
			SecondTestRun: runs[second],
			Permutation:   workflows.PairwiseOrder(first),
		}
	}

	return ptrs, rc
}

func TestGeneratePairIndices_GenerateRandomPair(t *testing.T) {
	generate_even := func(count int) []int {
		lt := make([]int, count)
		for i := range lt {
			lt[i] = i % 2
		}
		return lt
	}
	verify := func(name string, generated []int, even []int) {
		t.Run(name, func(t *testing.T) {
			// This can still happen because this is one of the random cases, then we should change to
			// a different seed.
			assert.NotEqualValues(t, generated, even, "shuffled pairs are still evenly distributed.")
			ct := 0
			for i := range generated {
				ct = ct + generated[i]
			}
			assert.EqualValues(t, len(generated)/2, ct, "pairs don't have equal 0's and 1's.")
		})
	}

	even10 := generate_even(10)
	verify("10 pairs with seed 0", generatePairOrderIndices(0, 10), even10)
	verify("10 pairs with seed 100", generatePairOrderIndices(100, 10), even10)
	verify("20 (even) pairs with seed 200", generatePairOrderIndices(200, 20), generate_even(20))
	verify("21 (odd) pairs with seed 210", generatePairOrderIndices(210, 21), generate_even(21))

	for i := 1; i < 10; i++ {
		pairs := i * 17 // 17 and 10169 are arbitrary prime numbers.
		verify(fmt.Sprintf("%v pairs", pairs), generatePairOrderIndices(int64(pairs*10169), pairs), generate_even(pairs))
	}
}

func TestPairwiseCommitRunner_GivenValidInput_ShouldReturnValues(t *testing.T) {
	const leftCommit = "573a50658f4301465569c3faf00a145093a1fe9b"
	const rightCommit = "a633e198b79b2e0c83c72a3006cdffe642871e22"
	const seed = int64(12312)
	p := PairwiseCommitsRunnerParams{
		SingleCommitRunnerParams: SingleCommitRunnerParams{
			PinpointJobID:     "179a34b2be0000",
			BotConfig:         "linux-perf",
			Benchmark:         "blink-perf.css",
			Story:             "gc-mini-tree.html",
			Chart:             "gc-mini-tree",
			AggregationMethod: "mean",
			Iterations:        30,
		},
		Seed:        seed,
		LeftCommit:  midpoint.NewCombinedCommit(&pb.Commit{GitHash: leftCommit}),
		RightCommit: midpoint.NewCombinedCommit(&pb.Commit{GitHash: rightCommit}),
	}
	target, err := bot_configs.GetIsolateTarget(p.BotConfig, p.Benchmark)
	require.NoError(t, err)

	freeBots := []string{
		"lin-1-h516--device1",
		"build60-h7--device2",
		"lin-2-h516--device1",
		"build65-h7--device4",
		"build59-h7--device2",
	}

	leftBuildChromeParams := workflows.BuildChromeParams{
		WorkflowID: p.PinpointJobID,
		Device:     p.BotConfig,
		Target:     target,
		Commit:     p.LeftCommit,
	}
	rightBuildChromeParams := workflows.BuildChromeParams{
		WorkflowID: p.PinpointJobID,
		Device:     p.BotConfig,
		Target:     target,
		Commit:     p.RightCommit,
	}
	leftBuild := &workflows.Build{
		BuildChromeParams: workflows.BuildChromeParams{
			Commit: midpoint.NewCombinedCommit(&pb.Commit{GitHash: leftCommit}),
		},
		Status: buildbucketpb.Status_SUCCESS,
		CAS:    &apipb.CASReference{CasInstance: "projects/chrome-swarming/instances/default_instance", Digest: &apipb.Digest{Hash: "062ccf0a30a362d8e4df3c9b82172a78e3d62c2990eb30927f5863a6b08e80bb", SizeBytes: 810}},
	}
	rightBuild := &workflows.Build{
		BuildChromeParams: workflows.BuildChromeParams{
			Commit: midpoint.NewCombinedCommit(&pb.Commit{GitHash: rightCommit}),
		},
		Status: buildbucketpb.Status_SUCCESS,
		CAS:    &apipb.CASReference{CasInstance: "projects/chrome-swarming/instances/default_instance", Digest: &apipb.Digest{Hash: "51845150f953c33ee4c0900589ba916ca28b7896806460aa8935c0de2b209db6", SizeBytes: 810}},
	}

	fakeChartValues := []float64{1, 2, 3, 4}
	goFirstIndices := generatePairOrderIndices(seed, int(p.Iterations))
	ptrs, rc := generatePairwiseTestRuns(p.SingleCommitRunnerParams.Chart, fakeChartValues, goFirstIndices)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(BuildChrome, workflow.RegisterOptions{Name: workflows.BuildChrome})
	env.RegisterWorkflowWithOptions(RunBenchmarkPairwiseWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmarkPairwise})

	env.OnWorkflow(workflows.BuildChrome, mock.Anything, leftBuildChromeParams).Return(leftBuild, nil).Once()
	env.OnWorkflow(workflows.BuildChrome, mock.Anything, rightBuildChromeParams).Return(rightBuild, nil).Once()
	env.OnActivity(FindAvailableBotsActivity, mock.Anything, p.BotConfig, p.Seed).Return(freeBots, nil).Once()

	env.OnWorkflow(workflows.RunBenchmarkPairwise, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, firstP, secondP *RunBenchmarkParams, first workflows.PairwiseOrder) (*workflows.PairwiseTestRun, error) {
		// return the channel with all of the data
		return <-rc, nil
	}).Times(int(p.Iterations))
	env.OnActivity(CollectValuesActivity, mock.Anything, mock.Anything, p.Benchmark, p.Chart, p.AggregationMethod).Return(fakeChartValues, nil).Times(2 * (int(p.Iterations) - 1))

	env.ExecuteWorkflow(PairwiseCommitsRunnerWorkflow, p)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var pr *PairwiseRun
	require.NoError(t, env.GetWorkflowResult(&pr))
	require.NotNil(t, pr)
	require.Equal(t, *leftBuild, *pr.Left.Build)
	require.Equal(t, *rightBuild, *pr.Right.Build)
	assert.Equal(t, pr.Order, goFirstIndices)
	for i, first := range pr.Order {
		if first == 0 { // left is first
			assert.EqualValues(t, ptrs[i].FirstTestRun, pr.Left.Runs[i], fmt.Sprintf("[%v], left first", i))
			assert.EqualValues(t, ptrs[i].SecondTestRun, pr.Right.Runs[i], fmt.Sprintf("[%v], left first", i))
		} else { // right is first
			assert.EqualValues(t, ptrs[i].FirstTestRun, pr.Right.Runs[i], fmt.Sprintf("[%v], right first", i))
			assert.EqualValues(t, ptrs[i].SecondTestRun, pr.Left.Runs[i], fmt.Sprintf("[%v], right first", i))
		}
	}
	env.AssertExpectations(t)
}
