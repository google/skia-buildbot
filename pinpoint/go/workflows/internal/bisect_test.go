package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func mockedSingleCommitRun(ctx workflow.Context, p *SingleCommitRunnerParams) (*CommitRun, error) {
	return &CommitRun{
		Build: &workflows.Build{
			BuildParams: workflows.BuildParams{
				Commit: p.CombinedCommit,
			},
		},
		Runs: make([]*workflows.TestRun, p.Iterations),
	}, nil
}

func mockedGetAllDataForCompareLocalActivity(ctx context.Context, lbr *BisectRun, hbr *BisectRun, chart string) (*CommitPairValues, error) {
	return &CommitPairValues{
		Lower:  CommitValues{lbr.Build.Commit, make([]float64, 0), make([]float64, 0)},
		Higher: CommitValues{hbr.Build.Commit, make([]float64, 0), make([]float64, 0)},
	}, nil
}

// TODO(b/327019543): More tests and test data should be added here
//
//	This is only to validate the dependent workflow signature and the workflow can connect.
func TestBisectWorkflow_SimpleNoDiffCommits_ShouldReturnEmptyCommit(t *testing.T) {
	mockResult := &CombinedResults{
		Result: &compare.CompareResults{
			Verdict: compare.Different,
		},
	}
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	freeBots := []string{
		"lin-1-h516--device1",
		"build60-h7--device2",
		"lin-2-h516--device1",
		"build65-h7--device4",
		"build59-h7--device2",
	}

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun).Times(2)
	env.OnActivity(GetAllDataForCompareLocalActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockedGetAllDataForCompareLocalActivity).Once()
	env.OnActivity(CompareActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockResult, nil).Once()
	env.OnActivity(FindAvailableBotsActivity, mock.Anything, mock.Anything, mock.Anything).Return(freeBots, nil).Once()

	env.ExecuteWorkflow(BisectWorkflow, &workflows.BisectParams{
		Request: &pb.ScheduleBisectRequest{
			ComparisonMagnitude: "1",
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var be *BisectExecution
	require.NoError(t, env.GetWorkflowResult(&be))
	require.NotNil(t, be)
	require.NotEmpty(t, be.JobId)
	require.Empty(t, be.Culprits)
	env.AssertExpectations(t)
}

func TestBisectRunTracker_NewIdx_ReturnSameRun(t *testing.T) {
	tracker := bisectRunTracker{}
	idx, run := tracker.newRun(&common.CombinedCommit{})
	require.Same(t, run, tracker.get(idx), "should be exact same addresses")
}

func TestBisectRunTracker_TwoRuns_ReturnDiffIndex(t *testing.T) {
	tracker := bisectRunTracker{}
	idx1, run1 := tracker.newRun(&common.CombinedCommit{})
	idx2, run2 := tracker.newRun(&common.CombinedCommit{})
	require.NotEqualValues(t, idx1, idx2)
	require.NotSame(t, run1, run2, "pointers should be different")
	require.NotSame(t, tracker.get(idx1), tracker.get(idx2), "pointers should be different")
}

func TestBisectRunTracker_NonExistIndex_ReturnNil(t *testing.T) {
	nonExist := BisectRunIndex(1000)
	tracker := bisectRunTracker{}
	require.Nil(t, tracker.get(nonExist))
	_, _ = tracker.newRun(&common.CombinedCommit{})
	require.Nil(t, tracker.get(nonExist))
}

func TestBisectRunTracker_ManyRuns_ReturnIndex(t *testing.T) {
	tracker := bisectRunTracker{}
	for i := 0; i < 100; i++ {
		idx, run := tracker.newRun(&common.CombinedCommit{})
		require.Same(t, run, tracker.get(idx), "should be exact same addresses")
	}
}

func TestBisectWorkflow_ReplayEvents_ShouldAlwaysPass(t *testing.T) {
	replayer := worker.NewWorkflowReplayer()

	replayer.RegisterWorkflowWithOptions(BisectWorkflow, workflow.RegisterOptions{Name: workflows.Bisect})

	err := replayer.ReplayWorkflowHistoryFromJSONFile(nil, "testdata/bisect_event_history_20240627.json")
	assert.NoError(t, err)
}
