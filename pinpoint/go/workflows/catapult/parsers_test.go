package catapult

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestParseImprovementDirection_Up_0(t *testing.T) {
	assert.Equal(t, int32(0), parseImprovementDir(compare.Up))
}

func TestParseImprovementDirection_Down_1(t *testing.T) {
	assert.Equal(t, int32(1), parseImprovementDir(compare.Down))
}
func TestParseImprovementDirection_UnknownDir_4(t *testing.T) {
	assert.Equal(t, int32(4), parseImprovementDir(compare.UnknownDir))
}

// mockParseRunDataWorkflow is a helper function to wrap parseRunData() under a workflow to mock the FetchTaskActivity calls.
func mockParseRunDataWorkflow(ctx workflow.Context, runData []*internal.BisectRun) (*pinpoint_proto.LegacyJobResponse, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	commitToAttempts, bots, _ := parseRunData(ctx, runData)
	resp := &pinpoint_proto.LegacyJobResponse{
		Bots:  bots,
		State: []*pinpoint_proto.LegacyJobResponse_State{},
	}

	for _, attempts := range commitToAttempts {
		state := &pinpoint_proto.LegacyJobResponse_State{
			Attempts: attempts,
		}
		resp.State = append(resp.State, state)
	}
	return resp, nil
}

func TestParseRunData_RunData_StatesAndAttempts(t *testing.T) {
	swarmingTaskID := "69067f22f5cc6710"
	botID := "build123-h1"
	bisectRuns := []*internal.BisectRun{
		{
			CommitRun: internal.CommitRun{
				Build: &workflows.Build{
					BuildChromeParams: workflows.BuildChromeParams{
						Commit: midpoint.NewCombinedCommit(midpoint.NewChromiumCommit("d9ac8dd553c566b8fe107dd8c8b2275c2c9c27f1")),
					},
				},
				Runs: []*workflows.TestRun{
					{
						TaskID: swarmingTaskID,
						CAS: &swarmingV1.SwarmingRpcsCASReference{
							CasInstance: "projects/chrome-swarming/instances/default_instance",
							Digest: &swarmingV1.SwarmingRpcsDigest{
								Hash:      "25009b847133c029dc585020ed7b60b6573fe12123559319ea5c04fec3b6e06c",
								SizeBytes: int64(183),
							},
						},
					},
				},
			},
		},
	}

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(mockParseRunDataWorkflow)
	env.RegisterActivity(FetchTaskActivity)

	env.OnActivity(FetchTaskActivity, mock.Anything, swarmingTaskID).Return(&swarmingV1.SwarmingRpcsTaskResult{
		BotId:  botID,
		TaskId: swarmingTaskID,
	}, nil)
	env.ExecuteWorkflow(mockParseRunDataWorkflow, bisectRuns)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var actual *pinpoint_proto.LegacyJobResponse
	require.NoError(t, env.GetWorkflowResult(&actual))
	require.NotNil(t, actual)
	assert.Equal(t, 1, len(actual.State))
	executions := actual.State[0].Attempts[0].Executions
	// TODO(jeffyoon@) - add checks for execution[0], which is the Build quest detail once Build is appended to CommitRun.
	testQuestDetail := executions[1].Details
	assert.Equal(t, botID, testQuestDetail[0].Value)
	assert.Equal(t, swarmingTaskID, testQuestDetail[1].Value)
	assert.Equal(t, 1, len(actual.Bots))
	assert.Equal(t, botID, actual.Bots[0])
}
