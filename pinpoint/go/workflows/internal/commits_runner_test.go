package internal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/pinpoint/go/workflows"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// generateTestRuns generates a test runs data
//
// It returns the expected runs, and a channel that was buffered to send to mocked workflow.
func generateTestRuns(chart string, c int) ([]*workflows.TestRun, chan *workflows.TestRun) {
	rc := make(chan *workflows.TestRun, c)
	trs := make([]*workflows.TestRun, c)
	for i := 0; i < c; i++ {
		trs[i] = &workflows.TestRun{
			Status: "COMPLETED",
			Values: map[string][]float64{
				chart: {},
			},
		}
		rc <- &workflows.TestRun{
			Status: "COMPLETED",
		}
	}
	return trs, rc
}

func Test_SingleCommitRunner_ShouldReturnValues(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	b := &workflows.Build{
		ID:     int64(1234),
		Status: buildbucketpb.Status_SUCCESS,
	}
	const iterations, chart = 5, "fake-chart"
	trs, rc := generateTestRuns(chart, iterations)

	env.RegisterWorkflowWithOptions(BuildChrome, workflow.RegisterOptions{Name: workflows.BuildChrome})
	env.RegisterWorkflowWithOptions(RunBenchmarkWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmark})

	env.OnWorkflow(workflows.BuildChrome, mock.Anything, mock.Anything).Return(b, nil).Once()
	env.OnWorkflow(workflows.RunBenchmark, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, b *RunBenchmarkParams) (*workflows.TestRun, error) {
		return <-rc, nil
	}).Times(iterations)
	env.OnActivity(CollectValuesActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]float64{}, nil).Times(iterations)

	env.ExecuteWorkflow(SingleCommitRunner, &SingleCommitRunnerParams{
		BotConfig:  "linux-perf",
		Iterations: int32(iterations),
		Chart:      chart,
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cr *CommitRun
	require.NoError(t, env.GetWorkflowResult(&cr))
	require.NotNil(t, cr)
	require.Equal(t, *b, *cr.Build)
	require.EqualValues(t, trs, cr.Runs)
	env.AssertExpectations(t)
}
