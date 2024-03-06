package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

// TODO(b/327019543): More tests and test data should be added here
//
//	This is only to validate the dependent workflow signature and the workflow can connect.
func TestBisect_SimpleNoDiffCommits_ShouldReturnEmptyCommit(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(&CommitRun{}, nil).Times(2)
	env.OnActivity(GetAllValues, mock.Anything, mock.Anything, mock.Anything).Return([]float64{}, nil).Twice()
	env.OnActivity(ComparePerformanceActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&compare.CompareResults{Verdict: compare.Same}, nil).Once()

	env.ExecuteWorkflow(BisectWorkflow, &workflows.BisectParams{
		Request: &pb.ScheduleBisectRequest{
			ComparisonMagnitude: "1",
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var be *pb.BisectExecution
	require.NoError(t, env.GetWorkflowResult(&be))
	require.NotNil(t, be)
	require.NotEmpty(t, be.JobId)
	require.Empty(t, be.Commit)
	env.AssertExpectations(t)
}

func TestUpdateRuns_NewCommit_SetsNewRun(t *testing.T) {
	commit := &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "hash",
		},
	}
	cRun := &CommitRun{
		Commit: commit,
		Build: &workflows.Build{
			ID: 1234,
		},
		Runs: []*workflows.TestRun{
			{TaskID: "task1"}, {TaskID: "task2"},
		},
	}

	cm := commitMap{}
	cm.updateRuns(commit, cRun)
	actual, ok := cm.get(commit)
	require.True(t, ok)
	assert.Equal(t, cRun, actual)
}

func TestUpdateRuns_ExistingCommit_AppendsNewRuns(t *testing.T) {
	commit := &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "hash",
		},
	}
	cRuns := &CommitRun{
		Commit: commit,
		Build: &workflows.Build{
			ID: 1234,
		},
		Runs: []*workflows.TestRun{
			{TaskID: "task1"},
		},
	}
	moreRuns := &CommitRun{
		Commit: commit,
		Build: &workflows.Build{
			ID: 1234,
		},
		Runs: []*workflows.TestRun{
			{TaskID: "task2"}, {TaskID: "task3"},
		},
	}
	expected := append(cRuns.Runs, moreRuns.Runs...)

	cm := commitMap{}
	cm.set(commit, cRuns)
	cm.updateRuns(commit, moreRuns)
	actual, ok := cm.get(commit)
	require.True(t, ok)
	assert.Equal(t, expected, actual.Runs)
}

func TestCalcNewRuns_BothEqual_MoreRunsForBoth(t *testing.T) {
	test := func(name string, runs, expected int32) {
		var lCommit = &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		}
		var hCommit = &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		}
		lRuns := &CommitRun{
			Runs: make([]*workflows.TestRun, runs),
		}
		hRuns := &CommitRun{
			Runs: make([]*workflows.TestRun, runs),
		}
		cm := commitMap{}
		cm.set(lCommit, lRuns)
		cm.set(hCommit, hRuns)
		lMoreRuns, hMoreRuns := cm.calcNewRuns(lCommit, hCommit)
		assert.Equal(t, lMoreRuns, hMoreRuns)
		assert.Equal(t, expected, lMoreRuns)
	}
	// see benchmarkRunIterations for how these run iterations are calculated
	test("0 runs each should schedule 10 runs each", 0, 10)
	test("5 runs each should schedule 5 runs each", 5, 5)
	test("10 runs each should schedule 10 runs each", 10, 10)
	test("20 runs each should schedule 20 runs each", 20, 20)
}

func TestCalcNewRuns_160Runs_NoMoreNewRuns(t *testing.T) {
	const runs = 160

	var lCommit = &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "lower",
		},
	}
	var hCommit = &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "higher",
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, runs),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, runs),
	}

	cm := commitMap{}
	cm.set(lCommit, lRuns)
	cm.set(hCommit, hRuns)
	lMoreRuns, hMoreRuns := cm.calcNewRuns(lCommit, hCommit)
	assert.Zero(t, lMoreRuns)
	assert.Zero(t, hMoreRuns)
}

func TestCalcNewRuns_LowerCommitMoreRuns_OnlySchedulesMoreRunsForHigherCommit(t *testing.T) {
	lCommit := &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "lower",
		},
	}
	hCommit := &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "higher",
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 10),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 0),
	}
	cm := commitMap{}
	cm.set(lCommit, lRuns)
	cm.set(hCommit, hRuns)
	lMoreRuns, hMoreRuns := cm.calcNewRuns(lCommit, hCommit)
	assert.Zero(t, lMoreRuns)
	assert.Equal(t, int32(10), hMoreRuns)
}

func TestCalcNewRuns_HigherCommitMoreRuns_OnlySchedulesMoreRunsForLowerCommit(t *testing.T) {
	lCommit := &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "lower",
		},
	}
	hCommit := &midpoint.CombinedCommit{
		Main: &midpoint.Commit{
			GitHash: "higher",
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 5),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 25),
	}
	cm := commitMap{}
	cm.set(lCommit, lRuns)
	cm.set(hCommit, hRuns)
	lMoreRuns, hMoreRuns := cm.calcNewRuns(lCommit, hCommit)
	assert.Zero(t, hMoreRuns)
	assert.Equal(t, int32(20), lMoreRuns)
}
