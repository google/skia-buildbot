package task_details

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/logdog/api/logpb"
	"go.skia.org/infra/mcp/services/skia/task_details/mocks"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/td"
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

	expected := `# Task Driver

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
		SwarmingTaskState: "FAILURE",
	}

	expected := `# Recipe

**Swarming Task ID:** abc123
**Swarming Task State:** FAILURE
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
		SwarmingTaskState: "SUCCESS",
		SwarmingTaskLogs:  "Log line 1\nLog line 2",
	}

	expected := `# Swarming Task

**ID:**    abc123
**State:** SUCCESS
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
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 0, 15).Return(entries[0:15], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 15, 15).Return(entries[15:30], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 30, 15).Return(entries[30:45], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 45, 15).Return(entries[45:50], nil)

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
	mockLogDog.On("GetLastEntry", ctx, logdogProject, logPath).Return(entries[len(entries)-1], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 35, 15).Return(entries[35:50], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 20, 15).Return(entries[20:35], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 5, 15).Return(entries[5:20], nil)
	mockLogDog.On("FetchLogEntries", ctx, logdogProject, logPath, 0, 5).Return(entries[0:5], nil) // Note the reduced limit.

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
