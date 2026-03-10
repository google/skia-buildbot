package task_scheduler

import (
	"fmt"

	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	argStartTime  = "start_time"
	argEndTime    = "end_time"
	argIssue      = "issue"
	argPatchset   = "patchset"
	argTaskStatus = "status"
	argRepo       = "repo"
	argRevision   = "revision"
	argTaskName   = "name"

	taskStatusPending = "PENDING"
)

func GetTools(c *TaskSchedulerClient) []common.Tool {
	return []common.Tool{
		{
			Name:        "search_tasks",
			Description: `Retrieve a list of matching tasks from the database.`,
			Arguments: []common.ToolArgument{
				{
					Name: argStartTime,
					Description: `
[Required] The start of the time range to search for tasks.
The input should be in the RFC 3339 format and GMT should be
used as the default timezone, eg. "2025-07-12T14:30:00-00:00".`,
					Required: true,
				},
				{
					Name: argEndTime,
					Description: `
[Optional] The end of the time range to search for tasks.
The input should be in the RFC 3339 format and GMT should be
used as the default timezone, eg. "2025-07-12T14:30:00-00:00".
If not provided, the current time is used.`,
					Required: false,
				},
				{
					Name:        argIssue,
					Description: `[Optional] CL issue ID. If not provided, try jobs are excluded from results.`,
				},
				{
					Name:        argPatchset,
					Description: `[Optional] CL patchset ID. If not provided, try jobs are excluded from results.`,
				},
				{
					Name: argTaskStatus,
					Description: fmt.Sprintf(`[Optional] Task status, one of %v`, []string{
						taskStatusPending,
						string(types.TASK_STATUS_RUNNING),
						string(types.TASK_STATUS_SUCCESS),
						string(types.TASK_STATUS_FAILURE),
						string(types.TASK_STATUS_MISHAP),
					}),
				},
				{
					Name:        argRepo,
					Description: `[Optional] Git repository URL of the task, eg. "https://skia.googlesource.com/skia.git"`,
				},
				{
					Name:        argRevision,
					Description: `[Optional] Full git commit hash at which the task ran.`,
				},
				{
					Name:        argTaskName,
					Description: `[Optional] Name of the task.`,
				},
			},
			Handler: c.SearchTasksHandler,
		},
	}
}
