package catapult

import (
	"fmt"
	"testing"
	"time"

	_ "embed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

//go:embed testdata/main_commit_message.txt
var mainCommitMsg string

//go:embed testdata/modified_dep_commit_message.txt
var modifiedDepCommitMsg string

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
func mockParseRunDataWorkflow(ctx workflow.Context, runData []*internal.BisectRun, chart string) (*pinpoint_proto.LegacyJobResponse, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	commitToAttempts, bots, _ := parseRunData(ctx, runData, chart)
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
	chart := "chart"
	bisectRuns := []*internal.BisectRun{
		{
			CommitRun: internal.CommitRun{
				Build: &workflows.Build{
					BuildParams: workflows.BuildParams{
						Commit: common.NewCombinedCommit(common.NewChromiumCommit("d9ac8dd553c566b8fe107dd8c8b2275c2c9c27f1")),
					},
					CAS: &apipb.CASReference{
						CasInstance: "projects/chrome-swarming/instances/default_instance",
						Digest: &apipb.Digest{
							Hash:      "25009b847133c029dc585020ed7b60b6573fe12123559319ea5c04fec3b6e06c",
							SizeBytes: int64(183),
						},
					},
				},
				Runs: []*workflows.TestRun{
					{
						TaskID: swarmingTaskID,
						Status: swarming.TASK_STATE_COMPLETED,
						CAS: &apipb.CASReference{
							CasInstance: "projects/chrome-swarming/instances/default_instance",
							Digest: &apipb.Digest{
								Hash:      "25009b847133c029dc585020ed7b60b6573fe12123559319ea5c04fec3b6e06c",
								SizeBytes: int64(183),
							},
						},
						Values: map[string][]float64{
							chart: {
								0.1, 0.2, 0.3,
							},
						},
					},
					{
						TaskID: swarmingTaskID,
						Status: backends.RunBenchmarkFailure,
					},
				},
			},
		},
	}

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(mockParseRunDataWorkflow)
	env.RegisterActivity(FetchTaskActivity)

	env.OnActivity(FetchTaskActivity, mock.Anything, swarmingTaskID).Return(&apipb.TaskResultResponse{
		BotId:  botID,
		TaskId: swarmingTaskID,
	}, nil)
	env.ExecuteWorkflow(mockParseRunDataWorkflow, bisectRuns, chart)
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
	assert.Equal(t, fmt.Sprintf(casIsolateHashTemplate, "25009b847133c029dc585020ed7b60b6573fe12123559319ea5c04fec3b6e06c", int64(183)), testQuestDetail[2].Value)

	questDetail2 := actual.State[0].Attempts[1].Executions[1].Details
	assert.Empty(t, questDetail2[2].Value)
}

// createCombinedResults a helper function to generate combinedresults
func createCombinedResults(lower string, lowerValues []float64, higher string, higherValues []float64) *internal.CombinedResults {
	return &internal.CombinedResults{
		ResultType: "Performance",
		CommitPairValues: internal.CommitPairValues{
			Lower: internal.CommitValues{
				Commit: common.NewCombinedCommit(common.NewChromiumCommit(lower)),
				Values: lowerValues,
			},
			Higher: internal.CommitValues{
				Commit: common.NewCombinedCommit(common.NewChromiumCommit(higher)),
				Values: higherValues,
			},
		},
	}
}

func TestParseResultValuesPerCommit_FunctionalResult_Nothing(t *testing.T) {
	// commit order from oldest to newest: 493a946, 2887740, 93dd3db, 836476df, f8e1800
	comparisons := []*internal.CombinedResults{
		{
			ResultType: internal.Functional,
		},
	}
	res := parseResultValuesPerCommit(comparisons)
	require.NotNil(t, res)
	assert.Empty(t, res)
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
	baseCommit := common.NewCombinedCommit(common.NewChromiumCommit("493a946"))
	assert.Equal(t, 0.0, res[baseCommit.Key()][0])
	fourthCommit := common.NewCombinedCommit(common.NewChromiumCommit("836476df"))
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

func TestParseToSortedCombinedCommits_LowerLower_SortedOrder(t *testing.T) {
	// order from oldest to newest:
	// 493a946, 2887740, 93dd3db, 836476df, f8e1800, 6649fc3, 58079459, 2604df7, 4b233ef7
	// lower lower meaning:
	// midpoint f8e1800, then 93dd3db, then 288740
	comparisons := []*internal.CombinedResults{
		// initial range
		createCombinedResults("493a946", nil, "4b233ef7", nil),
		// midpoint comparisons with 93dd3db
		createCombinedResults("493a946", nil, "f8e1800", nil),
		createCombinedResults("f8e1800", nil, "4b233ef7", nil),
		// lower side midpoint 93dd3db comparisons
		createCombinedResults("93dd3db", nil, "f8e1800", nil),
		createCombinedResults("493a946", nil, "93dd3db", nil),
		// lower side midpoint 288740 comparison
		createCombinedResults("493a946", nil, "2887740", nil),
		createCombinedResults("2887740", nil, "93dd3db", nil),
	}
	expected := []*common.CombinedCommit{
		common.NewCombinedCommit(common.NewChromiumCommit("493a946")),
		common.NewCombinedCommit(common.NewChromiumCommit("2887740")),
		common.NewCombinedCommit(common.NewChromiumCommit("93dd3db")),
		common.NewCombinedCommit(common.NewChromiumCommit("f8e1800")),
		common.NewCombinedCommit(common.NewChromiumCommit("4b233ef7")),
	}
	result := parseToSortedCombinedCommits(comparisons)
	for i, combinedCommit := range result {
		assert.Equal(t, expected[i].Key(), combinedCommit.Key())
	}
}

func TestParseToSortedCombinedCommits_LowerUpper_SortedOrder(t *testing.T) {
	// order from oldest to newest:
	// [493a946, 2887740, 93dd3db, 836476df, f8e1800, 6649fc3, 58079459, 2604df7, 4b233ef7]
	// lower upper meaning:
	// midpoint f8e1800, then 93dd3db, then 836476df
	comparisons := []*internal.CombinedResults{
		// initial range
		createCombinedResults("493a946", nil, "4b233ef7", nil),
		// midpoint comparisons with f8e1800
		createCombinedResults("493a946", nil, "f8e1800", nil),
		createCombinedResults("f8e1800", nil, "4b233ef7", nil),
		// lower side midpoint 93dd3db comparisons
		createCombinedResults("93dd3db", nil, "f8e1800", nil),
		createCombinedResults("493a946", nil, "93dd3db", nil),
		// lower side midpoint 836476df comparison
		createCombinedResults("93dd3db", nil, "836476df", nil),
		createCombinedResults("836476df", nil, "f8e1800", nil),
	}
	expected := []*common.CombinedCommit{
		common.NewCombinedCommit(common.NewChromiumCommit("493a946")),
		common.NewCombinedCommit(common.NewChromiumCommit("93dd3db")),
		common.NewCombinedCommit(common.NewChromiumCommit("836476df")),
		common.NewCombinedCommit(common.NewChromiumCommit("f8e1800")),
		common.NewCombinedCommit(common.NewChromiumCommit("4b233ef7")),
	}
	result := parseToSortedCombinedCommits(comparisons)
	for i, combinedCommit := range result {
		assert.Equal(t, expected[i].Key(), combinedCommit.Key())
	}
}

func TestParseToSortedCombinedCommits_UpperLower_SortedOrder(t *testing.T) {
	// order from oldest to newest:
	// [493a946, 2887740, 93dd3db, 836476df, f8e1800, 6649fc3, 58079459, 2604df7, 4b233ef7]
	// upper lower meaning:
	// midpoint f8e1800, then 58079459, then 6649fc3
	comparisons := []*internal.CombinedResults{
		// initial range
		createCombinedResults("493a946", nil, "4b233ef7", nil),
		// midpoint comparisons with f8e1800
		createCombinedResults("493a946", nil, "f8e1800", nil),
		createCombinedResults("f8e1800", nil, "4b233ef7", nil),
		// lower side midpoint 58079459 comparisons
		createCombinedResults("58079459", nil, "4b233ef7", nil),
		createCombinedResults("f8e1800", nil, "58079459", nil),
		// lower side midpoint 6649fc3 comparison
		createCombinedResults("6649fc3", nil, "58079459", nil),
		createCombinedResults("f8e1800", nil, "6649fc3", nil),
	}
	expected := []*common.CombinedCommit{
		common.NewCombinedCommit(common.NewChromiumCommit("493a946")),
		common.NewCombinedCommit(common.NewChromiumCommit("f8e1800")),
		common.NewCombinedCommit(common.NewChromiumCommit("6649fc3")),
		common.NewCombinedCommit(common.NewChromiumCommit("58079459")),
		common.NewCombinedCommit(common.NewChromiumCommit("4b233ef7")),
	}
	result := parseToSortedCombinedCommits(comparisons)
	for i, combinedCommit := range result {
		assert.Equal(t, expected[i].Key(), combinedCommit.Key())
	}
}

func TestParseToSortedCombinedCommits_UpperUpper_SortedOrder(t *testing.T) {
	// order from oldest to newest:
	// [493a946, 2887740, 93dd3db, 836476df, f8e1800, 6649fc3, 58079459, 2604df7, 4b233ef7]
	// upper upper meaning:
	// midpoint f8e1800, then 58079459, then 2604df7
	comparisons := []*internal.CombinedResults{
		// initial range
		createCombinedResults("493a946", nil, "4b233ef7", nil),
		// midpoint comparisons with f8e1800
		createCombinedResults("493a946", nil, "f8e1800", nil),
		createCombinedResults("f8e1800", nil, "4b233ef7", nil),
		// lower side midpoint 58079459 comparisons
		createCombinedResults("f8e1800", nil, "58079459", nil),
		createCombinedResults("58079459", nil, "4b233ef7", nil),
		// lower side midpoint 2604df7 comparison
		createCombinedResults("2604df7", nil, "4b233ef7", nil),
		createCombinedResults("58079459", nil, "2604df7", nil),
	}
	expected := []*common.CombinedCommit{
		common.NewCombinedCommit(common.NewChromiumCommit("493a946")),
		common.NewCombinedCommit(common.NewChromiumCommit("f8e1800")),
		common.NewCombinedCommit(common.NewChromiumCommit("58079459")),
		common.NewCombinedCommit(common.NewChromiumCommit("2604df7")),
		common.NewCombinedCommit(common.NewChromiumCommit("4b233ef7")),
	}
	result := parseToSortedCombinedCommits(comparisons)
	for i, combinedCommit := range result {
		assert.Equal(t, expected[i].Key(), combinedCommit.Key())
	}
}

// mockParseCommitDataWorkflow is a helper function to wrap parseCommitData() under a workflow to mock the FetchCommitActivity behavior
func mockParseCommitDataWorkflow(ctx workflow.Context, combinedCommit *common.CombinedCommit) ([]*pinpoint_proto.Commit, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	return parseCommitData(ctx, combinedCommit)
}

func TestParseCommitData_CombinedCommitWithModifiedDeps_Commits(t *testing.T) {
	chromiumHash := "493a946"
	depRepo := "https://chromium-review.googlesource.com/c/v8/v8"
	depHash := "f8e1800"
	chromiumCommit := common.NewChromiumCommit(chromiumHash)
	modifiedDepCommit := &pinpoint_proto.Commit{
		Repository: depRepo,
		GitHash:    depHash,
	}
	combinedCommit := common.NewCombinedCommit(chromiumCommit, modifiedDepCommit)
	timeNow := time.Now().UTC()

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(mockParseCommitDataWorkflow)
	env.RegisterActivity(FetchCommitActivity)

	mainCommitResp := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    chromiumHash,
			Author:  "John Doe (johndoe@gmail.com)",
			Subject: "[anchor-position] Implements resolving anchor-center.",
		},
		Body:      mainCommitMsg,
		Timestamp: timeNow,
	}
	modDepCommitResp := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    depHash,
			Author:  "Jane Doe (janedoe@gmail.com)",
			Subject: "[bazel] Fix some bazel buildifier complains",
		},
		Body:      modifiedDepCommitMsg,
		Timestamp: timeNow,
	}

	// context mocked w/ mock.Anything
	env.OnActivity(FetchCommitActivity, mock.Anything, chromiumCommit).Return(mainCommitResp, nil)
	env.OnActivity(FetchCommitActivity, mock.Anything, modifiedDepCommit).Return(modDepCommitResp, nil)

	env.ExecuteWorkflow(mockParseCommitDataWorkflow, combinedCommit)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var actual []*pinpoint_proto.Commit
	require.NoError(t, env.GetWorkflowResult(&actual))
	require.NotNil(t, actual)
	require.NotEmpty(t, actual)
	assert.Equal(t, 2, len(actual))

	mainCommit := actual[0]
	assert.Equal(t, fmt.Sprintf(repositoryUrlTemplate, common.ChromiumSrcGit, chromiumHash), mainCommit.Url)
	assert.Equal(t, "johndoe@gmail.com", mainCommit.Author)
	assert.Equal(t, "I40cc1e697cd8f8f0759f18ba814e19321e19702b", mainCommit.ChangeId)
	assert.Equal(t, "refs/heads/main", mainCommit.CommitBranch)
	assert.Equal(t, int32(1252779), mainCommit.CommitPosition)
	assert.Equal(t, "https://chromium-review.googlesource.com/c/chromium/src/+/5196073", mainCommit.ReviewUrl)

	modDepCommit := actual[1]
	assert.Equal(t, int32(92657), modDepCommit.CommitPosition)
	assert.Equal(t, "https://chromium-review.googlesource.com/c/v8/v8/+/5342761", modDepCommit.ReviewUrl)
}
