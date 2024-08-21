package catapult

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	perf_workflow "go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func generateCulpritFinderParams() *workflows.CulpritFinderParams {
	return &workflows.CulpritFinderParams{
		Request: &pinpoint_proto.ScheduleCulpritFinderRequest{
			StartGitHash:         "8f2037564966f83e53701d157622dd42b931a13f",
			EndGitHash:           "049ab03450dd980d3afc27f13edfef9f510ed819",
			Configuration:        "win-11-perf",
			Benchmark:            "speedometer2",
			Story:                "Speedometer2",
			Chart:                "RunsPerMinute",
			Statistic:            "",
			ComparisonMagnitude:  "1.0",
			ImprovementDirection: "Up",
		},
		CallbackParams: &pinpoint_proto.CulpritProcessingCallbackParams{
			AnomalyGroupId:        "12345",
			CulpritServiceUrl:     "http://backend/service",
			TemporalTaskQueueName: "mock-task-queue",
		},
	}
}

func mockProcessCulprit(ctx workflow.Context, input *perf_workflow.ProcessCulpritParam) (*perf_workflow.ProcessCulpritResult, error) {
	return nil, nil
}

func createPairwiseExecutionChannel(fakeCulprits []*pinpoint_proto.CombinedCommit) chan *pinpoint_proto.PairwiseExecution {
	rc := make(chan *pinpoint_proto.PairwiseExecution, len(fakeCulprits))
	pairwiseExecutions := make([]*pinpoint_proto.PairwiseExecution, len(fakeCulprits))
	pairwiseExecutions[0] = &pinpoint_proto.PairwiseExecution{}
	rc <- &pinpoint_proto.PairwiseExecution{}
	for i := 1; i < len(fakeCulprits); i++ {
		pairwiseExecutions[i] = &pinpoint_proto.PairwiseExecution{
			Culprit: fakeCulprits[i],
		}
		rc <- &pinpoint_proto.PairwiseExecution{
			Culprit: fakeCulprits[i],
		}
	}
	return rc
}

func TestCulpritFinder_NoRegression_ReturnsEarly(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})

	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(&pinpoint_proto.PairwiseExecution{
		Significant: false,
	}, nil).Once()

	env.ExecuteWorkflow(CulpritFinderWorkflow, generateCulpritFinderParams())
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.False(t, cfe.RegressionVerified)
	assert.Nil(t, cfe.Culprits)
	env.AssertExpectations(t)
}

func TestCulpritFinder_NoCulpritsAfterBisect_ReturnsRegression(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})
	env.RegisterWorkflowWithOptions(CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})
	env.RegisterWorkflowWithOptions(mockProcessCulprit, workflow.RegisterOptions{Name: perf_workflow.ProcessCulprit})

	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(
		&pinpoint_proto.PairwiseExecution{
			Significant: true,
			Statistic: &pinpoint_proto.PairwiseExecution_WilcoxonResult{
				ControlMedian:   0.1, // arbitrary values
				TreatmentMedian: 0.2,
			},
		}, nil).Once()
	env.OnWorkflow(workflows.CatapultBisect, mock.Anything, mock.Anything).Return(&pinpoint_proto.BisectExecution{}, nil).Once()
	env.OnWorkflow(perf_workflow.ProcessCulprit, mock.Anything, mock.Anything).Never()

	env.ExecuteWorkflow(CulpritFinderWorkflow, generateCulpritFinderParams())
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.True(t, cfe.RegressionVerified)
	assert.Nil(t, cfe.Culprits)
	env.AssertExpectations(t)
}

func TestCulpritFinder_CulpritsVerified_ReturnsCulprits_NoCallback(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})
	env.RegisterWorkflowWithOptions(CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})
	env.RegisterWorkflowWithOptions(mockProcessCulprit, workflow.RegisterOptions{Name: perf_workflow.ProcessCulprit})

	fakeCulprits := []*pinpoint_proto.CombinedCommit{
		{Main: common.NewChromiumCommit("commit2")},
		{Main: common.NewChromiumCommit("commit4")},
		{Main: common.NewChromiumCommit("commit6")},
	}

	fakeCulpritPairs := []*pinpoint_proto.Culprit{
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit1")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit2")},
		},
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit3")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit4")},
		},
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit5")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit6")},
		},
	}

	rc := createPairwiseExecutionChannel(fakeCulprits)

	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(
		&pinpoint_proto.PairwiseExecution{
			Significant: true,
			Statistic: &pinpoint_proto.PairwiseExecution_WilcoxonResult{
				ControlMedian:   0.1, // arbitrary values
				TreatmentMedian: 0.2,
			},
		}, nil).Once()
	env.OnWorkflow(workflows.CatapultBisect, mock.Anything, mock.Anything).Return(&pinpoint_proto.BisectExecution{
		Culprits:         fakeCulprits,
		DetailedCulprits: fakeCulpritPairs,
	}, nil).Once()
	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, pp *workflows.PairwiseParams) (*pinpoint_proto.PairwiseExecution, error) {
		return <-rc, nil
	}).Times(len(fakeCulprits))
	env.OnWorkflow(perf_workflow.ProcessCulprit, mock.Anything, mock.Anything).Never()

	params := generateCulpritFinderParams()
	params.CallbackParams.CulpritServiceUrl = ""
	env.ExecuteWorkflow(CulpritFinderWorkflow, params)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.True(t, cfe.RegressionVerified)
	assert.EqualValues(t, fakeCulprits[1:], cfe.Culprits)
	env.AssertExpectations(t)
}

func TestCulpritFinder_CulpritsVerified_ReturnsCulprits(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})
	env.RegisterWorkflowWithOptions(CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})
	env.RegisterWorkflowWithOptions(mockProcessCulprit, workflow.RegisterOptions{Name: perf_workflow.ProcessCulprit})

	fakeCulprits := []*pinpoint_proto.CombinedCommit{
		{Main: common.NewChromiumCommit("commit2")},
		{Main: common.NewChromiumCommit("commit4")},
		{Main: common.NewChromiumCommit("commit6")},
	}

	fakeCulpritPairs := []*pinpoint_proto.Culprit{
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit1")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit2")},
		},
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit3")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit4")},
		},
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit5")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit6")},
		},
	}

	rc := createPairwiseExecutionChannel(fakeCulprits)

	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(
		&pinpoint_proto.PairwiseExecution{
			Significant: true,
			Statistic: &pinpoint_proto.PairwiseExecution_WilcoxonResult{
				ControlMedian:   0.1, // arbitrary values
				TreatmentMedian: 0.2,
			},
		}, nil).Once()
	env.OnWorkflow(workflows.CatapultBisect, mock.Anything, mock.Anything).Return(&pinpoint_proto.BisectExecution{
		Culprits:         fakeCulprits,
		DetailedCulprits: fakeCulpritPairs,
	}, nil).Once()
	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, pp *workflows.PairwiseParams) (*pinpoint_proto.PairwiseExecution, error) {
		return <-rc, nil
	}).Times(len(fakeCulprits))
	env.OnWorkflow(perf_workflow.ProcessCulprit, mock.Anything, mock.Anything).Once()

	env.ExecuteWorkflow(CulpritFinderWorkflow, generateCulpritFinderParams())
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.True(t, cfe.RegressionVerified)
	assert.EqualValues(t, fakeCulprits[1:], cfe.Culprits)
	env.AssertExpectations(t)
}

func mockPinpointCommit(repo string, hash string) *pinpoint_proto.Commit {
	return &pinpoint_proto.Commit{
		Repository: repo,
		GitHash:    hash,
	}
}

func TestFindLastDepCommit_Main(t *testing.T) {
	c := mockPinpointCommit("repo", "deadbeef1234")
	combined := &pinpoint_proto.CombinedCommit{
		Main: c,
	}
	last_dep := findLastDepCommit(combined)
	assert.Equal(t, last_dep.Repository, "repo")
	assert.Equal(t, last_dep.GitHash, "deadbeef1234")
	combined_2 := &pinpoint_proto.CombinedCommit{
		Main:         c,
		ModifiedDeps: []*pinpoint_proto.Commit{},
	}
	last_dep_2 := findLastDepCommit(combined_2)
	assert.Equal(t, last_dep_2.Repository, "repo")
	assert.Equal(t, last_dep_2.GitHash, "deadbeef1234")
}

func TestFindLastDepCommit_Deps(t *testing.T) {
	c := mockPinpointCommit("repo", "deadbeef1234")
	c2 := mockPinpointCommit("repo2", "dadabeef1234")
	c3 := mockPinpointCommit("repo3", "bababeef1234")
	combined := &pinpoint_proto.CombinedCommit{
		Main:         c,
		ModifiedDeps: []*pinpoint_proto.Commit{c2, c3},
	}
	last_dep := findLastDepCommit(combined)
	assert.Equal(t, last_dep.Repository, "repo3")
	assert.Equal(t, last_dep.GitHash, "bababeef1234")
}
