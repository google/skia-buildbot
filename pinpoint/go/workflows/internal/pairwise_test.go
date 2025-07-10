package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	jobstore "go.skia.org/infra/pinpoint/go/sql/jobs_store"
	"go.skia.org/infra/pinpoint/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const mockChart = "chart"

var mockCommit = pinpoint_proto.CombinedCommit{
	Main: common.NewChromiumCommit("fake-commit"),
}
var mockBuild = pinpoint_proto.CASReference{
	CasInstance: "projects/chromium-swarm/instances/default_instance",
	Digest: &pinpoint_proto.CASReference_Digest{
		Hash:      "98f6b9deb73c81bd80f63149e66446547fa1fd332c382d358a6f6f1afb1b1ebb",
		SizeBytes: 732,
	},
}

func registerJobStoreActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(
		func(context.Context, *pinpoint_proto.SchedulePairwiseRequest, string) error { return nil },
		activity.RegisterOptions{Name: AddInitialJob},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, string, string, int64) error { return nil },
		activity.RegisterOptions{Name: UpdateJobStatus},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, string, string) error { return nil },
		activity.RegisterOptions{Name: SetErrors},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, string, map[string]*pinpoint_proto.PairwiseExecution_WilcoxonResult) error {
			return nil
		},
		activity.RegisterOptions{Name: AddResults},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, string, *jobstore.CommitRunData, *jobstore.CommitRunData) error { return nil },
		activity.RegisterOptions{Name: AddCommitRuns},
	)
}

func TestPairwiseWorkflow_GivenUnsuccessfulWorkflow_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerJobStoreActivities(env)
	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	env.OnWorkflow(workflows.PairwiseCommitsRunner, mock.Anything, mock.Anything).Return(nil, skerr.Fmt("some error")).Once()
	env.OnActivity(AddInitialJob, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(SetErrors, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(UpdateJobStatus, mock.Anything, mock.Anything, failed, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			StartCommit: &mockCommit,
			EndCommit:   &mockCommit,
			Chart:       mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())

	var pe *pinpoint_proto.PairwiseExecution
	require.Error(t, env.GetWorkflowResult(&pe))
	assert.Nil(t, pe)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenSuccessfulWorkflow_ReturnsCorrectPValues(t *testing.T) {
	mockedValuesA := []float64{1.2, 2.2, 1.6, 2.0}
	mockedValuesB := []float64{5.5, 6.6, 5.8, 6.3}
	mockResult := &PairwiseRun{
		Left: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesA,
					},
				},
			},
		},
		Right: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesB,
					},
				},
			},
		},
	}
	statResults, err := compare.ComparePairwise(mockedValuesA, mockedValuesB, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerJobStoreActivities(env)

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	env.OnWorkflow(workflows.PairwiseCommitsRunner, mock.Anything, mock.Anything).Return(mockResult, nil).Once()
	env.OnActivity(ComparePairwiseActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(statResults, nil).Once()
	env.OnActivity(AddInitialJob, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(AddCommitRuns, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(UpdateJobStatus, mock.Anything, mock.Anything, completed, mock.Anything).Return(nil).Once()
	env.OnActivity(AddResults, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			StartCommit: &mockCommit,
			EndCommit:   &mockCommit,
			Chart:       mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var pe *pinpoint_proto.PairwiseExecution
	require.NoError(t, env.GetWorkflowResult(&pe))
	assert.NotNil(t, pe)
	assert.NotEmpty(t, pe.JobId)
	assert.Equal(t, statResults.PValue, pe.Results[mockChart].PValue)
	assert.Nil(t, pe.CulpritCandidate)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenSuccessfulWorkflowWithCulprit_ReturnsCulprit(t *testing.T) {
	mockedValuesA := []float64{1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9}
	mockedValuesB := []float64{10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9}
	mockResult := &PairwiseRun{
		Left: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesA,
					},
				},
			},
		},
		Right: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesB,
					},
				},
			},
		},
	}
	statResults, err := compare.ComparePairwise(mockedValuesA, mockedValuesB, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerJobStoreActivities(env)

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	env.OnWorkflow(workflows.PairwiseCommitsRunner, mock.Anything, mock.Anything).Return(mockResult, nil).Once()
	env.OnActivity(ComparePairwiseActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(statResults, nil).Once()
	env.OnActivity(AddInitialJob, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(AddCommitRuns, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(UpdateJobStatus, mock.Anything, mock.Anything, completed, mock.Anything).Return(nil).Once()
	env.OnActivity(AddResults, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			Chart:      mockChart,
			StartBuild: &mockBuild,
			EndCommit:  &mockCommit,
		},
		CulpritVerify: true, // if this is true, return the culprit
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var pe *pinpoint_proto.PairwiseExecution
	require.NoError(t, env.GetWorkflowResult(&pe))
	assert.NotNil(t, pe)
	assert.NotEmpty(t, pe.JobId)
	assert.Equal(t, statResults.PValue, pe.Results[mockChart].PValue)
	assert.Equal(t, "fake-commit", pe.CulpritCandidate.Main.GitHash)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenNoStartBuild_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			Chart: mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.ErrorContains(t, env.GetWorkflowError(), "Base build and commit are empty")

	var pe *pinpoint_proto.PairwiseExecution
	require.Error(t, env.GetWorkflowResult(&pe))
	assert.Nil(t, pe)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenNoEndBuild_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			StartBuild: &mockBuild,
			Chart:      mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.ErrorContains(t, env.GetWorkflowError(), "Experiment build and commit are empty")

	var pe *pinpoint_proto.PairwiseExecution
	require.Error(t, env.GetWorkflowResult(&pe))
	assert.Nil(t, pe)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenBadCas_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			StartBuild: &mockBuild,
			EndBuild:   &pinpoint_proto.CASReference{},
			Chart:      mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.ErrorContains(t, env.GetWorkflowError(), "end build is invalid")

	var pe *pinpoint_proto.PairwiseExecution
	require.Error(t, env.GetWorkflowResult(&pe))
	assert.Nil(t, pe)
	env.AssertExpectations(t)
}
