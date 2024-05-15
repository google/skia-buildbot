package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

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
	verify("10 pairs with seed 0", generatePairIndices(0, 10), even10)
	verify("10 pairs with seed 100", generatePairIndices(100, 10), even10)
	verify("20 (even) pairs with seed 200", generatePairIndices(200, 20), generate_even(20))
	verify("21 (odd) pairs with seed 210", generatePairIndices(210, 21), generate_even(21))

	for i := 1; i < 10; i++ {
		pairs := i * 17 // 17 and 10169 are arbitrary prime numbers.
		verify(fmt.Sprintf("%v pairs", pairs), generatePairIndices(int64(pairs*10169), pairs), generate_even(pairs))
	}
}

func TestPairwiseCommitRunner_GivenValidInput_ShouldReturnValues(t *testing.T) {
	const leftCommit = "573a50658f4301465569c3faf00a145093a1fe9b"
	const rightCommit = "a633e198b79b2e0c83c72a3006cdffe642871e22"

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	freeBots := []string{
		"lin-1-h516--device1",
		"build60-h7--device2",
		"lin-2-h516--device1",
		"build65-h7--device4",
		"build59-h7--device2",
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

	p := PairwiseCommitsRunnerParams{
		SingleCommitRunnerParams: SingleCommitRunnerParams{
			PinpointJobID:     "179a34b2be0000",
			BotConfig:         "linux-perf",
			Benchmark:         "blink-perf.css",
			Story:             "gc-mini-tree.html",
			Chart:             "gc-mini-tree",
			AggregationMethod: "mean",
			Iterations:        4,
		},
		Seed:        12312,
		LeftCommit:  midpoint.NewCombinedCommit(&pb.Commit{GitHash: leftCommit}),
		RightCommit: midpoint.NewCombinedCommit(&pb.Commit{GitHash: rightCommit}),
	}

	fakeChartValues := []float64{1, 2, 3, 4}
	left_trs, left_rc := generateTestRuns(p.Chart, int(p.Iterations), fakeChartValues)
	right_trs, right_rc := generateTestRuns(p.Chart, int(p.Iterations), fakeChartValues)

	env.RegisterWorkflowWithOptions(BuildChrome, workflow.RegisterOptions{Name: workflows.BuildChrome})
	env.RegisterWorkflowWithOptions(RunBenchmarkWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmark})
	target, err := bot_configs.GetIsolateTarget(p.BotConfig, p.Benchmark)
	require.NoError(t, err)

	leftBuildChromeParams := workflows.BuildChromeParams{
		WorkflowID: p.PinpointJobID,
		Device:     p.BotConfig,
		Target:     target,
		Commit:     p.LeftCommit,
	}
	env.OnWorkflow(workflows.BuildChrome, mock.Anything, leftBuildChromeParams).Return(leftBuild, nil).Once()

	rightBuildChromeParams := workflows.BuildChromeParams{
		WorkflowID: p.PinpointJobID,
		Device:     p.BotConfig,
		Target:     target,
		Commit:     p.RightCommit,
	}
	env.OnWorkflow(workflows.BuildChrome, mock.Anything, rightBuildChromeParams).Return(rightBuild, nil).Once()

	env.OnActivity(FindAvailableBotsActivity, mock.Anything, p.BotConfig, p.Seed).Return(freeBots, nil).Once()

	// The following 2 mock calls have arguments that vary which each iteration. Hence to make this
	// test simpler we have accepted mock.Anything for those varying arguments.
	env.OnWorkflow(workflows.RunBenchmark, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, b *RunBenchmarkParams) (*workflows.TestRun, error) {
		if b.Commit.Main.GitHash == leftCommit {
			return <-left_rc, nil
		}

		return <-right_rc, nil
	}).Times(2 * int(p.Iterations))
	env.OnActivity(CollectValuesActivity, mock.Anything, mock.Anything, p.Benchmark, p.Chart, p.AggregationMethod).Return([]float64{1, 2, 3, 4}, nil).Times(2 * (int(p.Iterations) - 1))

	env.ExecuteWorkflow(PairwiseCommitsRunnerWorkflow, p)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var pr *PairwiseRun
	require.NoError(t, env.GetWorkflowResult(&pr))
	require.NotNil(t, pr)
	require.Equal(t, *leftBuild, *pr.Left.Build)
	require.Equal(t, *rightBuild, *pr.Right.Build)
	require.EqualValues(t, left_trs, pr.Left.Runs)
	require.EqualValues(t, right_trs, pr.Right.Runs)
	env.AssertExpectations(t)
}
