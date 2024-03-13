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

	cm := CommitMap{}
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

	cm := CommitMap{}
	cm.set(commit, cRuns)
	cm.updateRuns(commit, moreRuns)
	actual, ok := cm.get(commit)
	require.True(t, ok)
	assert.Equal(t, expected, actual.Runs)
}

func TestCalcSampleSize_BothEqual_MoreRunsForBoth(t *testing.T) {
	test := func(name string, runs, minSampleSize, expected int32) {
		cr := &CommitRangeTracker{
			Lower: &midpoint.CombinedCommit{
				Main: &midpoint.Commit{
					GitHash: "lower",
				},
			},
			Higher: &midpoint.CombinedCommit{
				Main: &midpoint.Commit{
					GitHash: "higher",
				},
			},
		}
		lRuns := &CommitRun{
			Runs: make([]*workflows.TestRun, runs),
		}
		hRuns := &CommitRun{
			Runs: make([]*workflows.TestRun, runs),
		}
		cm := &CommitMap{}
		cm.set(cr.Lower, lRuns)
		cm.set(cr.Higher, hRuns)
		actual := cm.calcSampleSize(cr.Lower, cr.Higher, minSampleSize)
		assert.Equal(t, expected, actual)
	}
	// see benchmarkRunIterations for how these run iterations are calculated
	test("0 runs each should expect 10 runs", 0, 10, 10)
	test("0 runs each with minSampleSize 20 should expect 20 runs", 0, 20, 20)
	test("15 runs each with minSampleSize 15 should expect 20 runs", 15, 15, 20)
	test("10 runs each should expect 20 runs", 10, 10, 20)
	test("20 runs each should expect 40 runs", 20, 10, 40)
}

func TestCalcSampleSize_160Runs_NoMoreNewRuns(t *testing.T) {
	const runs = int32(160)

	cr := &CommitRangeTracker{
		Lower: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		},
		Higher: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, runs),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, runs),
	}

	cm := &CommitMap{}
	cm.set(cr.Lower, lRuns)
	cm.set(cr.Higher, hRuns)
	actual := cm.calcSampleSize(cr.Lower, cr.Higher, 10)
	assert.Equal(t, runs, actual)
}

func TestCalcSampleSize_UnevenRuns_ExpectMaxOfCurrentRuns(t *testing.T) {
	cr := &CommitRangeTracker{
		Lower: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		},
		Higher: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 10),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 0),
	}
	cm := &CommitMap{}
	cm.set(cr.Lower, lRuns)
	cm.set(cr.Higher, hRuns)
	actual := cm.calcSampleSize(cr.Lower, cr.Higher, 5)
	assert.Equal(t, int32(10), actual)
}

func TestCalcNewRuns_HigherCommitMoreRuns_OnlySchedulesMoreRunsForLowerCommit(t *testing.T) {
	cr := &CommitRangeTracker{
		Lower: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		},
		Higher: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 5),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 25),
	}
	cm := &CommitMap{}
	cm.set(cr.Lower, lRuns)
	cm.set(cr.Higher, hRuns)
	cr.ExpectedSampleSize = cm.calcSampleSize(cr.Lower, cr.Higher, 10)
	lMoreRuns, hMoreRuns, err := cr.calcNewRuns(cm)
	require.NoError(t, err)
	assert.Zero(t, hMoreRuns)
	assert.Equal(t, int32(20), lMoreRuns)
}

func TestCalcNewRuns_LowerCommitMoreRuns_OnlySchedulesMoreRunsForHigherCommit(t *testing.T) {
	cr := &CommitRangeTracker{
		Lower: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		},
		Higher: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 5),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 0),
	}
	cm := &CommitMap{}
	cm.set(cr.Lower, lRuns)
	cm.set(cr.Higher, hRuns)
	cr.ExpectedSampleSize = cm.calcSampleSize(cr.Lower, cr.Higher, 10)
	lMoreRuns, hMoreRuns, err := cr.calcNewRuns(cm)
	require.NoError(t, err)
	assert.Zero(t, lMoreRuns)
	assert.Equal(t, int32(5), hMoreRuns)
}

func TestCalcNewRuns_NoExpectedSampleSize_ReturnsError(t *testing.T) {
	cr := &CommitRangeTracker{
		Lower: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		},
		Higher: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		},
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 5),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 0),
	}
	cm := &CommitMap{}
	cm.set(cr.Lower, lRuns)
	cm.set(cr.Higher, hRuns)
	lMoreRuns, hMoreRuns, err := cr.calcNewRuns(cm)
	require.Error(t, err)
	assert.Zero(t, lMoreRuns)
	assert.Zero(t, hMoreRuns)
}

func TestCalcNewRuns_NegativeRunsToSchedule_ReturnsError(t *testing.T) {
	cr := &CommitRangeTracker{
		Lower: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "lower",
			},
		},
		Higher: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{
				GitHash: "higher",
			},
		},
		ExpectedSampleSize: 5,
	}
	lRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 25),
	}
	hRuns := &CommitRun{
		Runs: make([]*workflows.TestRun, 30),
	}
	cm := &CommitMap{}
	cm.set(cr.Lower, lRuns)
	cm.set(cr.Higher, hRuns)
	lMoreRuns, hMoreRuns, err := cr.calcNewRuns(cm)
	require.Error(t, err)
	assert.Zero(t, lMoreRuns)
	assert.Zero(t, hMoreRuns)
}
