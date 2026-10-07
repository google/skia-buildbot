package task_details

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/logdog/api/logpb"
	"go.chromium.org/luci/logdog/common/types"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
	"go.skia.org/infra/mcp/services/skia/task_details/mocks"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/td"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetTaskStepsResult_String_TaskDriver(t *testing.T) {
	res := GetTaskStepsResult{
		TaskDriver: &display.TaskDriverRunDisplay{
			StepDisplay: &display.StepDisplay{
				StepProperties: &td.StepProperties{
					Id:   "root",
					Name: "Root Step",
				},
				Result: td.StepResultSuccess,
				Steps: []*display.StepDisplay{
					{
						StepProperties: &td.StepProperties{
							Id:   "sub-1",
							Name: "Sub Step 1",
						},
						Result: td.StepResultSuccess,
						Steps: []*display.StepDisplay{
							{
								StepProperties: &td.StepProperties{
									Id:   "sub-sub-1",
									Name: "Sub-sub Step 1",
								},
								Result: td.StepResultSuccess,
							},
						},
					},
					{
						StepProperties: &td.StepProperties{
							Id:   "sub-2",
							Name: "Sub Step 2",
						},
						Result: td.StepResultFailure,
					},
				},
			},
		},
	}

	expected := `## Task Driver details

- id=root name="Root Step" (SUCCESS)
  - id=sub-1 name="Sub Step 1" (SUCCESS)
    - id=sub-sub-1 name="Sub-sub Step 1" (SUCCESS)
  - id=sub-2 name="Sub Step 2" (FAILURE)
`
	require.Equal(t, expected, res.String())
}

func TestGetTaskStepsResult_String_Recipe(t *testing.T) {
	res := GetTaskStepsResult{
		Recipe: &RecipeStep{
			Name:   "Root Step",
			Status: "SUCCESS",
			Substeps: []*RecipeStep{
				{
					Name:   "Sub Step 1",
					Status: "SUCCESS",
					Substeps: []*RecipeStep{
						{
							Name:         "Sub-sub Step 1",
							Status:       "SUCCESS",
							StdoutStream: "steps/sub-sub-step-1/stdout",
						},
					},
				},
				{
					Name:         "Sub Step 2",
					Status:       "FAILURE",
					StdoutStream: "steps/sub-step-2/stdout",
					StderrStream: "steps/sub-step-2/stderr",
				},
			},
		},
		SwarmingTaskID:    "abc123",
		SwarmingBotID:     "some-bot",
		SwarmingTaskState: "FAILURE",
	}

	expected := `## Recipe details

**Swarming Task ID:** abc123
**Swarming Task State:** FAILURE
**Swarming Bot ID:** some-bot
**Steps:**
- "Root Step" (SUCCESS)
  - "Sub Step 1" (SUCCESS)
    - "Sub-sub Step 1" (SUCCESS)
      stdout log path: steps/sub-sub-step-1/stdout
  - "Sub Step 2" (FAILURE)
    stdout log path: steps/sub-step-2/stdout
    stderr log path: steps/sub-step-2/stderr
`
	require.Equal(t, expected, res.String())
}

func TestGetTaskStepsResult_String_Swarming(t *testing.T) {
	res := GetTaskStepsResult{
		SwarmingTaskID:    "abc123",
		SwarmingBotID:     "some-bot",
		SwarmingTaskState: "SUCCESS",
		SwarmingTaskLogs:  "Log line 1\nLog line 2",
	}

	expected := `## Raw Swarming Task (no steps available)

**Swarming Task ID:** abc123
**Swarming Task State:** SUCCESS
**Swarming Bot ID:** some-bot
**Logs:**
` + "```" + `
Log line 1
Log line 2
` + "```\n"

	require.Equal(t, expected, res.String())
}

const logPath = "task1231/+/step/0/log"

func TestGetRecipeStepLogsHandler_Pagination(t *testing.T) {
	ctx := t.Context()
	mockLogDog := mocks.NewLogDogClient(t)

	client := &TaskDetailsClient{
		logdog: mockLogDog,
	}

	// Create 50 mock entries.
	entries := make([]*logpb.LogEntry, 50)
	for i := 0; i < 50; i++ {
		entries[i] = &logpb.LogEntry{
			StreamIndex: uint64(i),
			Content: &logpb.LogEntry_Text{
				Text: &logpb.Text{
					Lines: []*logpb.Text_Line{{Value: []byte(fmt.Sprintf("line %d", i))}},
				},
			},
		}
	}
	tidx := types.MessageIndex(len(entries) - 1)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 0, 15).Return(entries[0:15], tidx, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 15, 15).Return(entries[15:30], tidx, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 30, 15).Return(entries[30:45], tidx, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 45, 15).Return(entries[45:50], tidx, nil)

	// Collect the log pages.
	resps := [][]string{}
	cursor := ""
	for {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]interface{}{
					argSwarmingTaskID: "task1230",
					argLogPath:        "step/0/log",
					argLimit:          15,
					argCursor:         cursor,
				},
			},
		}
		res, err := client.GetRecipeStepLogsHandler(ctx, req)
		require.NoError(t, err)
		logsRes, ok := res.(*GetLogsResponse)
		require.True(t, ok)
		resps = append(resps, logsRes.Logs)
		cursor = logsRes.Cursor
		if cursor == "" {
			break
		}
	}
	require.Len(t, resps, 4)

	// Ensure that all log lines were returned, in the correct order.
	i := 0
	for respIdx, lines := range resps {
		if respIdx == len(resps)-1 {
			require.Len(t, lines, 5)
		} else {
			require.Len(t, lines, 15)
		}
		for _, line := range lines {
			require.Equal(t, fmt.Sprintf("line %d", i), line)
			i++
		}
	}
}

func TestGetRecipeStepLogsHandler_Reverse(t *testing.T) {
	ctx := t.Context()
	mockLogDog := mocks.NewLogDogClient(t)

	client := &TaskDetailsClient{
		logdog: mockLogDog,
	}

	// Create 50 mock entries.
	entries := make([]*logpb.LogEntry, 50)
	for i := 0; i < 50; i++ {
		entries[i] = &logpb.LogEntry{
			StreamIndex: uint64(i),
			Content: &logpb.LogEntry_Text{
				Text: &logpb.Text{
					Lines: []*logpb.Text_Line{{Value: []byte(fmt.Sprintf("line %d", i))}},
				},
			},
		}
	}
	tidx := types.MessageIndex(len(entries) - 1)
	mockLogDog.On("GetLastEntry", ctx, LogDogProject, logPath).Return(entries[len(entries)-1], true, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 35, 15).Return(entries[35:50], tidx, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 20, 15).Return(entries[20:35], tidx, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 5, 15).Return(entries[5:20], tidx, nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 0, 5).Return(entries[0:5], tidx, nil) // Note the reduced limit.

	// Collect the log pages.
	resps := [][]string{}
	cursor := ""
	for {
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]interface{}{
					argSwarmingTaskID: "task1230",
					argLogPath:        "step/0/log",
					argLimit:          15,
					argCursor:         cursor,
					argReverse:        true,
				},
			},
		}
		res, err := client.GetRecipeStepLogsHandler(ctx, req)
		require.NoError(t, err)
		logsRes, ok := res.(*GetLogsResponse)
		require.True(t, ok)
		resps = append(resps, logsRes.Logs)
		cursor = logsRes.Cursor
		if cursor == "" {
			break
		}
	}
	require.Len(t, resps, 4)

	// Ensure that all log lines were returned, in the correct order.
	i := 0
	// Iterate the responses in reverse, which gives us the logs in order.
	for respIdx := len(resps) - 1; respIdx >= 0; respIdx-- {
		lines := resps[respIdx]
		if respIdx == len(resps)-1 {
			require.Len(t, lines, 5)
		} else {
			require.Len(t, lines, 15)
		}
		for _, line := range lines {
			require.Equal(t, fmt.Sprintf("line %d", i), line)
			i++
		}
	}
}

func TestGetRecipeStepLogsHandler_ByteTruncatedPage(t *testing.T) {
	ctx := t.Context()

	makeEntry := func(idx int) *logpb.LogEntry {
		return &logpb.LogEntry{
			StreamIndex: uint64(idx),
			Content: &logpb.LogEntry_Text{
				Text: &logpb.Text{
					Lines: []*logpb.Text_Line{{Value: []byte(fmt.Sprintf("line %d", idx))}},
				},
			},
		}
	}

	mockLogDog := mocks.NewLogDogClient(t)
	client := &TaskDetailsClient{logdog: mockLogDog}
	page1 := []*logpb.LogEntry{makeEntry(0), makeEntry(1)}
	page2 := []*logpb.LogEntry{makeEntry(2), makeEntry(3)}
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 0, 15).Return(page1, types.MessageIndex(3), nil)
	mockLogDog.On("FetchLogEntries", ctx, LogDogProject, logPath, 2, 15).Return(page2, types.MessageIndex(3), nil)

	res1, err := client.GetRecipeStepLogsHandler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				argSwarmingTaskID: "task1230",
				argLogPath:        "step/0/log",
				argLimit:          15,
			},
		},
	})
	require.NoError(t, err)
	logsRes1 := res1.(*GetLogsResponse)
	require.Equal(t, []string{"line 0", "line 1"}, logsRes1.Logs)
	require.NotEmpty(t, logsRes1.Cursor)

	res2, err := client.GetRecipeStepLogsHandler(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				argSwarmingTaskID: "task1230",
				argLogPath:        "step/0/log",
				argLimit:          15,
				argCursor:         logsRes1.Cursor,
			},
		},
	})
	require.NoError(t, err)
	logsRes2 := res2.(*GetLogsResponse)
	require.Equal(t, []string{"line 2", "line 3"}, logsRes2.Logs)
	require.Empty(t, logsRes2.Cursor)
}

func TestToRecipeStep_Finished(t *testing.T) {
	nowTs := timestamppb.Now()

	completeStep := &annopb.Step{
		Name:   "root",
		Status: annopb.Status_FAILURE,
		Ended:  nowTs,
		Substep: []*annopb.Step_Substep{
			{
				Substep: &annopb.Step_Substep_Step{
					Step: &annopb.Step{
						Name:   "sub1",
						Status: annopb.Status_FAILURE,
						Ended:  nowTs,
					},
				},
			},
		},
	}
	res := ToRecipeStep(completeStep)
	require.True(t, res.Finished)
	require.True(t, res.Substeps[0].Finished)

	unfinishedSubstep := &annopb.Step{
		Name:   "root",
		Status: annopb.Status_FAILURE,
		Ended:  nowTs,
		Substep: []*annopb.Step_Substep{
			{
				Substep: &annopb.Step_Substep_Step{
					Step: &annopb.Step{
						Name:   "sub1",
						Status: annopb.Status_RUNNING,
					},
				},
			},
		},
	}
	resUnfinished := ToRecipeStep(unfinishedSubstep)
	require.False(t, resUnfinished.Finished)
	require.False(t, resUnfinished.Substeps[0].Finished)
}
