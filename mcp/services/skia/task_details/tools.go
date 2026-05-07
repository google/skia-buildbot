package task_details

import (
	"fmt"

	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/skia/format"
)

const (
	argTaskID         = "task_id"
	argStepID         = "step_id"
	argLogID          = "log_id"
	argSwarmingTaskID = "swarming_task_id"
	argLogPath        = "log_path"
	argStartIndex     = "start_index"
	argLimit          = "limit"
	argCursor         = "cursor"
	argReverse        = "reverse"

	defaultLogLimit = 500
	maxLogLimit     = 500
)

func GetTools(c *TaskDetailsClient) []common.Tool {
	return []common.Tool{
		{
			Name:        "get_task_steps",
			Description: "Retrieve the full step listing for a task. Depending on the task, this may return a Task Driver, a Recipe, or a raw Swarming task log.",
			Arguments: []common.ToolArgument{
				{
					Name:        argTaskID,
					Description: "ID of the task.",
					Required:    true,
				},
				format.FormatToolArgument(),
			},
			Handler: format.FormatResponseWrapper(c.GetTaskStepsHandler),
		},
		{
			Name:        "get_task_driver_step_logs",
			Description: "Retrieve log entries for a task driver step.",
			Arguments: []common.ToolArgument{
				{
					Name:        argTaskID,
					Description: "ID of the task.",
					Required:    true,
				},
				{
					Name:        argStepID,
					Description: "ID of the step.",
					Required:    true,
				},
				{
					Name:        argLogID,
					Description: "ID of the log.",
					Required:    true,
				},
				{
					Name:        argLimit,
					Description: fmt.Sprintf("Maximum number of entries to load. Default %d, maximum %d.", defaultLogLimit, maxLogLimit),
				},
				{
					Name:        argCursor,
					Description: "Set this to retrieve the next page of results when paginating.",
				},
				{
					Name:        argReverse,
					Description: "If true, pages are loaded from the end of the stream but each page contains entries in chronological order.",
				},
				format.FormatToolArgument(),
			},
			Handler: format.FormatResponseWrapper(c.GetTaskDriverLogsHandler),
		},
		{
			Name:        "get_recipe_step_logs",
			Description: "Retrieve log lines for a recipe step.",
			Arguments: []common.ToolArgument{
				{
					Name:        argSwarmingTaskID,
					Description: "ID of the Swarming task.",
					Required:    true,
				},
				{
					Name:        argLogPath,
					Description: "Path of the step log as specified in Recipe step result data.",
					Required:    true,
				},
				{
					Name:        argLimit,
					Description: fmt.Sprintf("Maximum number of entries to load. Default %d, maximum %d.", defaultLogLimit, maxLogLimit),
				},
				{
					Name:        argCursor,
					Description: "Set this to retrieve the next page of results when paginating.",
				},
				{
					Name:        argReverse,
					Description: "If true, pages are loaded from the end of the stream but each page contains entries in chronological order.",
				},
				format.FormatToolArgument(),
			},
			Handler: format.FormatResponseWrapper(c.GetRecipeStepLogsHandler),
		},
	}
}
