package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// generateTestRuns generates a test runs data
//
// It returns the expected runs, and a channel that was buffered to send to mocked workflow.
func generateTestRuns(chart string, c int, chartExpectedValues []float64) ([]*workflows.TestRun, chan *workflows.TestRun) {
	rc := make(chan *workflows.TestRun, c)
	trs := make([]*workflows.TestRun, c)
	trs[0] = &workflows.TestRun{
		Status: run_benchmark.State(backends.RunBenchmarkFailure),
	}
	rc <- &workflows.TestRun{
		Status: run_benchmark.State(backends.RunBenchmarkFailure),
	}
	for i := 1; i < c; i++ {
		trs[i] = &workflows.TestRun{
			Status: run_benchmark.State(swarming.TASK_STATE_COMPLETED),
			Values: map[string][]float64{
				chart: chartExpectedValues,
			},
		}
		rc <- &workflows.TestRun{
			Status: run_benchmark.State(swarming.TASK_STATE_COMPLETED),
		}
	}

	return trs, rc
}

func TestSingleCommitRunner_GivenValidInput_ShouldReturnValues(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	b := &workflows.Build{
		ID:     int64(1234),
		Status: buildbucketpb.Status_SUCCESS,
	}
	const iterations, chart = 5, "fake-chart"
	fakeChartValues := []float64{1, 2, 3, 4}
	trs, rc := generateTestRuns(chart, iterations, fakeChartValues)

	env.RegisterWorkflowWithOptions(BuildWorkflow, workflow.RegisterOptions{Name: workflows.BuildChrome})
	env.RegisterWorkflowWithOptions(RunBenchmarkWorkflow, workflow.RegisterOptions{Name: workflows.RunBenchmark})

	env.OnWorkflow(workflows.BuildChrome, mock.Anything, mock.Anything).Return(b, nil).Once()
	env.OnWorkflow(workflows.RunBenchmark, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, b *RunBenchmarkParams) (*workflows.TestRun, error) {
		return <-rc, nil
	}).Times(iterations)
	// TestRun with RunBenchmarkFailure status will not collect data
	env.OnActivity(CollectValuesActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeChartValues, nil).Times(iterations - 1)

	env.ExecuteWorkflow(SingleCommitRunner, &SingleCommitRunnerParams{
		BotConfig:      "linux-perf",
		Iterations:     int32(iterations),
		Chart:          chart,
		CombinedCommit: &common.CombinedCommit{},
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

func TestAllValues_GivenNilValues_ReturnsNonNilValues(t *testing.T) {
	const chart = "chart"
	cr := &CommitRun{
		Runs: []*workflows.TestRun{
			{
				Values: map[string][]float64{
					chart: {1.0, 2.0, 3.0},
				},
			},
			{},
			{
				Values: map[string][]float64{
					chart: {4.0, 5.0, 6.0},
				},
			},
		},
	}
	actual := cr.AllValues(chart)
	assert.Equal(t, []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}, actual)
}

func TestGetBotDimensions_GivenValidInput_ShouldReturnBotDimensions(t *testing.T) {
	freeBots := []string{
		"build-h1-device0",
		"build-h1-device1",
		"build-h1-device2",
		"build-h1-device3",
		"build-h1-device4",
	}

	type args struct {
		finishedIteration int32
		iteration         int32
	}

	wantDevice1 := map[string]string{
		"key":   "id",
		"value": "build-h1-device1",
	}

	wantDevice2 := map[string]string{
		"key":   "id",
		"value": "build-h1-device2",
	}

	wantDevice4 := map[string]string{
		"key":   "id",
		"value": "build-h1-device4",
	}

	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "The first round of commit run with no finished iteration",
			args: args{0, 1},
			want: wantDevice1,
		},
		{
			name: "Commit run total iteration number smaller than bot number",
			args: args{1, 3},
			want: wantDevice4,
		},
		{
			name: "Commit run total iteration number larger than bot number",
			args: args{4, 8},
			want: wantDevice2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBotDimension(tt.args.finishedIteration, tt.args.iteration, freeBots)
			assert.Equal(t, tt.want, got, fmt.Sprintf("Test case %s failed!", tt.name))
		})
	}
}

func TestGetBotDimensions_GivenInValidInput_ShouldNotReturnBotDimensions(t *testing.T) {
	gotBotDimensionsWithNilBotList := getBotDimension(1, 2, nil)
	assert.Equal(t, (map[string]string)(nil), gotBotDimensionsWithNilBotList)

	gotBotDimensionsWithEmptyBotList := getBotDimension(1, 2, make([]string, 0))
	assert.Equal(t, (map[string]string)(nil), gotBotDimensionsWithEmptyBotList)
}
