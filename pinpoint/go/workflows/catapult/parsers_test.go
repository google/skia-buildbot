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

// createCombinedResults a helper function to generate combinedresults
func createCombinedResults(lower string, lowerValues []float64, higher string, higherValues []float64) *internal.CombinedResults {
	return &internal.CombinedResults{
		CommitPairValues: internal.CommitPairValues{
			Lower: internal.CommitValues{
				Commit: midpoint.NewCombinedCommit(midpoint.NewChromiumCommit(lower)),
				Values: lowerValues,
			},
			Higher: internal.CommitValues{
				Commit: midpoint.NewCombinedCommit(midpoint.NewChromiumCommit(higher)),
				Values: higherValues,
			},
		},
	}
}

func TestParseResultValuesPerCommit_ListOfCombinedResults_MapOfKeysToValues(t *testing.T) {
	// commit order from oldest to newest: 493a946, 2887740, 93dd3db, 836476df, f8e1800
	comparisons := []*internal.CombinedResults{
		// initial range
		createCombinedResults("493a946", []float64{0.0}, "f8e1800", []float64{5.0}),
		// midpoint comparisons with 93dd3db
		createCombinedResults("93dd3db", []float64{3.0}, "f8e1800", []float64{5.0}),
		createCombinedResults("493a946", []float64{0.0}, "93dd3db", []float64{3.0}),
		// lower side midpoint 2887740 comparisons
		createCombinedResults("493a946", []float64{0.0}, "2887740", []float64{1.0}),
		createCombinedResults("2887740", []float64{1.0}, "93dd3db", []float64{3.0}),
		// higher side midpoint 8d36476df comparisons
		createCombinedResults("93dd3db", []float64{3.0}, "836476df", []float64{4.0}),
		createCombinedResults("836476df", []float64{4.0}, "f8e1800", []float64{5.0}),
	}
	res := parseResultValuesPerCommit(comparisons)
	require.NotNil(t, res)
	assert.Equal(t, 5, len(res))
	// spot checking a few
	baseCommit := midpoint.NewCombinedCommit(midpoint.NewChromiumCommit("493a946"))
	assert.Equal(t, 0.0, res[baseCommit.Key()][0])
	fourthCommit := midpoint.NewCombinedCommit(midpoint.NewChromiumCommit("836476df"))
	assert.Equal(t, 4.0, res[fourthCommit.Key()][0])
}

func TestParseArguments(t *testing.T) {
	request := &pinpoint_proto.ScheduleBisectRequest{
		ComparisonMode:       "performance",
		StartGitHash:         "d9ac8dd553c566b8fe107dd8c8b2275c2c9c27f1",
		EndGitHash:           "81a6a08061d9a2da7413021bce961d125dc40ca2",
		Configuration:        "win-10_laptop_low_end-perf",
		Benchmark:            "blink_perf.owp_storage",
		Story:                "blob-perf-shm.html",
		Chart:                "blob-perf-shm",
		ComparisonMagnitude:  "21.9925",
		Project:              "chromium",
		InitialAttemptCount:  "20",
		ImprovementDirection: "DOWN",
	}

	arguments, err := parseArguments(request)
	require.NoError(t, err)
	assert.Equal(t, "performance_test_suite", arguments.Target)
	assert.Equal(t, float64(21.9925), arguments.ComparisonMagnitude)
	assert.Equal(t, int32(20), arguments.InitialAttemptCount)
	assert.Nil(t, arguments.Tags)
}
